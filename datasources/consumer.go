package datasources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/cdc"
	token "github.com/tidepool-org/clinic-worker/restrictedtoken"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
)

const (
	dataSourcesTopic = "tidepool.data_sources"
	defaultTimeout   = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type CDCConsumer struct {
	logger *zap.SugaredLogger

	auth      clients.AuthClient
	clinics   clinics.ClientWithResponsesInterface
	data      clients.DataClient
	shoreline shoreline.Client
}

type Params struct {
	fx.In

	Logger    *zap.SugaredLogger
	Auth      clients.AuthClient
	Clinics   clinics.ClientWithResponsesInterface
	Data      clients.DataClient
	Shoreline shoreline.Client
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = dataSourcesTopic

	return events.NewFaultTolerantConsumerGroup(config, CreateConsumer(p))
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &CDCConsumer{
		logger:    p.Logger,
		auth:      p.Auth,
		clinics:   p.Clinics,
		data:      p.Data,
		shoreline: p.Shoreline,
	}, nil
}

func (p *CDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *CDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *CDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := CDCEvent{
		Offset: cm.Offset,
	}

	if err := p.unmarshalEvent(cm.Value, &event); err != nil {
		p.logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := p.handleCDCEvent(event); err != nil {
		p.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}
	return nil
}

func (p *CDCConsumer) unmarshalEvent(value []byte, event *CDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *CDCConsumer) handleCDCEvent(event CDCEvent) error {
	if event.ShouldApplyUpdates() {
		p.logger.Infow("processing data sources event for user", "userid", event.FullDocument.UserID)
		p.logger.Debugw("event being processed", "event", event)
		if err := p.applyPatientDataSourcesUpdate(event); err != nil {
			return err
		}
	} else {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	if p.containsNewDeviceIssues(event) {
		if err := p.handleDeviceIssuesAfterUpdates(event); err != nil {
			return err
		}
	}
	return nil
}

func (p *CDCConsumer) containsNewDeviceIssues(event CDCEvent) bool {
	// DataSources start in a disconnected state upon creation so only check state on updates
	return event.FullDocument.UserID != nil && event.OperationType == cdc.OperationTypeUpdate && event.UpdateDescription.UpdatedFields.State != nil && (*event.UpdateDescription.UpdatedFields.State == "disconnected" || *event.UpdateDescription.UpdatedFields.State == "error")
}

// handleDeviceIssuesAfterUpdates checks if the conditions are met
// for sending a notification email to the user to alert them about a device
// connection issue. It is ran after the patient dataSources are updated in
// order to see more up-to-date information on whether a user shared any data
// w/ any clinics or not as that determines the email template to send.
func (p *CDCConsumer) handleDeviceIssuesAfterUpdates(event CDCEvent) error {
	fmt.Printf("handling device issue %+v\n", event.UpdateDescription.UpdatedFields)
	if event.UpdateDescription.UpdatedFields.State == nil || event.FullDocument.UserID == nil || *event.FullDocument.UserID == "" || event.FullDocument.ProviderName == nil {
		return nil
	}

	eventProviderName := *event.FullDocument.ProviderName
	updatedState := *event.UpdateDescription.UpdatedFields.State
	userId := *event.FullDocument.UserID

	// handle personal user account for errors only here. The patients consumer will handle errors/disconnections for patients who have shared data.
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	clinicsResponse, err := p.clinics.ListClinicsForPatientWithResponse(ctx, *event.FullDocument.UserID, nil)
	if err != nil {
		return fmt.Errorf(`unable to retrieve clinics for patient "%v": %w`, *event.FullDocument.UserID, err)
	}
	hasSharedData := false
	// To determine which email to send, check if user has shared data w/ any clinics
	for _, relationship := range *clinicsResponse.JSON200 {
		patient := relationship.Patient
		if patient.ConnectionRequests == nil {
			continue
		}
		connectionRequests := *patient.ConnectionRequests

		providerRequests := append(append(connectionRequests.Abbott, connectionRequests.Dexcom...), connectionRequests.Twiist...)
		if slices.ContainsFunc(providerRequests, func(req clinics.ProviderConnectionRequestV1) bool {
			return string(req.ProviderName) == eventProviderName
		}) {
			// Potentially has shared data (or does this mean always if the data
			// source exists in the data_sources collection? Just to be sure, will
			// check the mirrored version in patients)
			if patient.DataSources != nil {
				for _, ds := range *patient.DataSources {
					if string(ds.ProviderName) == eventProviderName {
						hasSharedData = true
						break
					}
				}
				if hasSharedData {
					break
				}
			}
		}
	}

	// Only send emails to personal users on error. Send emails to users who have shared data on error OR disconnected.
	if updatedState == "error" || (hasSharedData && updatedState == "disconnected") {
		// user is not associated w/ any clinic, send personal email on errors only as per [BACK-3478]
		user, err := p.shoreline.GetUser(*event.FullDocument.UserID, p.shoreline.TokenProvide())
		if err != nil {
			return fmt.Errorf(`unable to get user: %w`, err)
		}
		if user.Username != "" {
			restrictedToken, err := token.CreateRestrictedTokenForProvider(p.auth, p.shoreline, userId, *event.FullDocument.ProviderName)
			if err != nil {
				return fmt.Errorf(`error creating restricted token: %w`, err)
			}
			template := "device_issue_personal"
			if hasSharedData {
				template = "device_issue_shared"
			}
			body := clients.DeviceConnectionIssuesData{
				DataSourceState:   updatedState,
				DataSourceId:      event.FullDocument.ID.Value,
				EmailTemplate:     template,
				ProviderName:      *event.FullDocument.ProviderName,
				RestrictedTokenId: restrictedToken.ID,
				UserId:            userId,
			}
			if err := p.data.SendDeviceConnectionIssuesNotification(body); err != nil {
				return fmt.Errorf(`unable to issue request : %w`, err)
			}
		}
	}

	return nil
}

func (p *CDCConsumer) applyPatientDataSourcesUpdate(event CDCEvent) error {
	p.logger.Debugw("applying patient data sources update", "offset", event.Offset)
	if event.FullDocument.UserID == nil {
		return errors.New("expected user id to be defined")
	}

	userId := clinics.UserId(*event.FullDocument.UserID)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	sources, err := p.data.ListSources(string(userId))
	if err != nil {
		return fmt.Errorf("unexpected status code when fetching user data sources %w", err)
	}

	if len(sources) > 0 {
		var updateBody clinics.UpdatePatientDataSourcesJSONRequestBody

		for _, source := range sources {
			updateBody = append(updateBody, event.CreateUpdateBody(*source))
		}

		response, err := p.clinics.UpdatePatientDataSourcesWithResponse(ctx, userId, updateBody)
		if err != nil {
			return err
		}

		if !(response.StatusCode() == http.StatusOK || response.StatusCode() == http.StatusNotFound) {
			return fmt.Errorf("unexpected status code when updating patient data sources %v", response.StatusCode())
		}
	}

	return nil
}

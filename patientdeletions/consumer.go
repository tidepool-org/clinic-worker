package patientdeletions

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	dataclient "github.com/tidepool-org/platform/data/client"
	platformlog "github.com/tidepool-org/platform/log"
	"github.com/tidepool-org/platform/log/null"
)

const (
	patientDeletionsTopic = "clinic.patient_deletions"
	defaultTimeout        = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type PatientDeletionsCDCConsumer struct {
	logger *zap.SugaredLogger

	data      dataclient.Client
	shoreline shoreline.Client
}

type Params struct {
	fx.In

	Logger    *zap.SugaredLogger
	Data      dataclient.Client
	Shoreline shoreline.Client
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = patientDeletionsTopic

	return events.NewFaultTolerantConsumerGroup(config, CreateConsumer(p))
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewPatientDeletionsCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewPatientDeletionsCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &PatientDeletionsCDCConsumer{
		logger:    p.Logger,
		data:      p.Data,
		shoreline: p.Shoreline,
	}, nil
}

func (p *PatientDeletionsCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *PatientDeletionsCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *PatientDeletionsCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := PatientDeletionsCDCEvent{
		Offset: cm.Offset,
	}
	if err := unmarshalEvent(cm.Value, &event); err != nil {
		p.logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := p.handleCDCEvent(event); err != nil {
		p.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}
	return nil
}

func (p *PatientDeletionsCDCConsumer) handleCDCEvent(event PatientDeletionsCDCEvent) error {
	// Every patient deletion is recorded as an insertion into the patient_deletions collection.
	if event.OperationType != cdc.OperationTypeInsert ||
		!event.FullDocument.IsCustodial() ||
		event.FullDocument.Patient.UserId == "" {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	ctx, cancel := context.WithTimeout(platformlog.NewContextWithLogger(context.Background(), null.NewLogger()), defaultTimeout)
	defer cancel()

	userID := event.FullDocument.Patient.UserId
	hasData, err := p.data.HasAnyData(ctx, userID)
	if err != nil {
		return fmt.Errorf(`unable to check if custodial patient has data: %w`, err)
	}
	// Only custodial users with NO data can have their keycloak user account actually deleted.
	if !hasData {
		if err := p.shoreline.DeleteUser(userID, p.shoreline.TokenProvide()); err != nil {
			return fmt.Errorf(`unable to delete custodial user without data: %w`, err)
		}
	} else {
		// Otherwise if patient has data, remove the email from the user but do not
		// delete the user. Note the API expects no `username` field and an EMPTY
		// (not null) array for the `emails` field in order to "remove" an email.
		emptyEmails := []string{}
		update := shoreline.UserUpdate{
			Emails: &emptyEmails,
		}
		if err := p.shoreline.UpdateUser(userID, update, p.shoreline.TokenProvide()); err != nil {
			return fmt.Errorf(`unable to update custodial user with data to empty email: %w`, err)
		}
	}
	return nil
}

func unmarshalEvent(value []byte, event *PatientDeletionsCDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

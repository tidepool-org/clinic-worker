package patients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"

	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	summaries "github.com/tidepool-org/go-common/clients/summary"
	"github.com/tidepool-org/go-common/events"
	confirmations "github.com/tidepool-org/hydrophone/client"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	patientsTopic                     = "clinic.patients"
	defaultClinicianName              = "Clinic administrator"
	defaultTimeout                    = 30 * time.Second
	restrictedTokenExpirationDuration = time.Hour * 24 * 30
	DexcomDataSourceProviderName      = "dexcom"
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type PatientCDCConsumer struct {
	logger *zap.SugaredLogger

	confirmations confirmations.ClientWithResponsesInterface
	mailer        clients.MailerClient
	auth          clients.AuthClient
	shoreline     shoreline.Client
	seagull       clients.Seagull
	clinics       clinics.ClientWithResponsesInterface
	summaries     summaries.ClientWithResponsesInterface
	data          clients.DataClient
}

type Params struct {
	fx.In

	Logger *zap.SugaredLogger

	Confirmations confirmations.ClientWithResponsesInterface
	Mailer        clients.MailerClient
	Auth          clients.AuthClient
	Shoreline     shoreline.Client
	Seagull       clients.Seagull
	Clinics       clinics.ClientWithResponsesInterface
	Summaries     summaries.ClientWithResponsesInterface
	Data          clients.DataClient
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = patientsTopic

	return events.NewFaultTolerantConsumerGroup(config, CreateConsumer(p))
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewPatientCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewPatientCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &PatientCDCConsumer{
		logger:        p.Logger,
		confirmations: p.Confirmations,
		mailer:        p.Mailer,
		auth:          p.Auth,
		seagull:       p.Seagull,
		shoreline:     p.Shoreline,
		clinics:       p.Clinics,
		summaries:     p.Summaries,
		data:          p.Data,
	}, nil
}

func (p *PatientCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *PatientCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *PatientCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := PatientCDCEvent{
		Offset: cm.Offset,
	}
	if err := UnmarshalEvent(cm.Value, &event); err != nil {
		p.logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := p.handleCDCEvent(event); err != nil {
		p.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}
	return nil
}

func UnmarshalEvent(value []byte, event *PatientCDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *PatientCDCConsumer) handleCDCEvent(event PatientCDCEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if event.IsProfileUpdateEvent() {
		p.logger.Infow("processing profile update", "event", event)
		if err := p.applyProfileUpdate(event); err != nil {
			return err
		}

		if err := p.applyInviteUpdate(ctx, event); err != nil {
			return err
		}
	}

	if event.IsPatientCreateFromExistingUserEvent() {
		p.logger.Infow("processing patient create from existing user", "event", event)
		// Add existing user data sources to patient
		if err := p.addPatientDataSources(event); err != nil {
			return err
		}
	}

	if event.PatientNeedsSummary() {
		p.logger.Infow("processing summary initialization", "event", event)
		err := p.populateSummary(*event.FullDocument.UserId)
		if err != nil {
			return err
		}
	}

	if event.IsUploadReminderEvent() {
		p.logger.Infow("processing upload reminder", "event", event)
		return p.sendUploadReminder(*event.FullDocument.UserId)
	}

	var connectionRequests ConnectionRequests
	emailUpdated := event.UpdateDescription.UpdatedFields.Email != nil || slices.Contains(event.UpdateDescription.RemovedFields, "email")

	if emailUpdated {
		// If the email was updated, resend the most recent connection requests for each provider
		for _, requests := range event.FullDocument.ProviderConnectionRequests {
			connectionRequests = AppendMostRecentConnectionRequest(connectionRequests, requests)
		}
	} else {
		// If the email was not updated in this event, get all updated connection requests (if any)
		connectionRequests = event.UpdateDescription.UpdatedFields.GetUpdatedConnectionRequests()
	}

	if len(connectionRequests) > 0 {
		p.logger.Infow("processing connection requests", "event", event)

		providers := map[string]struct{}{}
		for _, r := range connectionRequests {
			providers[r.ProviderName] = struct{}{}
		}

		errs := make([]error, 0, len(providers))
		for providerName := range providers {
			templatePrefix := fmt.Sprintf("request_%s_", providerName)
			action := "connect"
			if event.FullDocument.IsCustodial() {
				action = "connect_custodial"
			}
			if event.FullDocument.DataSources != nil {
				for _, source := range *event.FullDocument.DataSources {
					if *source.ProviderName == providerName && *source.State == string(clinics.PendingReconnect) {
						action = "reconnect"
						break
					}
				}
			}

			templateName := templatePrefix + action
			errs = append(errs, p.sendProviderConnectEmail(ctx, SendProviderConnectEmailParams{
				ClinicId:     event.FullDocument.ClinicId.Value,
				ProviderName: providerName,
				UserId:       *event.FullDocument.UserId,
				PatientName:  *event.FullDocument.FullName,
				TemplateName: templateName,
			}))
		}
		if err := errors.Join(errs...); err != nil {
			return err
		}
	}

	return nil
}

/*
Here we populate summaries for all supported types, this is different from the patientsummary
functions, as with new patients, we don't know which summaries a user has, and should pull all.
*/
func (p *PatientCDCConsumer) populateSummary(userId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cgmSummaryResponse, err := p.summaries.GetSummaryWithResponse(ctx, "cgm", userId)
	if err != nil {
		return err
	}

	bgmSummaryResponse, err := p.summaries.GetSummaryWithResponse(ctx, "bgm", userId)
	if err != nil {
		return err
	}

	if !(cgmSummaryResponse.StatusCode() == http.StatusOK || cgmSummaryResponse.StatusCode() == http.StatusNotFound) {
		return fmt.Errorf("unexpected status code when retrieving patient summary %v", cgmSummaryResponse.StatusCode())
	}

	if !(bgmSummaryResponse.StatusCode() == http.StatusOK || bgmSummaryResponse.StatusCode() == http.StatusNotFound) {
		return fmt.Errorf("unexpected status code when retrieving patient summary %v", bgmSummaryResponse.StatusCode())
	}

	// user has no summary, do nothing
	if cgmSummaryResponse.JSON200 == nil && bgmSummaryResponse.JSON200 == nil {
		p.logger.Warnf("No existing BGM or CGM summary to copy for userId %s", userId)
		return nil
	}

	updateBody, err := CreateSummaryUpdateBody(cgmSummaryResponse.JSON200, bgmSummaryResponse.JSON200)
	if err != nil {
		return err
	}

	response, err := p.clinics.UpdatePatientSummaryWithResponse(ctx, userId, updateBody)
	if err != nil {
		return err
	}

	if !(response.StatusCode() == http.StatusOK || response.StatusCode() == http.StatusNoContent) {
		return fmt.Errorf("unexpected status code when updating patient summary %v", response.StatusCode())
	}

	return nil
}

func (p *PatientCDCConsumer) sendUploadReminder(userId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	email, err := p.getUserEmail(userId)
	if err != nil {
		return err
	}

	p.logger.Infow("Sending upload reminder",
		"userId", userId,
		"email", email,
	)
	template := events.SendEmailTemplateEvent{
		Recipient: email,
		Template:  "patient_upload_reminder",
	}

	return p.mailer.SendEmailTemplate(ctx, template)
}

type SendProviderConnectEmailParams struct {
	ProviderName         string
	UserId               string
	ClinicId             string
	PatientName          string
	TemplateName         string
	RevokeExistingTokens bool
}

func (p *PatientCDCConsumer) sendProviderConnectEmail(ctx context.Context, params SendProviderConnectEmailParams) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	restrictedTokenPaths := []string{"/v1/oauth/" + params.ProviderName}
	restrictedTokenExpirationTime := time.Now().Add(restrictedTokenExpirationDuration)

	currentRestrictedTokens, err := p.getUserRestrictedTokens(params.UserId)
	if err != nil {
		return err
	}

	var currentRestrictedTokenId string
	for _, token := range currentRestrictedTokens {
		if reflect.DeepEqual(token.Paths, &restrictedTokenPaths) {
			currentRestrictedTokenId = token.ID
			break
		}
	}

	// Revoke all existing tokens and re-create them to make sure old ones are not valid
	// in case the email of the patient changed
	if currentRestrictedTokenId != "" {
		err := p.auth.DeleteRestrictedToken(currentRestrictedTokenId, p.shoreline.TokenProvide())
		if err != nil {
			return err
		}
	}

	email, err := p.getUserEmail(params.UserId)
	if err != nil {
		return err
	}

	// Email has been removed, no need to (re)create tokens or send an email
	if email == "" {
		p.logger.Infow("Abort sending data provider connect email - empty email",
			"userId", params.UserId,
			"clinicId", params.ClinicId,
			"providerName", params.ProviderName,
		)
		return nil
	}

	clinicName, err := p.getClinicName(ctx, params.ClinicId)
	if err != nil {
		return err
	}

	createdRestrictedToken, err := p.auth.CreateRestrictedToken(params.UserId, restrictedTokenExpirationTime, restrictedTokenPaths, p.shoreline.TokenProvide())
	if err != nil {
		return err
	}

	// Send the email with restricted token ID
	p.logger.Infow("Sending provider connect email",
		"userId", params.UserId,
		"email", email,
		"clinicId", params.ClinicId,
		"providerName", params.ProviderName,
	)

	template := events.SendEmailTemplateEvent{
		Recipient: email,
		Template:  params.TemplateName,
		Variables: map[string]string{
			"ClinicName":        clinicName,
			"RestrictedTokenId": createdRestrictedToken.ID,
			"PatientName":       params.PatientName,
			"ProviderName":      params.ProviderName,
		},
	}

	return p.mailer.SendEmailTemplate(ctx, template)
}

func (p *PatientCDCConsumer) getUserEmail(userId string) (string, error) {
	p.logger.Debugw("Fetching user by id", "userId", userId)
	user, err := p.shoreline.GetUser(userId, p.shoreline.TokenProvide())
	if err != nil {
		if e, ok := err.(*status.StatusError); ok && e.Code == http.StatusNotFound {
			// User was probably deleted, nothing we can do
			return "", nil
		}
		return "", fmt.Errorf("unexpected error when fetching user: %w", err)
	}
	return user.Username, nil
}

func (p *PatientCDCConsumer) getUserRestrictedTokens(userId string) (clients.RestrictedTokens, error) {
	p.logger.Debugw("Fetching restricted tokens by user id", "userId", userId)

	restrictedTokens, err := p.auth.ListUserRestrictedTokens(userId, p.shoreline.TokenProvide())
	if err != nil {
		return nil, fmt.Errorf("unexpected error when fetching user: %w", err)
	}
	return restrictedTokens, nil
}

func (p *PatientCDCConsumer) getClinicianName(ctx context.Context, clinicId, clinicianId string) (string, error) {
	p.logger.Debugw("Fetching clinician by id", "clinicId", clinicId, "clinicianId", clinicianId)
	response, err := p.clinics.GetClinicianWithResponse(ctx, clinics.ClinicId(clinicId), clinics.ClinicianId(clinicianId))
	if err != nil {
		return defaultClinicianName, err
	}

	if response.StatusCode() == http.StatusOK {
		if response.JSON200.Name != nil && len(*response.JSON200.Name) > 0 {
			return *response.JSON200.Name, nil
		}
	} else if response.StatusCode() != http.StatusNotFound {
		return defaultClinicianName, fmt.Errorf("unexpected status code when fetching clinician %v", response.StatusCode())
	}

	return defaultClinicianName, nil
}

func (p *PatientCDCConsumer) getClinicName(ctx context.Context, clinicId string) (string, error) {
	p.logger.Debugw("Fetching clinic by id", "clinicId", clinicId)
	response, err := p.clinics.GetClinicWithResponse(ctx, clinics.ClinicId(clinicId))
	if err != nil {
		return "", err
	}

	if response.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("unexpected status code when fetching clinic %v", response.StatusCode())
	}

	return response.JSON200.Name, nil
}

func (p *PatientCDCConsumer) applyProfileUpdate(event PatientCDCEvent) error {
	userId := *event.FullDocument.UserId
	profile := make(map[string]interface{}, 0)
	if err := p.seagull.GetCollection(userId, "profile", p.shoreline.TokenProvide(), &profile); err != nil {
		p.logger.Errorw("unable to fetch user profile", "offset", event.Offset, zap.Error(err))
		return err
	}

	event.ApplyUpdatesToExistingProfile(profile)
	p.logger.Debugw("applying profile update", "profile", profile)

	err := p.seagull.UpdateCollection(userId, "profile", p.shoreline.TokenProvide(), profile)
	if err != nil {
		p.logger.Errorw("unable to update patient profile",
			"offset", event.Offset,
			"profile", profile,
			zap.Error(err),
		)
	}

	return err
}

func (p *PatientCDCConsumer) applyInviteUpdate(ctx context.Context, event PatientCDCEvent) error {
	p.logger.Debugw("applying invite update", "offset", event.Offset)
	if event.FullDocument.UserId == nil {
		return errors.New("expected patient id to be defined")
	}

	invite := confirmations.SendAccountSignupConfirmationJSONRequestBody{
		ClinicId:  &event.FullDocument.ClinicId.Value,
		InvitedBy: event.FullDocument.InvitedBy,
	}

	response, err := p.confirmations.SendAccountSignupConfirmationWithResponse(ctx, *event.FullDocument.UserId, invite)
	if err != nil {
		return fmt.Errorf("unable to upsert confirmation: %w", err)
	}

	// Hydrophone returns 403 when there's an existing invite, or 404 if not found, as in the case of
	// deleted users, so those are expected responses
	if response.StatusCode() != http.StatusOK && response.StatusCode() != http.StatusForbidden && response.StatusCode() != http.StatusNotFound {
		return fmt.Errorf("unexpected status code %v when upserting confirmation", response.StatusCode())
	}

	return nil
}

func (p *PatientCDCConsumer) addPatientDataSources(event PatientCDCEvent) error {
	p.logger.Debugw("adding patient data sources", "offset", event.Offset)
	if event.FullDocument.UserId == nil {
		return errors.New("expected user id to be defined")
	}

	userId := clinics.UserId(*event.FullDocument.UserId)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	sources, err := p.data.ListSources(string(userId))
	if err != nil {
		return fmt.Errorf("unexpected error when fetching user data sources %w", err)
	}

	if len(sources) > 0 {
		var updateBody clinics.UpdatePatientDataSourcesJSONRequestBody

		for _, source := range sources {
			updateBody = append(updateBody, event.CreateDataSourceBody(*source))
		}

		response, err := p.clinics.UpdatePatientDataSourcesWithResponse(ctx, userId, updateBody)
		if err != nil {
			return err
		}

		if !(response.StatusCode() == http.StatusOK || response.StatusCode() == http.StatusNotFound) {
			return fmt.Errorf("unexpected status code when adding patient data sources %v", response.StatusCode())
		}
	}

	return nil
}

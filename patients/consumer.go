package patients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/confirmation"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	summaries "github.com/tidepool-org/go-common/clients/summary"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	patientsTopic        = "clinic.patients"
	defaultClinicianName = "Clinic administrator"
	defaultTimeout       = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type PatientCDCConsumer struct {
	logger *zap.SugaredLogger

	hydrophone confirmation.Service
	mailer     clients.MailerClient
	auth       clients.AuthClient
	shoreline  shoreline.Client
	seagull    clients.Seagull
	clinics    clinics.ClientWithResponsesInterface
	summaries  summaries.ClientWithResponsesInterface
}

type Params struct {
	fx.In

	Logger *zap.SugaredLogger

	Hydrophone confirmation.Service
	Mailer     clients.MailerClient
	Auth       clients.AuthClient
	Shoreline  shoreline.Client
	Seagull    clients.Seagull
	Clinics    clinics.ClientWithResponsesInterface
	Summaries  summaries.ClientWithResponsesInterface
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
		logger:     p.Logger,
		hydrophone: p.Hydrophone,
		mailer:     p.Mailer,
		auth:       p.Auth,
		seagull:    p.Seagull,
		shoreline:  p.Shoreline,
		clinics:    p.Clinics,
		summaries:  p.Summaries,
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

func (p *PatientCDCConsumer) unmarshalEvent(value []byte, event *PatientCDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *PatientCDCConsumer) handleCDCEvent(event PatientCDCEvent) error {
	if event.IsProfileUpdateEvent() {
		p.logger.Infow("processing profile update", "event", event)
		if err := p.applyProfileUpdate(event); err != nil {
			return err
		}

		if err := p.applyInviteUpdate(event); err != nil {
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

	if event.IsRequestDexcomConnectEvent() {
		p.logger.Infow("processing upload reminder", "event", event)
		return p.sendDexcomConnectEmail(
			*event.FullDocument.UserId,
			event.FullDocument.ClinicId.Value,
			*event.FullDocument.LastRequestedDexcomConnect.ClinicianId,
			*event.FullDocument.FullName,
		)
	}

	return nil
}

func (p *PatientCDCConsumer) populateSummary(userId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	summaryResponse, err := p.summaries.GetSummaryWithResponse(ctx, summaries.UserId(userId))
	if err != nil {
		return err
	}

	if !(summaryResponse.StatusCode() == http.StatusOK || summaryResponse.StatusCode() == http.StatusNotFound) {
		return fmt.Errorf("unexpected status code when retrieving patient summary %v", summaryResponse.StatusCode())
	}

	userSummary := summaryResponse.JSON200
	// user has no summary, do nothing
	if userSummary == nil {
		return nil
	}

	updateBody := CreateSummaryUpdateBody(userSummary)

	response, err := p.clinics.UpdatePatientSummaryWithResponse(ctx, clinics.PatientId(userId), updateBody)
	if err != nil {
		return err
	}

	if !(response.StatusCode() == http.StatusOK || response.StatusCode() == http.StatusNotFound) {
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

func (p *PatientCDCConsumer) sendDexcomConnectEmail(userId, clinicId, clinicianId, patientName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	email, err := p.getUserEmail(userId)
	if err != nil {
		return err
	}

	clinicName, err := p.getClinicName(ctx, clinicId)
	if err != nil {
		return err
	}

	clinicianName, err := p.getClinicianName(ctx, clinicId, clinicianId)

	restrictedTokenPaths := []string{"/v1/oauth/dexcom/authorize"}
	restrictedTokenExpirationTime := time.Now().Add(time.Hour * 24)
	restrictedToken, err := p.auth.CreateRestrictedToken(userId, restrictedTokenExpirationTime, restrictedTokenPaths, p.shoreline.TokenProvide())
	if err != nil {
		return err
	}

	p.logger.Infow("Sending Dexcom connect email",
		"userId", userId,
		"email", email,
		"clinicId", clinicId,
		"restrictedTokenId", restrictedToken.ID,
	)

	template := events.SendEmailTemplateEvent{
		Recipient: email,
		Template:  "request_dexcom_connect",
		Variables: map[string]string{
			"ClinicName":    clinicName,
			"ClinicianName": clinicianName,
			// TODO: proper dexcomConnectURL
			"DexcomConnectURL": "https://DexcomConnectUrl" + "?restrictedToken=" + string(restrictedToken.ID),
			"PatientName":      patientName,
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

func (p *PatientCDCConsumer) applyInviteUpdate(event PatientCDCEvent) error {
	p.logger.Debugw("applying invite update", "offset", event.Offset)
	if event.FullDocument.UserId == nil {
		return errors.New("expected patient id to be defined")
	}

	invite := confirmation.SignUpInvite{
		UserId:    *event.FullDocument.UserId,
		ClinicId:  event.FullDocument.ClinicId.Value,
		InvitedBy: event.FullDocument.InvitedBy,
	}
	if err := p.hydrophone.UpsertSignUpInvite(invite); err != nil {
		p.logger.Warnw("unable to upsert sign up invite", "offset", event.Offset, zap.Error(err))
		return err
	}

	return nil
}

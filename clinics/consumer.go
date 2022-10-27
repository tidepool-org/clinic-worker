package clinics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	clinicsTopic   = "clinic.clinics"
	defaultTimeout = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type ClinicsCDCConsumer struct {
	logger *zap.SugaredLogger

	clinics   clinics.ClientWithResponsesInterface
	mailer    clients.MailerClient
	shoreline shoreline.Client
}

type Params struct {
	fx.In

	Logger    *zap.SugaredLogger
	Mailer    clients.MailerClient
	Shoreline shoreline.Client
	Clinics   clinics.ClientWithResponsesInterface
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = clinicsTopic

	return events.NewFaultTolerantConsumerGroup(config, CreateConsumer(p))
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewClinicsCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewClinicsCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &ClinicsCDCConsumer{
		logger:    p.Logger,
		mailer:    p.Mailer,
		shoreline: p.Shoreline,
		clinics:   p.Clinics,
	}, nil
}

func (p *ClinicsCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *ClinicsCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *ClinicsCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := ClinicCDCEvent{
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

func (p *ClinicsCDCConsumer) unmarshalEvent(value []byte, event *ClinicCDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *ClinicsCDCConsumer) handleCDCEvent(event ClinicCDCEvent) error {
	if !event.ShouldApplyUpdates() {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	p.logger.Infow("processing event", "event", event)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if event.isPatientTagDelete() {
		return p.DeletePatientTagFromClinicPatients(ctx, event.FullDocument.Id.Value, event.UpdateDescription.UpdatedFields.LastDeletedPatientTag.Value)
	}

	clinicName := event.FullDocument.Name
	adminUserId := event.FullDocument.Admins[0]
	recipient, err := p.getUserEmail(adminUserId)
	if err != nil {
		return err
	}

	p.logger.Infow("Sending clinic created email",
		"userId", adminUserId,
		"email", recipient,
	)
	template := events.SendEmailTemplateEvent{
		Recipient: recipient,
		Template:  "clinic_created",
		Variables: map[string]string{
			"ClinicName": clinicName,
		},
	}

	return p.mailer.SendEmailTemplate(ctx, template)
}

func (p *ClinicsCDCConsumer) getUserEmail(userId string) (string, error) {
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

func (p *ClinicsCDCConsumer) DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, patientTagId string) error {
	p.logger.Debugw("Deleteing tag from clinic patients", "clinicId", clinicId, "patientTagId", patientTagId)

	response, err := p.clinics.DeletePatientTagFromClinicPatientsWithResponse(ctx, clinics.ClinicId(clinicId), clinics.PatientTagId(patientTagId))
	if err != nil {
		return err
	} else if response.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected response when deleting tag %s from clinic %s patients", clinicId, patientTagId)
	}

	return nil
}

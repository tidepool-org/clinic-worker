package merge

import (
	"context"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/go-common/events"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
	"slices"
	"time"
)

const (
	mergePlansTopic = "clinic.merge_plans"
	defaultTimeout  = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateMergePlansConsumerGroup,
})

// MergePlansCDCConsumer is kafka consumer for executed merge plans
type MergePlansCDCConsumer struct {
	logger    *zap.SugaredLogger
	mailer    clients.MailerClient
	shoreline shoreline.Client
}

type MergePlansConsumerCDCConsumerParams struct {
	fx.In

	Logger    *zap.SugaredLogger
	Mailer    clients.MailerClient
	Shoreline shoreline.Client
}

func CreateMergePlansConsumerGroup(p MergePlansConsumerCDCConsumerParams) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = mergePlansTopic

	return events.NewFaultTolerantConsumerGroup(config, NewMergePlansConsumer(p))
}

func NewMergePlansConsumer(p MergePlansConsumerCDCConsumerParams) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewMergePlansConsumerCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewMergePlansConsumerCDCConsumer(p MergePlansConsumerCDCConsumerParams) (events.MessageConsumer, error) {
	return &MergePlansCDCConsumer{
		logger:    p.Logger,
		mailer:    p.Mailer,
		shoreline: p.Shoreline,
	}, nil
}

func (s *MergePlansCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (s *MergePlansCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return s.handleMessage(cm)
}

func (s *MergePlansCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	s.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := cdc.Event[PersistentPlan[bson.Raw]]{
		Offset: cm.Offset,
	}

	if err := UnmarshalEvent(cm.Value, &event); err != nil {
		s.logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := s.handleCDCEvent(event); err != nil {
		s.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}

	return nil
}

func (s *MergePlansCDCConsumer) handleCDCEvent(event cdc.Event[PersistentPlan[bson.Raw]]) error {
	if event.FullDocument == nil {
		s.logger.Warnw("document is empty", "offset", event.Offset)
		return nil
	}

	switch event.FullDocument.Type {
	case patientPlanType:
		return s.handlePatientPlan(event)
	case clinicianPlanType:
		return s.handleClinicianPlan(event)
	default:
		s.logger.Debugw("ignoring plan", "offset", event.Offset)
	}

	return nil
}

func (s *MergePlansCDCConsumer) handlePatientPlan(event cdc.Event[PersistentPlan[bson.Raw]]) error {
	plan := PatientPlan{}
	if err := bson.UnmarshalExtJSON(event.FullDocument.Plan, true, event); err != nil {
		return err
	}

	if plan.SourcePatient != nil && plan.SourcePatient.UserId != nil {
		recipient, err := s.getUserEmail(*plan.SourcePatient.UserId)
		if err != nil {
			return err
		}

		s.logger.Infow("Sending patient notification for clinic migration",
			"userId", *plan.SourcePatient.UserId,
			"email", recipient,
		)

		template := events.SendEmailTemplateEvent{
			Recipient: recipient,
			Template:  "patient_migrated",
			Variables: map[string]string{
				"SourceClinicName": plan.SourceClinicName,
				"TargetClinicName": plan.TargetClinicName,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		return s.mailer.SendEmailTemplate(ctx, template)
	}

	return nil
}

func (s *MergePlansCDCConsumer) handleClinicianPlan(event cdc.Event[PersistentPlan[bson.Raw]]) error {
	plan := ClinicianPlan{}
	if err := bson.UnmarshalExtJSON(event.FullDocument.Plan, true, event); err != nil {
		return err
	}

	if template := GetClinicianEmailNotificationTemplate(plan); template != "" && plan.Clinician.UserId != "" {
		recipient, err := s.getUserEmail(plan.Clinician.UserId)
		if err != nil {
			return err
		}

		s.logger.Infow("Sending clinician notification for clinic migration",
			"userId", plan.Clinician.UserId,
			"email", recipient,
		)

		template := events.SendEmailTemplateEvent{
			Recipient: recipient,
			Template:  template,
			Variables: map[string]string{
				"SourceClinicName": plan.SourceClinicName,
				"TargetClinicName": plan.TargetClinicName,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		return s.mailer.SendEmailTemplate(ctx, template)
	}

	return nil
}

func (s *MergePlansCDCConsumer) getUserEmail(userId string) (string, error) {
	s.logger.Debugw("Fetching user by id", "userId", userId)
	user, err := s.shoreline.GetUser(userId, s.shoreline.TokenProvide())
	if err != nil {
		if e, ok := err.(*status.StatusError); ok && e.Code == http.StatusNotFound {
			// User was probably deleted, nothing we can do
			return "", nil
		}
		return "", fmt.Errorf("unexpected error when fetching user: %w", err)
	}
	return user.Username, nil
}

func UnmarshalEvent(value []byte, event *cdc.Event[PersistentPlan[bson.Raw]]) error {
	return bson.UnmarshalExtJSON(value, true, event)
}

func GetClinicianEmailNotificationTemplate(plan ClinicianPlan) string {
	switch plan.ClinicianAction {
	case ClinicianActionRetain, ClinicianActionMergeInto:
		return "source_clinic_merged_notification"
	case ClinicianActionMove, ClinicianActionMerge:
		if slices.Contains(plan.ResultingRoles, "CLINIC_ADMIN") {
			return "target_clinic_admins_merger_notification"
		}
	}
	return ""
}

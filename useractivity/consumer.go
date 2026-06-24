package useractivity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/events"
)

const (
	// userActivityTopic is the suffix of the Debezium topic carrying changes from
	// the keycloak Postgres outbox table (connector topic.prefix "keycloak", schema
	// "public", table tidepool_user_activity_event). The deployment's
	// KAFKA_TOPIC_PREFIX is prepended by events.GetKafkaTopic, matching how the
	// mongo CDC consumers (e.g. datasources) resolve their topics.
	userActivityTopic = "keycloak.public.tidepool_user_activity_event"
	defaultTimeout    = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type CDCConsumer struct {
	logger  *zap.SugaredLogger
	clinics clinics.ClientWithResponsesInterface
}

type Params struct {
	fx.In

	Logger  *zap.SugaredLogger
	Clinics clinics.ClientWithResponsesInterface
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = userActivityTopic

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
		logger:  p.Logger,
		clinics: p.Clinics,
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

	// Debezium emits a tombstone (null value) alongside deletes, and the Filter SMT
	// passes tombstones through. There is nothing to project from an empty value, so
	// skip it — otherwise json.Unmarshal fails and the message is retried forever.
	if len(cm.Value) == 0 {
		p.logger.Debugw("skipping message with empty value", "offset", cm.Offset)
		return nil
	}

	event := Envelope{
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

func (p *CDCConsumer) unmarshalEvent(value []byte, event *Envelope) error {
	// Debezium emits the value as raw JSON (schemas disabled), unlike the mongo CDC
	// topics which double-encode the value as a JSON string.
	return json.Unmarshal(value, event)
}

func (p *CDCConsumer) handleCDCEvent(event Envelope) error {
	if !event.ShouldApplyUpdates() {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	row := event.After
	p.logger.Infow("processing user activity event",
		"userId", row.UserID, "eventType", row.EventType, "offset", event.Offset)

	// Updates are applied as last-writer-wins. This is safe because the Debezium
	// source connector keys the outbox by user_id, so a user's events share a
	// partition and arrive in commit (event_time) order, and runs with
	// snapshot.mode=never so history is not replayed. See the users-source
	// connector in the deployment charts.

	body, err := event.CreateUpdateBody()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	response, err := p.clinics.UpdateClinicianSecurityProfileWithResponse(ctx, clinics.UserId(row.UserID), *body)
	if err != nil {
		return err
	}

	// A 404 means the user is not a clinician (e.g. a patient); the event is still
	// a no-op success since the outbox covers every keycloak user.
	if !(response.StatusCode() == http.StatusNoContent || response.StatusCode() == http.StatusNotFound) {
		return fmt.Errorf("unexpected status code when updating clinician security profile %v", response.StatusCode())
	}

	return nil
}

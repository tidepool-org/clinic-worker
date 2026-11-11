package userprofiles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/IBM/sarama"
	"github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/events"
)

const (
	userAttributesTopic = "keycloak.public.user_attribute"
	defaultTimeout      = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type UserProfileCDCConsumer struct {
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

	config.KafkaTopic = userAttributesTopic

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
	return &UserProfileCDCConsumer{
		logger:  p.Logger,
		clinics: p.Clinics,
	}, nil
}

func (p *UserProfileCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *UserProfileCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *UserProfileCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)

	m := kafka_sarama.NewMessageFromConsumerMessage(cm)
	if m.Value == nil {
		// Ignore tombstone messages
		return nil
	}

	event := KeycloakUserAttrEvent{}
	if err := json.Unmarshal(m.Value, &event); err != nil {
		return err
	}

	if err := p.handleCDCEvent(event); err != nil {
		p.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}
	return nil
}

func (p *UserProfileCDCConsumer) handleCDCEvent(event KeycloakUserAttrEvent) error {
	if !event.ShouldApplyUpdates() {
		return nil
	}

	row := event.After
	userId := event.After.UserId

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	update := clinics.UpdateClinicUserDetailsJSONRequestBody{
		// The value of the attribute is the new clinician/patient name.
		Name: &row.Value,
	}

	resp, err := p.clinics.UpdateClinicUserDetailsWithResponse(ctx, clinics.UserId(userId), update)
	if err != nil {
		p.logger.Errorw(`could not update clinic user details "name" field`, "userId", userId, zap.Error(err))
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		err := fmt.Errorf("unexpected response code %v", resp.StatusCode())
		p.logger.Errorw(`could not update clinic user details "name" field`, "userId", userId, zap.Error(err))
		return err
	}
	return nil
}

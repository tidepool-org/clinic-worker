package redox

import (
	"encoding/json"
	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/redox/models"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"strconv"
	"time"
)

const (
	redoxTopic     = "clinic.redox"
	defaultTimeout = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type RedoxCDCConsumer struct {
	logger *zap.SugaredLogger

	clinics   clinics.ClientWithResponsesInterface
	shoreline shoreline.Client
}

type Params struct {
	fx.In

	Clinics   clinics.ClientWithResponsesInterface
	Logger    *zap.SugaredLogger
	Shoreline shoreline.Client
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = redoxTopic

	return events.NewFaultTolerantConsumerGroup(config, CreateConsumer(p))
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewRedoxCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewRedoxCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &RedoxCDCConsumer{
		clinics:   p.Clinics,
		logger:    p.Logger,
		shoreline: p.Shoreline,
	}, nil
}

func (p *RedoxCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *RedoxCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *RedoxCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := cdc.Event[models.MessageEnvelope]{
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

func (p *RedoxCDCConsumer) unmarshalEvent(value []byte, event *cdc.Event[models.MessageEnvelope]) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *RedoxCDCConsumer) handleCDCEvent(event cdc.Event[models.MessageEnvelope]) error {
	if event.FullDocument == nil {
		p.logger.Infow("skipping event with no full document", "offset", event.Offset)
	}

	if event.FullDocument.Meta.IsValid() {
		p.logger.Infow("skipping event with invalid meta", "offset", event.Offset)
	}

	switch event.FullDocument.Meta.DataModel {
	case "Order":
		if event.FullDocument.Meta.EventType == "New" {
			order := models.NewOrder{}
			if err := bson.Unmarshal(event.FullDocument.Message, &order); err != nil {
				p.logger.Errorw("unable to unmarshal order", "offset", event.Offset, zap.Error(err))
				return err
			}

			p.logger.Debugw("successfully unmarshalled new order", "offset", event.Offset, "order", order)
		}
	}

	return nil
}

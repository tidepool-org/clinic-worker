package redox

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic/redox/models"
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

var Module = fx.Provide(
	NewModuleConfig,
	NewClient,
	NewOrderProcessor,
	fx.Annotated{
		Group:  "consumers",
		Target: CreateConsumerGroup,
	},
)

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_REDOX_ENABLED" default:"false"`
}

func NewModuleConfig() (ModuleConfig, error) {
	config := ModuleConfig{}
	err := envconfig.Process("", &config)
	return config, err
}

type CDCConsumer struct {
	logger *zap.SugaredLogger

	config         ModuleConfig
	orderProcessor OrderProcessor
}

type Params struct {
	fx.In

	Logger *zap.SugaredLogger

	Config         ModuleConfig
	OrderProcessor OrderProcessor
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	if !p.Config.Enabled {
		return &cdc.DisabledEventConsumer{}, nil
	}

	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = redoxTopic

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
		logger:         p.Logger,
		config:         p.Config,
		orderProcessor: p.OrderProcessor,
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

func (p *CDCConsumer) unmarshalEvent(value []byte, event *cdc.Event[models.MessageEnvelope]) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return bson.UnmarshalExtJSON([]byte(message), true, event)
}

func (p *CDCConsumer) handleCDCEvent(event cdc.Event[models.MessageEnvelope]) error {
	if event.FullDocument == nil {
		p.logger.Infow("skipping event with no full document", "offset", event.Offset)
	}

	if event.FullDocument.Meta.IsValid() {
		p.logger.Infow("skipping event with invalid meta", "offset", event.Offset)
	}

	// We only expect orders for now
	if event.FullDocument.Meta.DataModel != DataModelOrder {
		p.logger.Infow("unexpected data model", "order", event.FullDocument.Meta, "offset", event.Offset)
		return nil
	}

	switch event.FullDocument.Meta.EventType {
	case EventTypeNewOrder:
		order := models.NewOrder{}
		if err := bson.Unmarshal(event.FullDocument.Message, &order); err != nil {
			p.logger.Errorw("unable to unmarshal order", "offset", event.Offset, zap.Error(err))
			return err
		}

		p.logger.Debugw("successfully unmarshalled new order", "offset", event.Offset, "order", order.Meta)

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		p.logger.Debugw("processing new order", "offset", event.Offset, "order", order.Meta)
		return p.orderProcessor.ProcessOrder(ctx, order)
	default:
		p.logger.Infow("unexpected order event type", "order", event.FullDocument.Meta, "offset", event.Offset)
	}

	return nil
}

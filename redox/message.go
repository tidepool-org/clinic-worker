package redox

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/go-common/events"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// MessageCDCConsumer is kafka consumer for redox message CDC events
type MessageCDCConsumer struct {
	logger *zap.SugaredLogger

	config         ModuleConfig
	orderProcessor NewOrderProcessor
}

type MessageCDCConsumerParams struct {
	fx.In

	Logger *zap.SugaredLogger

	Config         ModuleConfig
	OrderProcessor NewOrderProcessor
}

func CreateRedoxMessageConsumerGroup(p MessageCDCConsumerParams) (events.EventConsumer, error) {
	if !p.Config.Enabled {
		return &cdc.DisabledEventConsumer{}, nil
	}

	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = redoxTopic

	return events.NewFaultTolerantConsumerGroup(config, NewRedoxMessageConsumer(p))
}

func NewRedoxMessageConsumer(p MessageCDCConsumerParams) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewRedoxMessageCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewRedoxMessageCDCConsumer(p MessageCDCConsumerParams) (events.MessageConsumer, error) {
	return &MessageCDCConsumer{
		logger:         p.Logger,
		config:         p.Config,
		orderProcessor: p.OrderProcessor,
	}, nil
}

func (m *MessageCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (m *MessageCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return m.handleMessage(cm)
}

func (m *MessageCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	m.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := cdc.Event[models.MessageEnvelope]{
		Offset: cm.Offset,
	}

	if err := m.unmarshalEvent(cm.Value, &event); err != nil {
		m.logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := m.handleCDCEvent(event); err != nil {
		m.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}

	return nil
}

func (m *MessageCDCConsumer) unmarshalEvent(value []byte, event *cdc.Event[models.MessageEnvelope]) error {
	return bson.UnmarshalExtJSON(value, true, event)
}

func (m *MessageCDCConsumer) handleCDCEvent(event cdc.Event[models.MessageEnvelope]) error {
	if event.FullDocument == nil {
		m.logger.Infow("skipping event with no full document", "offset", event.Offset)
	}

	if event.FullDocument.Meta.IsValid() {
		m.logger.Infow("skipping event with invalid meta", "offset", event.Offset)
	}

	// We only expect orders for now
	if event.FullDocument.Meta.DataModel != DataModelOrder {
		m.logger.Infow("unexpected data model", "order", event.FullDocument.Meta, "offset", event.Offset)
		return nil
	}

	switch event.FullDocument.Meta.EventType {
	case EventTypeNewOrder:
		order := models.NewOrder{}
		if err := bson.Unmarshal(event.FullDocument.Message, &order); err != nil {
			m.logger.Errorw("unable to unmarshal order", "offset", event.Offset, zap.Error(err))
			return err
		}

		m.logger.Debugw("successfully unmarshalled new order", "offset", event.Offset, "order", order.Meta)

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		m.logger.Debugw("processing new order", "offset", event.Offset, "order", order.Meta)
		return m.orderProcessor.ProcessOrder(ctx, *event.FullDocument, order)
	default:
		m.logger.Infow("unexpected order event type", "order", event.FullDocument.Meta, "offset", event.Offset)
	}

	return nil
}

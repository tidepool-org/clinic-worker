package redox

import (
	"context"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/go-common/asyncevents"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"time"
)

const (
	redoxMessageTopic = "clinic.redox"
)

type MessageCDCConsumerParams struct {
	fx.In

	Logger *zap.SugaredLogger

	Config         ModuleConfig
	OrderProcessor NewOrderProcessor
}

type MessageCDCConsumer struct {
	logger *zap.SugaredLogger

	config         ModuleConfig
	orderProcessor NewOrderProcessor
}

func NewMessageCDCConsumer(p MessageCDCConsumerParams) (asyncevents.SaramaEventsRunner, error) {
	if !p.Config.Enabled {
		return &cdc.DisabledSaramaEventsRunner{}, nil
	}

	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = redoxMessageTopic
	config.SaramaConfig.ClientID = config.KafkaTopicPrefix + "clinic-worker"

	prefixedTopics := []string{config.GetPrefixedTopic()}

	runnerCfg := asyncevents.SaramaRunnerConfig{
		Brokers: config.KafkaBrokers,
		GroupID: config.KafkaConsumerGroup,
		Topics:  prefixedTopics,
		Sarama:  config.SaramaConfig,
		MessageConsumer: &MessageCDCConsumer{
			config:         p.Config,
			logger:         p.Logger,
			orderProcessor: p.OrderProcessor,
		},
	}

	delays := []time.Duration{0, time.Second * 60, time.Second * 300}
	logger := &cdc.AsynceventsLoggerAdapter{
		SugaredLogger: p.Logger,
	}

	eventsRunner := asyncevents.NewCascadingSaramaEventsRunner(runnerCfg, logger, delays, defaultTimeout)
	return eventsRunner, nil
}

func (c *MessageCDCConsumer) Consume(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error {
	if msg == nil {
		return nil
	}

	err := c.HandleMessage(ctx, msg)
	if err != nil {
		session.MarkMessage(msg, fmt.Sprintf("I have given up after error: %s", err))
		c.logger.Warnw("Unable to consume redox message", "error", err)
		return err
	}
	return nil
}

func (c *MessageCDCConsumer) HandleMessage(ctx context.Context, cm *sarama.ConsumerMessage) error {
	c.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := cdc.Event[models.MessageEnvelope]{
		Offset: cm.Offset,
	}

	if err := c.unmarshalEvent(cm.Value, &event); err != nil {
		c.logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := c.handleCDCEvent(ctx, event); err != nil {
		c.logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}

	return nil
}

func (c *MessageCDCConsumer) unmarshalEvent(value []byte, event *cdc.Event[models.MessageEnvelope]) error {
	return bson.UnmarshalExtJSON(value, true, event)
}

func (c *MessageCDCConsumer) handleCDCEvent(ctx context.Context, event cdc.Event[models.MessageEnvelope]) error {
	if event.FullDocument == nil {
		c.logger.Infow("skipping event with no full document", "offset", event.Offset)
		return nil
	}

	if !event.FullDocument.Meta.IsValid() {
		c.logger.Infow("skipping event with invalid meta", "offset", event.Offset)
		return nil
	}

	// We only expect orders for now
	if event.FullDocument.Meta.DataModel != DataModelOrder {
		c.logger.Infow("unexpected data model", "order", event.FullDocument.Meta, "offset", event.Offset)
		return nil
	}

	switch event.FullDocument.Meta.EventType {
	case EventTypeNewOrder:
		order := models.NewOrder{}
		if err := bson.Unmarshal(event.FullDocument.Message, &order); err != nil {
			c.logger.Errorw("unable to unmarshal order", "offset", event.Offset, zap.Error(err))
			return err
		}

		c.logger.Debugw("successfully unmarshalled new order", "offset", event.Offset, "order", order.Meta)
		c.logger.Debugw("processing new order", "offset", event.Offset, "order", order.Meta)
		return c.orderProcessor.ProcessOrder(ctx, *event.FullDocument, order)
	default:
		c.logger.Infow("unexpected order event type", "order", event.FullDocument.Meta, "offset", event.Offset)
	}

	return nil
}

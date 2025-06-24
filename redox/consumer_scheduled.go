package redox

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/go-common/asyncevents"
)

const (
	scheduledSummaryAndReportsTopic = "clinic.scheduledSummaryAndReportsOrders"
)

type ScheduledSummaryAndReportsCDCConsumerParams struct {
	fx.In

	Logger *zap.SugaredLogger

	Config    ModuleConfig
	Processor ScheduledSummaryAndReportProcessor
}

func NewScheduledSummaryAndReportsRunner(p ScheduledSummaryAndReportsCDCConsumerParams) (asyncevents.Runner, error) {
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

	delays := []time.Duration{0, time.Second * 60, time.Second * 300}
	logger := &cdc.AsynceventsLoggerAdapter{
		SugaredLogger: p.Logger,
	}

	managerConfig := asyncevents.CascadingSaramaEventsManagerConfig{
		Consumer: &ScheduledSummaryAndReportsCDCConsumer{
			Config:    p.Config,
			Logger:    p.Logger,
			Processor: p.Processor,
		},
		Brokers:            config.KafkaBrokers,
		GroupID:            config.KafkaConsumerGroup,
		Topics:             prefixedTopics,
		ConsumptionTimeout: defaultTimeout,
		Delays:             delays,
		Logger:             logger,
		Sarama:             config.SaramaConfig,
	}
	eventsManager := asyncevents.NewCascadingSaramaEventsManager(managerConfig)
	return eventsManager, nil
}

// ScheduledSummaryAndReportsCDCConsumer is kafka consumer for scheduled summary and reports CDC events
type ScheduledSummaryAndReportsCDCConsumer struct {
	Logger *zap.SugaredLogger

	Config    ModuleConfig
	Processor ScheduledSummaryAndReportProcessor
}

func (s *ScheduledSummaryAndReportsCDCConsumer) Consume(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error {
	if msg == nil {
		return nil
	}

	err := s.HandleMessage(ctx, msg)
	if err != nil {
		session.MarkMessage(msg, fmt.Sprintf("I have given up after error: %s", err))
		s.Logger.Warnw("Unable to consume redox message", "error", err)
		return err
	}
	return nil
}

func (s *ScheduledSummaryAndReportsCDCConsumer) HandleMessage(ctx context.Context, cm *sarama.ConsumerMessage) error {
	s.Logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := cdc.Event[ScheduledSummaryAndReport]{
		Offset: cm.Offset,
	}

	if err := UnmarshalEvent(cm.Value, &event); err != nil {
		s.Logger.Warnw("unable to unmarshal message", "offset", cm.Offset, zap.Error(err))
		return err
	}

	if err := s.handleCDCEvent(ctx, event); err != nil {
		s.Logger.Errorw("unable to process cdc event", "offset", cm.Offset, zap.Error(err))
		return err
	}

	return nil
}

func (s *ScheduledSummaryAndReportsCDCConsumer) handleCDCEvent(ctx context.Context, event cdc.Event[ScheduledSummaryAndReport]) error {
	if event.FullDocument == nil {
		s.Logger.Errorw("skipping event with no full document", "offset", event.Offset)
		return nil
	}

	scheduled := event.FullDocument
	if !scheduled.LastMatchedOrder.Meta.IsValid() {
		s.Logger.Errorw("skipping event with invalid meta", "offset", event.Offset)
		return nil
	}

	// We only expect orders for now
	if scheduled.LastMatchedOrder.Meta.DataModel != DataModelOrder {
		s.Logger.Errorw("unexpected data model", "order", scheduled.LastMatchedOrder.Meta, "offset", event.Offset)
		return nil
	}

	switch scheduled.LastMatchedOrder.Meta.EventType {
	case EventTypeNewOrder:
		if err := bson.Unmarshal(scheduled.LastMatchedOrder.Message, &scheduled.DecodedOrder); err != nil {
			s.Logger.Errorw("unable to unmarshal order", "offset", event.Offset, zap.Error(err))
			return err
		}

		s.Logger.Debugw("successfully unmarshalled new order", "offset", event.Offset, "order", scheduled.DecodedOrder.Meta)
		s.Logger.Debugw("processing new order", "offset", event.Offset, "order", scheduled.DecodedOrder.Meta)
		return s.Processor.ProcessOrder(ctx, *scheduled)
	default:
		s.Logger.Infow("unexpected order event type", "order", scheduled.LastMatchedOrder.Meta, "offset", event.Offset)
	}

	return nil
}

func UnmarshalEvent(value []byte, event *cdc.Event[ScheduledSummaryAndReport]) error {
	return bson.UnmarshalExtJSON(value, true, event)
}

package redox

import (
	"context"
	"github.com/IBM/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/go-common/events"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	scheduledSummaryAndReportsTopic = "clinic.scheduledSummaryAndReportsOrders"
)

// ScheduledSummaryAndReportsCDCConsumer is kafka consumer for scheduled summary and reports CDC events
type ScheduledSummaryAndReportsCDCConsumer struct {
	logger *zap.SugaredLogger

	config    ModuleConfig
	processor ScheduledSummaryAndReportProcessor
}

type ScheduledSummaryAndReportsCDCConsumerParams struct {
	fx.In

	Logger *zap.SugaredLogger

	Config    ModuleConfig
	Processor ScheduledSummaryAndReportProcessor
}

func CreateScheduledSummaryAndReportsConsumerGroup(p ScheduledSummaryAndReportsCDCConsumerParams) (events.EventConsumer, error) {
	if !p.Config.Enabled {
		return &cdc.DisabledEventConsumer{}, nil
	}

	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = scheduledSummaryAndReportsTopic

	return events.NewFaultTolerantConsumerGroup(config, NewScheduledSummaryAndReportsConsumer(p))
}

func NewScheduledSummaryAndReportsConsumer(p ScheduledSummaryAndReportsCDCConsumerParams) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewScheduledSummaryAndReportsCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewScheduledSummaryAndReportsCDCConsumer(p ScheduledSummaryAndReportsCDCConsumerParams) (events.MessageConsumer, error) {
	return &ScheduledSummaryAndReportsCDCConsumer{
		logger:    p.Logger,
		config:    p.Config,
		processor: p.Processor,
	}, nil
}

func (s *ScheduledSummaryAndReportsCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (s *ScheduledSummaryAndReportsCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return s.handleMessage(cm)
}

func (s *ScheduledSummaryAndReportsCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	s.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := cdc.Event[ScheduledSummaryAndReport]{
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

func (s *ScheduledSummaryAndReportsCDCConsumer) handleCDCEvent(event cdc.Event[ScheduledSummaryAndReport]) error {
	if event.FullDocument == nil {
		s.logger.Errorw("skipping event with no full document", "offset", event.Offset)
		return nil
	}

	scheduled := event.FullDocument
	if !scheduled.LastMatchedOrder.Meta.IsValid() {
		s.logger.Errorw("skipping event with invalid meta", "offset", event.Offset)
		return nil
	}

	// We only expect orders for now
	if scheduled.LastMatchedOrder.Meta.DataModel != DataModelOrder {
		s.logger.Errorw("unexpected data model", "order", scheduled.LastMatchedOrder.Meta, "offset", event.Offset)
		return nil
	}

	switch scheduled.LastMatchedOrder.Meta.EventType {
	case EventTypeNewOrder:
		if err := bson.Unmarshal(scheduled.LastMatchedOrder.Message, &scheduled.DecodedOrder); err != nil {
			s.logger.Errorw("unable to unmarshal order", "offset", event.Offset, zap.Error(err))
			return err
		}

		s.logger.Debugw("successfully unmarshalled new order", "offset", event.Offset, "order", scheduled.DecodedOrder.Meta)

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		s.logger.Debugw("processing new order", "offset", event.Offset, "order", scheduled.DecodedOrder.Meta)
		return s.processor.ProcessOrder(ctx, *scheduled)
	default:
		s.logger.Infow("unexpected order event type", "order", scheduled.LastMatchedOrder.Meta, "offset", event.Offset)
	}

	return nil
}

func UnmarshalEvent(value []byte, event *cdc.Event[ScheduledSummaryAndReport]) error {
	return bson.UnmarshalExtJSON(value, true, event)
}

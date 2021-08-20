package migration

import (
	"context"
	"encoding/json"
	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"strconv"
)

const (
	migrationsTopic = "clinic.migrations"
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type MigrationCDCConsumer struct {
	logger *zap.SugaredLogger

	migrator Migrator
}

type Params struct {
	fx.In

	Logger   *zap.SugaredLogger
	Migrator Migrator
}

func CreateConsumerGroup(p Params) (events.EventConsumer, error) {
	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = migrationsTopic

	return events.NewFaultTolerantConsumerGroup(config, CreateConsumer(p))
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		delegate, err := NewPatientCDCConsumer(p)
		if err != nil {
			return nil, err
		}
		return cdc.NewRetryingConsumer(delegate), nil
	}
}

func NewPatientCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &MigrationCDCConsumer{
		logger:   p.Logger,
		migrator: p.Migrator,
	}, nil
}

func (p *MigrationCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *MigrationCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	return p.handleMessage(cm)
}

func (p *MigrationCDCConsumer) handleMessage(cm *sarama.ConsumerMessage) error {
	p.logger.Debugw("handling kafka message", "offset", cm.Offset)
	event := MigrationCDCEvent{
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

func (p *MigrationCDCConsumer) unmarshalEvent(value []byte, event *MigrationCDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *MigrationCDCConsumer) handleCDCEvent(event MigrationCDCEvent) error {
	if event.OperationType != cdc.OperationTypeInsert {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	p.logger.Infow("processing event", "event", event)
	return p.migrator.MigratePatients(context.Background(), event.FullDocument.UserId, event.FullDocument.ClinicId)
}

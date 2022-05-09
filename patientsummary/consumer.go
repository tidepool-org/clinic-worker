package patientsummary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"time"
)

const (
	patientsSummaryTopic = "data.summary"
	defaultTimeout       = 30 * time.Second
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: CreateConsumerGroup,
})

type CDCConsumer struct {
	logger *zap.SugaredLogger

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

	config.KafkaTopic = patientsSummaryTopic

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
	event := CDCEvent{
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

func (p *CDCConsumer) unmarshalEvent(value []byte, event *CDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *CDCConsumer) handleCDCEvent(event CDCEvent) error {
	if !event.ShouldApplyUpdates() {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	p.logger.Infow("processing event", "event", event)

	return p.applyPatientSummaryUpdate(event)
}

func (p *CDCConsumer) applyPatientSummaryUpdate(event CDCEvent) error {
	p.logger.Debugw("applying patient summary update", "offset", event.Offset)
	if event.FullDocument.UserId == nil {
		return errors.New("expected user id to be defined")
	}

	userId := clinics.PatientId(*event.FullDocument.UserId)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	updateBody := event.CreateUpdateBody()

	response, err := p.clinics.UpdatePatientSummaryWithResponse(ctx, userId, updateBody)
	if err != nil {
		return err
	}

	if response.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code when updating patient summary %v", response.StatusCode())
	}

	return nil
}

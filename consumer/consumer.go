package consumer

import (
	"encoding/json"
	"errors"
	"github.com/Shopify/sarama"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
)

type PatientCDCConsumer struct {
	shoreline      shoreline.Client
	seagull        clients.Seagull
}

type Params struct {
	fx.In

	Shoreline      shoreline.Client
	Seagull        clients.Seagull
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		return NewPatientCDCConsumer(p)
	}
}

func NewPatientCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &PatientCDCConsumer{
		seagull: p.Seagull,
		shoreline: p.Shoreline,
	}, nil
}

func (p *PatientCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *PatientCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}
	event := PatientCDCEvent{}
	if err := json.Unmarshal(cm.Value, &event); err != nil {
		return err
	}

	return p.handleCDCEvent(event)
}

func (p *PatientCDCConsumer) handleCDCEvent(event PatientCDCEvent) error {
	if !event.ShouldApplyUpdates() {
		return nil
	}

	if err := p.applyProfileUpdate(event); err != nil {
		return err
	}

	return p.applyInviteUpdate(event)
}

func (p *PatientCDCConsumer) applyProfileUpdate(event PatientCDCEvent) error {
	if event.FullDocument.UserId == nil {
		return errors.New("expected patient id to be defined")
	}

	userId := *event.FullDocument.UserId
	profile := make(map[string]interface{}, 0)
	if err := p.seagull.GetCollection(userId, "profile", p.shoreline.TokenProvide(), &profile); err != nil {
		return err
	}

	event.UpdateDescription.ApplyUpdatesToExistingProfile(profile)

	return p.seagull.UpdateCollection(userId, "profile", p.shoreline.TokenProvide(), profile)

}

func (p *PatientCDCConsumer) applyInviteUpdate(event PatientCDCEvent) error {
	return nil
}

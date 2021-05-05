package patients

import (
	"encoding/json"
	"errors"
	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/confirmation"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"strconv"
)

type PatientCDCConsumer struct {
	logger *zap.SugaredLogger

	hydrophone confirmation.Service
	shoreline  shoreline.Client
	seagull    clients.Seagull
}

type Params struct {
	fx.In

	Logger *zap.SugaredLogger

	Hydrophone confirmation.Service
	Shoreline  shoreline.Client
	Seagull    clients.Seagull
}

func CreateConsumer(p Params) events.ConsumerFactory {
	return func() (events.MessageConsumer, error) {
		return NewPatientCDCConsumer(p)
	}
}

func NewPatientCDCConsumer(p Params) (events.MessageConsumer, error) {
	return &PatientCDCConsumer{
		logger:     p.Logger,
		hydrophone: p.Hydrophone,
		seagull:    p.Seagull,
		shoreline:  p.Shoreline,
	}, nil
}

func (p *PatientCDCConsumer) Initialize(config *events.CloudEventsConfig) error {
	return nil
}

func (p *PatientCDCConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	if cm == nil {
		return nil
	}

	p.logger.Debugw("handling kafka message", "offset", cm.Offset)

	event := PatientCDCEvent{
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

func (p *PatientCDCConsumer) unmarshalEvent(value []byte, event *PatientCDCEvent) error {
	message, err := strconv.Unquote(string(value))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(message), event)
}

func (p *PatientCDCConsumer) handleCDCEvent(event PatientCDCEvent) error {
	if !event.ShouldApplyUpdates() {
		p.logger.Debugw("skipping handling of event", "offset", event.Offset)
		return nil
	}

	if err := p.applyProfileUpdate(event); err != nil {
		return err
	}

	return p.applyInviteUpdate(event)
}

func (p *PatientCDCConsumer) applyProfileUpdate(event PatientCDCEvent) error {
	p.logger.Debugw("applying profile update", "event", event)
	if event.FullDocument.UserId == nil {
		return errors.New("expected patient id to be defined")
	}

	userId := *event.FullDocument.UserId
	profile := make(map[string]interface{}, 0)
	if err := p.seagull.GetCollection(userId, "profile", p.shoreline.TokenProvide(), &profile); err != nil {
		p.logger.Errorw("unable to fetch user profile", "offset", event.Offset, zap.Error(err))
		return err
	}

	event.ApplyUpdatesToExistingProfile(profile)
	err := p.seagull.UpdateCollection(userId, "profile", p.shoreline.TokenProvide(), profile)
	if err != nil {
		p.logger.Errorw("unable to update patient profile",
			"offset", event.Offset,
			"profile", profile,
			zap.Error(err),
		)
	}

	return err
}

func (p *PatientCDCConsumer) applyInviteUpdate(event PatientCDCEvent) error {
	p.logger.Debugw("applying invite update", "offset", event.Offset)
	if event.FullDocument.UserId == nil {
		return errors.New("expected patient id to be defined")
	}

	userId := *event.FullDocument.UserId
	if err := p.hydrophone.UpsertSignUpInvite(userId); err != nil {
		p.logger.Warnw("unable to upsert sign up invite", "offset", event.Offset, zap.Error(err))
		return err
	}

	return nil
}

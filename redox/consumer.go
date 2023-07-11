package redox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/redox/models"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
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

			p.logger.Debugw("successfully unmarshalled new order", "offset", event.Offset, "order", order.Meta)
			return p.handleOrder(order)
		}
	}

	return nil
}

func (p *RedoxCDCConsumer) handleOrder(order models.NewOrder) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	criteria := clinics.MatchClinicAndPatientJSONRequestBody{
		Clinic: clinics.EHRMatchClinicRequest{
			SourceId: *order.Meta.Source.ID,
		},
		Patient: &clinics.EHRMatchPatientRequest{},
	}
	if order.Order.OrderingFacility != nil && order.Order.OrderingFacility.Name != nil {
		criteria.Clinic.FacilityName = order.Order.OrderingFacility.Name
	}

	for _, identifier := range order.Patient.Identifiers {
		if identifier.IDType == "MR" {
			criteria.Patient.Mrn = identifier.ID
			break
		}
	}
	if criteria.Patient.Mrn == "" {
		p.logger.Warnw("unable to find MRN for order", "order", order.Meta)
		return nil
	}
	if order.Patient.Demographics == nil {
		p.logger.Warnw("unable to find patient demographics for order", "order", order.Meta)
		return nil
	}
	if order.Patient.Demographics.DOB == nil || *order.Patient.Demographics.DOB == "" {
		p.logger.Warnw("unable to find patient DOB for order", "order", order.Meta)
		return nil
	}
	if order.Patient.Demographics.FirstName == nil || *order.Patient.Demographics.FirstName == "" {
		p.logger.Warnw("unable to find patient first name for order", "order", order.Meta)
		return nil
	}
	if order.Patient.Demographics.LastName == nil || *order.Patient.Demographics.LastName == "" {
		p.logger.Warnw("unable to find patient last name for order", "order", order.Meta)
		return nil
	}

	criteria.Patient.DateOfBirth = *order.Patient.Demographics.DOB
	criteria.Patient.FirstName = order.Patient.Demographics.FirstName
	criteria.Patient.LastName = order.Patient.Demographics.LastName
	criteria.Patient.MiddleName = order.Patient.Demographics.MiddleName

	response, err := p.clinics.MatchClinicAndPatientWithResponse(ctx, criteria)
	if err != nil {
		p.logger.Warnw("unable to match clinic and patient", "order", order.Meta, zap.Error(err))
		return err
	}
	if response.StatusCode() != http.StatusOK {
		p.logger.Warnw("unable to match clinic and patient", "order", order.Meta, "status", response.StatusCode())
		return fmt.Errorf("unable to match clinic and patient. unexpected response: %d", response.StatusCode())
	}
	if response.JSON200 == nil {
		return fmt.Errorf("unable to match clinic and patient: %d", errors.New("response body is nil"))
	}
	match := response.JSON200
	if match.Patients == nil || len(*match.Patients) == 0 {
		p.logger.Warnw("no matching patients were found", "order", order.Meta, "clinicId", match.Clinic.Id)
	} else if len(*match.Patients) > 1 {
		p.logger.Warnw(fmt.Sprintf("%v patients were found matching the order", len(*match.Patients)), "order", order.Meta, "clinicId", match.Clinic.Id)
	}

	patient := (*match.Patients)[0]
	p.logger.Infow("successfully matched clinic and patient", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)

	return nil
}

package redox

import (
	"fmt"
	"slices"
	"time"

	"github.com/tidepool-org/clinic-worker/types"
	models "github.com/tidepool-org/clinic/redox_models"
)

const (
	EventTypeNewFlowsheet = "New"
	DataModelFlowsheet    = "Flowsheet"

	AdditionalIdentifierExtensionId string = "additional-identifier"
	AdditionalIdentifierURI         string = "https://api.redoxengine.com/extensions/additional-identifier"
	AdditionalIdentifierTypeOrderId string = "orderId"

	AdditionalProviderInfoExtensionId string = "additional-provider-info"
	AdditionalProviderInfoURI         string = "https://api.redoxengine.com/extensions/additional-provider-info"

	days14 = 14 * 24 * time.Hour
)

type AdditionalIdentifierExtension struct {
	URL        string               `json:"url"`
	Identifier AdditionalIdentifier `json:"identifier"`
}

type AdditionalIdentifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type AdditionalProviderExtension struct {
	URL          string                          `json:"url"`
	Participants []AdditionalProviderParticipant `json:"participants"`
}

type AdditionalProviderParticipant struct {
	Id     string `json:"id"`
	IdType string `json:"idType"`
	Person struct {
		Name struct {
			Given  []string `json:"given"`
			Family string   `json:"family"`
		} `json:"name"`
	} `json:"person"`
}

func NewFlowsheet() models.NewFlowsheet {
	flowsheet := models.NewFlowsheet{}
	now := time.Now().Format(time.RFC3339)

	flowsheet.Meta.EventType = EventTypeNewFlowsheet
	flowsheet.Meta.DataModel = DataModelFlowsheet
	flowsheet.Meta.EventDateTime = &now
	return flowsheet
}

// Observation is a single flowsheet observation. The clinic service computes,
// converts and formats all summary-statistic values; the worker maps the
// service's response onto these structs without any further formatting.
type Observation struct {
	Code        string
	Value       string
	ValueType   string
	Units       *string
	DateTime    string
	Description string
}

func AppendObservation(f *models.NewFlowsheet, o *Observation) {
	observation := types.NewItemForSlice(f.Observations)
	observation.Code = o.Code
	observation.Value = o.Value
	observation.ValueType = o.ValueType
	observation.Units = o.Units
	observation.Description = &o.Description
	observation.DateTime = o.DateTime
	f.Observations = append(f.Observations, observation)
}

func SetVisitNumberInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Visit != nil && order.Visit.VisitNumber != nil {
		if flowsheet.Visit == nil {
			flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
		}
		flowsheet.Visit.VisitNumber = order.Visit.VisitNumber
	}
}

func SetVisitLocationInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Visit == nil {
		return
	}

	if flowsheet.Visit == nil {
		flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
	}
	flowsheet.Visit.Location = order.Visit.Location
}

func SetAccountNumberInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Visit != nil && order.Visit.AccountNumber != nil {
		if flowsheet.Visit == nil {
			flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
		}
		flowsheet.Visit.AccountNumber = order.Visit.AccountNumber
	}
}

func SetOrderIdInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Order.ID != "" {
		extensions := map[string]any{
			AdditionalIdentifierExtensionId: AdditionalIdentifierExtension{
				URL: AdditionalIdentifierURI,
				Identifier: AdditionalIdentifier{
					Type:  AdditionalIdentifierTypeOrderId,
					Value: order.Order.ID,
				},
			}}
		flowsheet.Visit.Extensions = &extensions
	}
}

func SetProviderInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Order.Provider == nil {
		return
	}
	if order.Order.Provider.ID == nil {
		return
	}
	if order.Order.Provider.FirstName == nil && order.Order.Provider.LastName == nil {
		return
	}

	participant := AdditionalProviderParticipant{
		Id: *order.Order.Provider.ID,
	}

	if order.Order.Provider.IDType != nil {
		participant.IdType = *order.Order.Provider.IDType
	}
	if order.Order.Provider.FirstName != nil {
		participant.Person.Name.Given = []string{*order.Order.Provider.FirstName}
	}
	if order.Order.Provider.LastName != nil {
		participant.Person.Name.Family = *order.Order.Provider.LastName
	}

	initVisitExtensions(flowsheet)
	(*flowsheet.Visit.Extensions)[AdditionalProviderInfoExtensionId] = AdditionalProviderExtension{
		URL:          AdditionalProviderInfoURI,
		Participants: []AdditionalProviderParticipant{participant},
	}
}

func initVisitExtensions(flowsheet *models.NewFlowsheet) {
	if flowsheet.Visit == nil {
		flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
	}
	if flowsheet.Visit.Extensions == nil {
		extensions := make(map[string]any)
		flowsheet.Visit.Extensions = &extensions
	}
}

func ObservationsToGMINoteComponents(observations []*Observation) []NoteComponent {
	gmiObservations := slices.DeleteFunc(slices.Clone(observations), func(o *Observation) bool {
		return o.Code != "GLUCOSE_MANAGEMENT_INDICATOR"
	})
	var components []NoteComponent
	for _, observation := range gmiObservations {
		components = append(components, ObservationToGMINoteComponent(observation))
	}
	return components
}

func ObservationToGMINoteComponent(o *Observation) NoteComponent {
	dateTimeComment := fmt.Sprintf("DateTime Observed: %s", o.DateTime)
	return NoteComponent{
		Comments: dateTimeComment,
		ID:       o.Code,
		Name:     o.Description,
		Value:    o.Value,
	}
}

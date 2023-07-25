package redox

import (
	"github.com/tidepool-org/clinic/redox/models"
	"time"
)

const (
	EventTypeNewResults              = "New"
	DataModelResults                 = "Results"
	OrderStatusResulted              = "Resulted"
	MatchingResultCode               = "MATCHING_RESULT"
	MatchingResultMessageCode        = "MATCHING_RESULT_MESSAGE"
	MatchingResultDescription        = "Indicates whether the order was successfully matched"
	MatchingResultMessageDescription = "Message indicating the result of the matching process"

	NoMatchingPatientsMessage       = "No matching patients were found"
	MultipleMatchingPatientsMessage = "Multiple matching patients were found"
	SuccessfulMatchingMessage       = "Patient was successfully matched"
)

type MatchingResult struct {
	IsSuccess bool
	Message   string
}

func NewResults() models.NewResults {
	res := models.NewResults{}
	now := time.Now().Format(time.RFC3339)

	res.Meta.EventType = EventTypeNewResults
	res.Meta.DataModel = DataModelResults
	res.Meta.EventDateTime = &now

	return res
}

func SetResultsPatientFromOrder(order models.NewOrder, results *models.NewResults) {
	results.Patient.Identifiers = order.Patient.Identifiers
	results.Patient.Demographics = order.Patient.Demographics
}

func SetMatchingResult(matching MatchingResult, order models.NewOrder, results *models.NewResults) {
	results.Orders = []struct {
		ApplicationOrderID *string        `json:"ApplicationOrderID"`
		CollectionDateTime *string        `json:"CollectionDateTime"`
		CompletionDateTime *string        `json:"CompletionDateTime"`
		ID                 string         `json:"ID"`
		Notes              *[]interface{} `json:"Notes,omitempty"`
		Priority           *string        `json:"Priority"`
		Procedure          *struct {
			Code        *string `json:"Code"`
			Codeset     *string `json:"Codeset"`
			Description *string `json:"Description"`
		} `json:"Procedure,omitempty"`
		Provider *struct {
			Address *struct {
				City          *string `json:"City"`
				Country       *string `json:"Country"`
				County        *string `json:"County"`
				State         *string `json:"State"`
				StreetAddress *string `json:"StreetAddress"`
				ZIP           *string `json:"ZIP"`
			} `json:"Address,omitempty"`
			Credentials    *[]interface{} `json:"Credentials,omitempty"`
			EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
			FirstName      *string        `json:"FirstName"`
			ID             *string        `json:"ID"`
			IDType         *string        `json:"IDType"`
			LastName       *string        `json:"LastName"`
			Location       *struct {
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			NPI         *string `json:"NPI"`
			PhoneNumber *struct {
				Office *string `json:"Office"`
			} `json:"PhoneNumber,omitempty"`
		} `json:"Provider,omitempty"`
		ResponseFlag        *string `json:"ResponseFlag"`
		ResultCopyProviders *[]struct {
			Address *struct {
				City          *string `json:"City"`
				Country       *string `json:"Country"`
				County        *string `json:"County"`
				State         *string `json:"State"`
				StreetAddress *string `json:"StreetAddress"`
				ZIP           *string `json:"ZIP"`
			} `json:"Address,omitempty"`
			Credentials    *[]interface{} `json:"Credentials,omitempty"`
			EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
			FirstName      *string        `json:"FirstName"`
			ID             *string        `json:"ID"`
			IDType         *string        `json:"IDType"`
			LastName       *string        `json:"LastName"`
			Location       *struct {
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			PhoneNumber *struct {
				Office *string `json:"Office"`
			} `json:"PhoneNumber,omitempty"`
		} `json:"ResultCopyProviders,omitempty"`
		Results []struct {
			AbnormalFlag       *string        `json:"AbnormalFlag"`
			Code               string         `json:"Code"`
			Codeset            *string        `json:"Codeset"`
			CompletionDateTime *string        `json:"CompletionDateTime"`
			Description        *string        `json:"Description"`
			FileType           *string        `json:"FileType"`
			Notes              *[]interface{} `json:"Notes,omitempty"`
			ObservationMethod  *struct {
				Code        *string `json:"Code"`
				Codeset     *string `json:"Codeset"`
				Description *string `json:"Description"`
			} `json:"ObservationMethod,omitempty"`
			Performer *struct {
				Address *struct {
					City          *string `json:"City"`
					Country       *string `json:"Country"`
					County        *string `json:"County"`
					State         *string `json:"State"`
					StreetAddress *string `json:"StreetAddress"`
					ZIP           *string `json:"ZIP"`
				} `json:"Address,omitempty"`
				Credentials    *[]interface{} `json:"Credentials,omitempty"`
				EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
				FirstName      *string        `json:"FirstName"`
				ID             *string        `json:"ID"`
				IDType         *string        `json:"IDType"`
				LastName       *string        `json:"LastName"`
				Location       *struct {
					Department            *string `json:"Department"`
					DepartmentIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"DepartmentIdentifiers,omitempty"`
					Facility            *string `json:"Facility"`
					FacilityIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"FacilityIdentifiers,omitempty"`
					Room *string `json:"Room"`
					Type *string `json:"Type"`
				} `json:"Location,omitempty"`
				PhoneNumber *struct {
					Office *string `json:"Office"`
				} `json:"PhoneNumber,omitempty"`
			} `json:"Performer,omitempty"`
			PrimaryResultsInterpreter *struct {
				Address *struct {
					City          *string `json:"City"`
					Country       *string `json:"Country"`
					County        *string `json:"County"`
					State         *string `json:"State"`
					StreetAddress *string `json:"StreetAddress"`
					ZIP           *string `json:"ZIP"`
				} `json:"Address,omitempty"`
				Credentials    *[]interface{} `json:"Credentials,omitempty"`
				EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
				FirstName      *string        `json:"FirstName"`
				ID             *string        `json:"ID"`
				IDType         *string        `json:"IDType"`
				LastName       *string        `json:"LastName"`
				Location       *struct {
					Department            *string `json:"Department"`
					DepartmentIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"DepartmentIdentifiers,omitempty"`
					Facility            *string `json:"Facility"`
					FacilityIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"FacilityIdentifiers,omitempty"`
					Room *string `json:"Room"`
					Type *string `json:"Type"`
				} `json:"Location,omitempty"`
				NPI         *string `json:"NPI"`
				PhoneNumber *struct {
					Office *string `json:"Office"`
				} `json:"PhoneNumber,omitempty"`
			} `json:"PrimaryResultsInterpreter,omitempty"`
			Producer *struct {
				Address *struct {
					City          *string `json:"City"`
					Country       *string `json:"Country"`
					County        *string `json:"County"`
					State         *string `json:"State"`
					StreetAddress *string `json:"StreetAddress"`
					ZIP           *string `json:"ZIP"`
				} `json:"Address,omitempty"`
				ID     *string `json:"ID"`
				IDType *string `json:"IDType"`
				Name   *string `json:"Name"`
			} `json:"Producer,omitempty"`
			ReferenceRange *struct {
				High *float32 `json:"High"`
				Low  *float32 `json:"Low"`
				Text *string  `json:"Text"`
			} `json:"ReferenceRange,omitempty"`
			RelatedGroupID *string `json:"RelatedGroupID"`
			Specimen       *struct {
				BodySite *string `json:"BodySite"`
				ID       *string `json:"ID"`
				Source   *string `json:"Source"`
			} `json:"Specimen,omitempty"`
			Status    *string `json:"Status"`
			Units     *string `json:"Units"`
			Value     string  `json:"Value"`
			ValueType string  `json:"ValueType"`
		} `json:"Results"`
		ResultsStatus       *string `json:"ResultsStatus"`
		Status              string  `json:"Status"`
		TransactionDateTime *string `json:"TransactionDateTime"`
	}{{}}

	results.Orders[0].ID = order.Order.ID
	results.Orders[0].Status = OrderStatusResulted

	results.Orders[0].Results = []struct {
		AbnormalFlag       *string        `json:"AbnormalFlag"`
		Code               string         `json:"Code"`
		Codeset            *string        `json:"Codeset"`
		CompletionDateTime *string        `json:"CompletionDateTime"`
		Description        *string        `json:"Description"`
		FileType           *string        `json:"FileType"`
		Notes              *[]interface{} `json:"Notes,omitempty"`
		ObservationMethod  *struct {
			Code        *string `json:"Code"`
			Codeset     *string `json:"Codeset"`
			Description *string `json:"Description"`
		} `json:"ObservationMethod,omitempty"`
		Performer *struct {
			Address *struct {
				City          *string `json:"City"`
				Country       *string `json:"Country"`
				County        *string `json:"County"`
				State         *string `json:"State"`
				StreetAddress *string `json:"StreetAddress"`
				ZIP           *string `json:"ZIP"`
			} `json:"Address,omitempty"`
			Credentials    *[]interface{} `json:"Credentials,omitempty"`
			EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
			FirstName      *string        `json:"FirstName"`
			ID             *string        `json:"ID"`
			IDType         *string        `json:"IDType"`
			LastName       *string        `json:"LastName"`
			Location       *struct {
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			PhoneNumber *struct {
				Office *string `json:"Office"`
			} `json:"PhoneNumber,omitempty"`
		} `json:"Performer,omitempty"`
		PrimaryResultsInterpreter *struct {
			Address *struct {
				City          *string `json:"City"`
				Country       *string `json:"Country"`
				County        *string `json:"County"`
				State         *string `json:"State"`
				StreetAddress *string `json:"StreetAddress"`
				ZIP           *string `json:"ZIP"`
			} `json:"Address,omitempty"`
			Credentials    *[]interface{} `json:"Credentials,omitempty"`
			EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
			FirstName      *string        `json:"FirstName"`
			ID             *string        `json:"ID"`
			IDType         *string        `json:"IDType"`
			LastName       *string        `json:"LastName"`
			Location       *struct {
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			NPI         *string `json:"NPI"`
			PhoneNumber *struct {
				Office *string `json:"Office"`
			} `json:"PhoneNumber,omitempty"`
		} `json:"PrimaryResultsInterpreter,omitempty"`
		Producer *struct {
			Address *struct {
				City          *string `json:"City"`
				Country       *string `json:"Country"`
				County        *string `json:"County"`
				State         *string `json:"State"`
				StreetAddress *string `json:"StreetAddress"`
				ZIP           *string `json:"ZIP"`
			} `json:"Address,omitempty"`
			ID     *string `json:"ID"`
			IDType *string `json:"IDType"`
			Name   *string `json:"Name"`
		} `json:"Producer,omitempty"`
		ReferenceRange *struct {
			High *float32 `json:"High"`
			Low  *float32 `json:"Low"`
			Text *string  `json:"Text"`
		} `json:"ReferenceRange,omitempty"`
		RelatedGroupID *string `json:"RelatedGroupID"`
		Specimen       *struct {
			BodySite *string `json:"BodySite"`
			ID       *string `json:"ID"`
			Source   *string `json:"Source"`
		} `json:"Specimen,omitempty"`
		Status    *string `json:"Status"`
		Units     *string `json:"Units"`
		Value     string  `json:"Value"`
		ValueType string  `json:"ValueType"`
	}{{}, {}}

	description := MatchingResultDescription
	results.Orders[0].Results[0].Code = MatchingResultCode
	results.Orders[0].Results[0].Description = &description

	results.Orders[0].Results[0].ValueType = "String"
	if matching.IsSuccess {
		results.Orders[0].Results[0].Value = "SUCCESS"
	} else {
		results.Orders[0].Results[0].Value = "FAILURE"
	}

	messageDescription := MatchingResultMessageDescription
	results.Orders[0].Results[1].Code = MatchingResultMessageCode
	results.Orders[0].Results[1].Description = &messageDescription
	results.Orders[0].Results[1].ValueType = "String"
	results.Orders[0].Results[1].Value = matching.Message
}

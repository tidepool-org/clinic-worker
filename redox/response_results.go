package redox

import (
	models "github.com/tidepool-org/clinic/redox_models"
	"time"
)

const (
	EventTypeNewResults               = "New"
	DataModelResults                  = "Results"
	OrderStatusResulted               = "Resulted"
	AccountCreationResultCode         = "ACCOUNT_CREATION_RESULT"
	AccountCreationResultDescription  = "Indicates whether the account was successfully created"
	AccountCreationMessageCode        = "ACCOUNT_CREATION_RESULT_MESSAGE"
	AccountCreationMessageDescription = "Message indicating the result of the account creation"

	MatchingResultCode               = "MATCHING_RESULT"
	MatchingResultMessageCode        = "MATCHING_RESULT_MESSAGE"
	MatchingResultDescription        = "Indicates whether the order was successfully matched"
	MatchingResultMessageDescription = "Message indicating the result of the matching process"

	NoMatchingPatientsMessage       = "No matching patients were found"
	MultipleMatchingPatientsMessage = "Multiple matching patients were found"
	SuccessfulMatchingMessage       = "Patient was successfully matched"

	SuccessfulAccountCreationMessage = "Account was successfully created"
)

type ResultsNotification struct {
	IsSuccess bool
	Message   string
}

type NotificationFields struct {
	MessageCode                string
	MessageDescription         string
	OperationResultCode        string
	OperationResultDescription string
}

var (
	accountCreationNotificationFields = NotificationFields{
		MessageCode:                AccountCreationMessageCode,
		MessageDescription:         AccountCreationMessageDescription,
		OperationResultCode:        AccountCreationResultCode,
		OperationResultDescription: AccountCreationResultDescription,
	}
	patientMatchingNotificationFields = NotificationFields{
		MessageCode:                MatchingResultMessageCode,
		MessageDescription:         MatchingResultMessageDescription,
		OperationResultCode:        MatchingResultCode,
		OperationResultDescription: MatchingResultDescription,
	}
)

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

func SetMatchingResult(notification ResultsNotification, order models.NewOrder, results *models.NewResults) {
	SetNotificationResult(notification, patientMatchingNotificationFields, order, results)
}

func SetAccountCreationResults(notification ResultsNotification, order models.NewOrder, results *models.NewResults) {
	SetNotificationResult(notification, accountCreationNotificationFields, order, results)
}

func SetNotificationResult(notification ResultsNotification, fields NotificationFields, order models.NewOrder, results *models.NewResults) {
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

	results.Orders[0].Results[0].Code = fields.OperationResultCode
	results.Orders[0].Results[0].Description = &fields.OperationResultDescription

	results.Orders[0].Results[0].ValueType = "String"
	if notification.IsSuccess {
		results.Orders[0].Results[0].Value = "SUCCESS"
	} else {
		results.Orders[0].Results[0].Value = "FAILURE"
	}

	results.Orders[0].Results[1].Code = fields.MessageCode
	results.Orders[0].Results[1].Description = &fields.MessageDescription
	results.Orders[0].Results[1].ValueType = "String"
	results.Orders[0].Results[1].Value = notification.Message
}

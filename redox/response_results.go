package redox

import (
	"time"

	"github.com/tidepool-org/clinic-worker/types"
	models "github.com/tidepool-org/clinic/redox_models"
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

var (
	OrderResultsStatusFinal = "Final"
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
	now := time.Now().Format(time.RFC3339)

	results.Orders = types.NewSlice(results.Orders, 1)

	results.Orders[0].ID = order.Order.ID
	results.Orders[0].Status = OrderStatusResulted
	results.Orders[0].CompletionDateTime = &now
	results.Orders[0].Procedure = order.Order.Procedure
	results.Orders[0].Provider = order.Order.Provider
	results.Orders[0].ResultsStatus = &OrderResultsStatusFinal

	results.Orders[0].Results = types.NewSlice(results.Orders[0].Results, 2)

	results.Orders[0].Results[0].Code = fields.OperationResultCode
	results.Orders[0].Results[0].CompletionDateTime = &now
	results.Orders[0].Results[0].Description = &fields.OperationResultDescription
	results.Orders[0].Results[0].ValueType = "String"
	if notification.IsSuccess {
		results.Orders[0].Results[0].Value = "SUCCESS"
	} else {
		results.Orders[0].Results[0].Value = "FAILURE"
	}
	results.Orders[0].Results[0].Status = &OrderResultsStatusFinal

	results.Orders[0].Results[1].Code = fields.MessageCode
	results.Orders[0].Results[1].CompletionDateTime = &now
	results.Orders[0].Results[1].Description = &fields.MessageDescription
	results.Orders[0].Results[1].ValueType = "String"
	results.Orders[0].Results[1].Value = notification.Message
	results.Orders[0].Results[1].Status = &OrderResultsStatusFinal
}

func SetVisitNumberInResult(order models.NewOrder, result *models.NewResults) {
	if order.Visit != nil && order.Visit.VisitNumber != nil {
		if result.Visit == nil {
			result.Visit = types.NewStructPtr(result.Visit)
		}
		result.Visit.VisitNumber = order.Visit.VisitNumber
	}
}

func SetVisitLocationInResult(order models.NewOrder, result *models.NewResults) {
	if order.Visit == nil {
		return
	}

	if result.Visit == nil {
		result.Visit = types.NewStructPtr(result.Visit)
	}
	result.Visit.Location = order.Visit.Location
}

func SetAccountNumberInResult(order models.NewOrder, result *models.NewResults) {
	if order.Visit != nil && order.Visit.AccountNumber != nil {
		if result.Visit == nil {
			result.Visit = types.NewStructPtr(result.Visit)
		}
		result.Visit.AccountNumber = order.Visit.AccountNumber
	}
}

package redox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/oapi-codegen/runtime/types"
	"github.com/tidepool-org/clinic-worker/report"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"go.uber.org/zap"
)

const (
	EventTypeNewOrder               = "New"
	DataModelOrder                  = "Order"
	MinimumAgeSelfOwnedAccountYears = 13
)

type NewOrderProcessor interface {
	ProcessOrder(ctx context.Context, envelope models.MessageEnvelope, order models.NewOrder) error
	SendSummaryAndReport(ctx context.Context, params SummaryAndReportParameters) error
}

type SummaryAndReportParameters struct {
	Match               clinics.EHRMatchResponse
	Order               models.NewOrder
	DocumentId          string
	PrecedingDocumentId string
}

func (s SummaryAndReportParameters) ShouldReplacePrecedingReport() bool {
	if s.Match.Settings.ScheduledReports.OnUploadNoteEventType != nil &&
		*s.Match.Settings.ScheduledReports.OnUploadNoteEventType == clinics.ScheduledReportsOnUploadNoteEventTypeReplace &&
		s.PrecedingDocumentId != "" {
		return true
	}
	return false
}

var (
	ErrNoMatchingPatients       = fmt.Errorf("no matching patient")
	ErrMultipleMatchingPatients = fmt.Errorf("multiple matching patients")
)

func (s SummaryAndReportParameters) GetMatchingPatient() (p clinics.Patient, err error) {
	if s.Match.Patients == nil || len(*s.Match.Patients) == 0 {
		err = ErrNoMatchingPatients
	} else if len(*s.Match.Patients) > 1 {
		err = ErrMultipleMatchingPatients
	} else {
		p = (*s.Match.Patients)[0]
	}
	return
}

type newOrderProcessor struct {
	logger *zap.SugaredLogger

	clinics         clinics.ClientWithResponsesInterface
	client          Client
	reportGenerator report.Generator
	shorelineClient shoreline.Client
}

func NewNewOrderProcessor(clinics clinics.ClientWithResponsesInterface, redox Client, reportGenerator report.Generator, shorelineClient shoreline.Client, logger *zap.SugaredLogger) NewOrderProcessor {
	return &newOrderProcessor{
		logger:          logger,
		clinics:         clinics,
		client:          redox,
		reportGenerator: reportGenerator,
		shorelineClient: shorelineClient,
	}
}

func (o *newOrderProcessor) ProcessOrder(ctx context.Context, envelope models.MessageEnvelope, order models.NewOrder) error {
	matchRequest := clinics.EHRMatchRequest{
		MessageRef: &clinics.EHRMatchMessageRef{
			DocumentId: envelope.Id.Hex(),
			DataModel:  clinics.EHRMatchMessageRefDataModel(order.Meta.DataModel),
			EventType:  clinics.EHRMatchMessageRefEventType(order.Meta.EventType),
		},
	}
	response, err := o.clinics.MatchClinicAndPatientWithResponse(ctx, matchRequest)
	if err != nil {
		o.logger.Warnw("unable to match", "order", order.Meta, zap.Error(err))
		// Return an error so we can retry the request
		return err
	}

	if response.StatusCode() != http.StatusOK {
		o.logger.Warnw("unable to match clinic and patient", "order", order.Meta, "status", response.StatusCode())
		// Return an error so we can retry the request
		return fmt.Errorf("unable to match clinic and patient. unexpected response: %d", response.StatusCode())
	}

	if response.JSON200 == nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to match clinic and patient: %d", errors.New("response body is nil"))
	}

	match := response.JSON200
	procedureCode := GetProcedureCode(order)

	params := SummaryAndReportParameters{
		Match: *match,
		Order: order,
	}

	if ProcedureCodesMatch(procedureCode, match.Settings.ProcedureCodes.EnableSummaryReports) {
		return o.handleEnableSummaryReports(ctx, params)
	} else if ProcedureCodesMatch(procedureCode, match.Settings.ProcedureCodes.DisableSummaryReports) {
		return o.handleDisableSummaryReports(ctx, params)
	} else if ProcedureCodesMatch(procedureCode, match.Settings.ProcedureCodes.CreateAccount) {
		return o.handleCreateAccount(ctx, order, *match)
	}

	return o.handleUnknownProcedure(ctx, order, *match)
}

func GetProcedureCode(order models.NewOrder) string {
	var procedureCode string
	if order.Order.Procedure != nil && order.Order.Procedure.Code != nil {
		procedureCode = *order.Order.Procedure.Code
	}
	return procedureCode
}

func ProcedureCodesMatch(code string, configuration *string) bool {
	if code == "" || configuration == nil || *configuration == "" {
		return false
	}
	return code == *configuration
}

func (o *newOrderProcessor) handleEnableSummaryReports(ctx context.Context, params SummaryAndReportParameters) error {
	if params.Match.Patients == nil || len(*params.Match.Patients) == 0 {
		return o.handleNoMatchingPatients(ctx, params)
	} else if len(*params.Match.Patients) > 1 {
		return o.handleMultipleMatchingPatients(ctx, params)
	}

	patient := (*params.Match.Patients)[0]
	o.logger.Infow("successfully matched clinic and patient", "order", params.Order.Meta, "clinicId", params.Match.Clinic.Id, "patientId", patient.Id)
	if err := o.handleSuccessfulPatientMatch(ctx, params); err != nil {
		return err
	}

	return o.SendSummaryAndReport(ctx, params)
}

func (o *newOrderProcessor) handleDisableSummaryReports(ctx context.Context, params SummaryAndReportParameters) error {
	patient, err := params.GetMatchingPatient()
	if errors.Is(err, ErrNoMatchingPatients) {
		return o.handleNoMatchingPatients(ctx, params)
	} else if errors.Is(err, ErrMultipleMatchingPatients) {
		return o.handleMultipleMatchingPatients(ctx, params)
	} else if err != nil {
		return err
	}

	o.logger.Infow("successfully matched clinic and patient", "order", params.Order.Meta, "clinicId", params.Match.Clinic.Id, "patientId", patient.Id)
	return o.handleSuccessfulPatientMatch(ctx, params)
}

func (o *newOrderProcessor) handleCreateAccount(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	if match.Patients != nil && len(*match.Patients) > 0 {
		ids := make([]string, len(*match.Patients))
		for i, patient := range *match.Patients {
			ids[i] = *patient.Id
		}

		o.logger.Infow(
			"unable to create new patient account, because matching patients were found",
			"order", order.Meta,
			"clinicId", match.Clinic.Id,
			"patientIds", strings.Join(ids, ","),
		)

		err := fmt.Errorf("patient already exists")
		return o.handleAccountCreationError(ctx, err, order, match)
	}

	permission := make(map[string]interface{})
	createPatient := clinics.CreatePatientAccountJSONRequestBody{
		Permissions: &clinics.PatientPermissions{
			Custodian: &permission,
			Note:      &permission,
			Upload:    &permission,
			View:      &permission,
		},
	}

	var err error
	createPatient.Email, err = GetEmailAddressFromOrder(order)
	if err != nil {
		return o.handleAccountCreationError(ctx, err, order, match)
	}
	createPatient.BirthDate, err = GetBirthDateFromOrder(order)
	if err != nil {
		return o.handleAccountCreationError(ctx, err, order, match)
	}
	createPatient.FullName, err = GetFullNameFromOrder(order)
	if err != nil {
		return o.handleAccountCreationError(ctx, err, order, match)
	}
	createPatient.Mrn, err = GetMrnFromOrder(order)
	if err != nil {
		return o.handleAccountCreationError(ctx, err, order, match)
	}

	if createPatient.Email != nil {
		if exists, err := o.emailExists(*createPatient.Email); err != nil {
			o.logger.Errorw("unexpected error when checking for duplicate emails", "order", order.Meta, "error", err)
			return err
		} else if exists {
			err = fmt.Errorf("the email address is already in use")
			return o.handleAccountCreationError(ctx, err, order, match)
		}
	}

	resp, err := o.clinics.CreatePatientAccountWithResponse(ctx, *match.Clinic.Id, createPatient)
	if err != nil {
		// Retry in case of unexpected failure
		o.logger.Errorw("unable to create patient account", "order", order.Meta, "error", err)
		return err
	}
	if (resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusConflict) || resp.JSON200 == nil {
		// Retry in case of failure
		o.logger.Errorw("unexpected response when creating patient account", "order", order.Meta, "statusCode", resp.StatusCode())
		return err
	}

	o.logger.Infow("patient account was successfully created", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", resp.JSON200.Id)
	return o.handleAccountCreationSuccess(ctx, order, match)
}

func (o *newOrderProcessor) emailExists(email string) (bool, error) {
	user, err := o.shorelineClient.GetUser(email, o.shorelineClient.TokenProvide())
	if user != nil && len(user.UserID) > 0 {
		return true, nil
	}
	if err != nil {
		statusErr := &status.StatusError{}
		if ok := errors.As(err, &statusErr); ok && statusErr.Code == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return false, nil
}

func (o *newOrderProcessor) SendSummaryAndReport(ctx context.Context, params SummaryAndReportParameters) error {
	patient, err := params.GetMatchingPatient()
	if err != nil {
		return err
	}
	flowsheet, err := o.createSummaryStatisticsFlowsheet(params)
	if err != nil {
		return err
	}
	notes, err := o.createReportNote(ctx, params)
	if err != nil {
		// return the error so we can retry the request
		return err
	}

	o.logger.Infow("sending flowsheet", "order", params.Order.Meta, "clinicId", params.Match.Clinic.Id, "patientId", patient.Id)
	if err := o.client.Send(ctx, flowsheet); err != nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to send flowsheet: %w", err)
	}

	if notes != nil {
		o.logger.Infow("sending note", "order", params.Order.Meta, "clinicId", params.Match.Clinic.Id, "patientId", patient.Id)
		if err := o.client.Send(ctx, notes); err != nil {
			// Return an error so we can retry the request
			return fmt.Errorf("unable to send notes: %w", err)
		}
	} else {
		o.logger.Infow("the patient has no summary data", "order", params.Order.Meta, "clinicId", params.Match.Clinic.Id, "patientId", patient.Id)
	}

	return nil
}

func (o *newOrderProcessor) createSummaryStatisticsFlowsheet(params SummaryAndReportParameters) (models.NewFlowsheet, error) {
	patient, err := params.GetMatchingPatient()
	if err != nil {
		return models.NewFlowsheet{}, err
	}

	source := o.client.GetSource()
	destinationId := params.Match.Settings.DestinationIds.Flowsheet
	destinations := []struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	}{{
		ID: &destinationId,
	}}

	flowsheet := NewFlowsheet()
	flowsheet.Meta.Source = &source
	flowsheet.Meta.Destinations = &destinations
	flowsheet.Patient.Identifiers = params.Order.Patient.Identifiers
	flowsheet.Patient.Demographics = params.Order.Patient.Demographics

	SetVisitNumberInFlowsheet(params.Order, &flowsheet)
	SetAccountNumberInFlowsheet(params.Order, &flowsheet)
	PopulateSummaryStatistics(patient, params.Match.Clinic, &flowsheet)

	return flowsheet, nil
}

func (o *newOrderProcessor) createReportNote(ctx context.Context, params SummaryAndReportParameters) (Notes, error) {
	patient, err := params.GetMatchingPatient()
	if err != nil {
		return nil, err
	}

	reportingPeriod := report.GetReportingPeriodBounds(patient, days14)
	if reportingPeriod == nil {
		return nil, nil
	}

	var notes Notes
	if params.ShouldReplacePrecedingReport() {
		o.logger.Infow("creating replacement note",
			"order", params.Order.Meta,
			"clinicId", params.Match.Clinic.Id,
			"patientId", patient.Id,
			"precedingDocumentId", params.PrecedingDocumentId,
		)
		notes, err = CreateReplaceNotes(params.PrecedingDocumentId)
		if err != nil {
			return nil, err
		}
	} else {
		o.logger.Infow("creating new note",
			"order", params.Order.Meta,
			"clinicId", params.Match.Clinic.Id,
			"patientId", patient.Id,
		)
		notes = CreateNewNotes()
	}

	notes.SetSourceFromClient(o.client)
	notes.SetDestination(params.Match.Settings.DestinationIds.Notes)

	notes.SetOrderId(params.Order)
	notes.SetVisitNumberFromOrder(params.Order)
	notes.SetAccountNumberFromOrder(params.Order)

	documentId := params.DocumentId
	if documentId == "" {
		documentId = GenerateReportDocumentId(*params.Match.Clinic.Id, *patient.Id)
	}

	notes.SetReportMetadata(documentId)
	notes.SetPatientFromOrder(params.Order)
	notes.SetProcedureFromOrder(params.Order)
	notes.SetProviderFromOrder(params.Order)

	reportParameters := report.Parameters{
		UserDetail: report.UserDetail{
			UserId:      *patient.Id,
			FullName:    patient.FullName,
			DateOfBirth: patient.BirthDate.String(),
		},
		ReportDetail: report.ReportDetail{
			Reports: []string{"all"},
		},
	}
	if params.Match.Clinic.Id != nil {
		reportParameters.ClinicId = *params.Match.Clinic.Id
	}
	if params.Match.Clinic.Timezone != nil {
		reportParameters.ReportDetail.TimezoneName = string(*params.Match.Clinic.Timezone)
	}
	if patient.Mrn != nil {
		reportParameters.UserDetail.MRN = *patient.Mrn
	}
	if reportingPeriod != nil {
		if !reportingPeriod.Start.IsZero() {
			reportParameters.ReportDetail.StartDate = reportingPeriod.Start.Format(time.RFC3339)
		}
		if !reportingPeriod.End.IsZero() {
			reportParameters.ReportDetail.EndDate = reportingPeriod.End.Format(time.RFC3339)
		}
	}
	if params.Match.Clinic.PreferredBgUnits != "" {
		reportParameters.ReportDetail.BgUnits = string(params.Match.Clinic.PreferredBgUnits)
	}

	rprt, err := o.reportGenerator.GenerateReport(ctx, reportParameters)
	if err != nil {
		return nil, fmt.Errorf("unable to generate report: %w", err)
	}

	if o.client.IsUploadFileEnabled() {
		upload, err := o.client.UploadFile(ctx, NoteReportFileName, rprt.Document)
		if err != nil {
			return nil, fmt.Errorf("unable to upload report: %w", err)
		}
		if err := notes.SetUploadReference(NoteReportFileName, NoteReportFileType, *upload); err != nil {
			return nil, fmt.Errorf("unable to set upload reference in notes: %w", err)
		}
	} else {
		err = notes.SetEmbeddedFile(NoteReportFileName, NoteReportFileType, rprt.Document)
		if err != nil {
			return nil, fmt.Errorf("unable to embed report in notes: %w", err)
		}
	}

	return notes, nil
}

func (o *newOrderProcessor) handleUnknownProcedure(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Unknown procedure code. Ignoring order.", "order", order.Meta, "settings", match.Settings)
	return nil
}

func (o *newOrderProcessor) handleNoMatchingPatients(ctx context.Context, params SummaryAndReportParameters) error {
	o.logger.Infow("No patients matched.", "order", params.Order.Meta)
	return o.sendMatchingResultsNotification(ctx, ResultsNotification{
		IsSuccess: false,
		Message:   NoMatchingPatientsMessage,
	}, params)
}

func (o *newOrderProcessor) handleMultipleMatchingPatients(ctx context.Context, params SummaryAndReportParameters) error {
	o.logger.Infow("Multiple patients matched.", "order", params.Order.Meta)
	return o.sendMatchingResultsNotification(ctx, ResultsNotification{
		IsSuccess: false,
		Message:   MultipleMatchingPatientsMessage,
	}, params)
}

func (o *newOrderProcessor) handleSuccessfulPatientMatch(ctx context.Context, params SummaryAndReportParameters) error {
	o.logger.Infow("Found matching patient.", "order", params.Order.Meta)
	return o.sendMatchingResultsNotification(ctx, ResultsNotification{
		IsSuccess: true,
		Message:   SuccessfulMatchingMessage,
	}, params)
}

func (o *newOrderProcessor) handleAccountCreationSuccess(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("account was successfully created", "order", order.Meta)
	return o.sendAccountCreationResultsNotification(ctx, ResultsNotification{
		IsSuccess: true,
		Message:   SuccessfulAccountCreationMessage,
	}, order, match)
}

func (o *newOrderProcessor) handleAccountCreationError(ctx context.Context, err error, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Warnw("unable to create account", "order", order.Meta, "error", err)
	return o.sendAccountCreationResultsNotification(ctx, ResultsNotification{
		IsSuccess: false,
		Message:   err.Error(),
	}, order, match)
}

func (o *newOrderProcessor) sendMatchingResultsNotification(ctx context.Context, notification ResultsNotification, params SummaryAndReportParameters) error {
	o.logger.Infow("Sending matching results notification", "order", params.Order.Meta)
	source := o.client.GetSource()
	destinationId := params.Match.Settings.DestinationIds.Results
	destinations := []struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	}{{
		ID: &destinationId,
	}}

	results := NewResults()
	results.Meta.Source = &source
	results.Meta.Destinations = &destinations
	SetResultsPatientFromOrder(params.Order, &results)
	SetMatchingResult(notification, params.Order, &results)
	SetAccountNumberInResult(params.Order, &results)
	SetVisitNumberInResult(params.Order, &results)

	if err := o.client.Send(ctx, results); err != nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to send results: %w", err)
	}

	return nil
}

func (o *newOrderProcessor) sendAccountCreationResultsNotification(ctx context.Context, notification ResultsNotification, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Sending account creation results notification", "order", order.Meta)
	source := o.client.GetSource()
	destinationId := match.Settings.DestinationIds.Results
	destinations := []struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	}{{
		ID: &destinationId,
	}}

	results := NewResults()
	results.Meta.Source = &source
	results.Meta.Destinations = &destinations
	SetResultsPatientFromOrder(order, &results)
	SetAccountCreationResults(notification, order, &results)
	SetAccountNumberInResult(order, &results)
	SetVisitNumberInResult(order, &results)

	if err := o.client.Send(ctx, results); err != nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to send results: %w", err)
	}

	return nil
}

func GetBirthDateFromOrder(order models.NewOrder) (types.Date, error) {
	if order.Patient.Demographics == nil || order.Patient.Demographics.DOB == nil {
		return types.Date{}, fmt.Errorf("date of birth is missing")
	}

	birthDate := &types.Date{}
	err := birthDate.UnmarshalText([]byte(*order.Patient.Demographics.DOB))
	if err != nil {
		return *birthDate, err
	}
	return *birthDate, nil
}

func GetEmailAddressFromOrder(order models.NewOrder) (*string, error) {
	birthDate, err := GetBirthDateFromOrder(order)
	if err != nil {
		return nil, err
	}

	var email *string
	if shouldUseGuarantorEmail(birthDate) {
		email, err = GetGuarantorEmailAddressFromOrder(order)
		if err != nil {
			return nil, err
		}
	} else {
		email, err = GetPatientEmailAddressFromOrder(order)
		if err != nil {
			return nil, err
		}
	}

	if email == nil {
		return nil, nil
	}

	addr, err := mail.ParseAddress(*email)
	if err != nil {
		return nil, fmt.Errorf("email address is invalid")
	}

	return &addr.Address, nil
}

func shouldUseGuarantorEmail(birthDate types.Date) bool {
	now := time.Now()
	cutoff := birthDate.AddDate(MinimumAgeSelfOwnedAccountYears, 0, 0)
	return !cutoff.Before(now)
}

func GetPatientEmailAddressFromOrder(order models.NewOrder) (*string, error) {
	if order.Patient.Demographics == nil || order.Patient.Demographics.EmailAddresses == nil || len(*order.Patient.Demographics.EmailAddresses) == 0 {
		return nil, nil
	}

	email, ok := (*order.Patient.Demographics.EmailAddresses)[0].(string)
	if !ok {
		return nil, fmt.Errorf("patient email address is not a string")
	}

	return &email, nil
}

func GetGuarantorEmailAddressFromOrder(order models.NewOrder) (*string, error) {
	if order.Visit == nil || order.Visit.Guarantor == nil || order.Visit.Guarantor.EmailAddresses == nil || len(*order.Visit.Guarantor.EmailAddresses) == 0 {
		return nil, nil
	}

	email, ok := (*order.Visit.Guarantor.EmailAddresses)[0].(string)
	if !ok {
		return nil, fmt.Errorf("guarantor email address is not a string")
	}

	return &email, nil
}

func GetFullNameFromOrder(order models.NewOrder) (string, error) {
	if order.Patient.Demographics == nil {
		return "", fmt.Errorf("patient demographics is missing")
	}
	if order.Patient.Demographics.FirstName == nil || len(*order.Patient.Demographics.FirstName) == 0 {
		return "", fmt.Errorf("first name is missing")
	}
	if order.Patient.Demographics.LastName == nil || len(*order.Patient.Demographics.LastName) == 0 {
		return "", fmt.Errorf("last name is missing")
	}
	name := strings.Join([]string{*order.Patient.Demographics.FirstName, *order.Patient.Demographics.LastName}, " ")
	return name, nil
}

func GetMrnFromOrder(order models.NewOrder) (*string, error) {
	if len(order.Patient.Identifiers) == 0 {
		return nil, fmt.Errorf("mrn is missing")
	}
	var mrn *string
	for _, identifier := range order.Patient.Identifiers {
		if strings.ToLower(identifier.IDType) == "mrn" {
			mrn = &identifier.ID
			break
		}
	}

	if mrn == nil {
		return nil, fmt.Errorf("mrn is missing")
	}

	return mrn, nil
}

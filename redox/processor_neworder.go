package redox

import (
	"context"
	"errors"
	"fmt"
	"github.com/oapi-codegen/runtime/types"
	"github.com/tidepool-org/clinic-worker/report"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"go.uber.org/zap"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const (
	EventTypeNewOrder               = "New"
	DataModelOrder                  = "Order"
	MinimumAgeSelfOwnedAccountYears = 13
)

type NewOrderProcessor interface {
	ProcessOrder(ctx context.Context, envelope models.MessageEnvelope, order models.NewOrder) error
	SendSummaryAndReport(ctx context.Context, patient clinics.Patient, order models.NewOrder, match clinics.EHRMatchResponse) error
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

	if procedureCode == match.Settings.ProcedureCodes.EnableSummaryReports {
		return o.handleEnableSummaryReports(ctx, envelope, order, *match)
	} else if procedureCode == match.Settings.ProcedureCodes.DisableSummaryReports {
		return o.handleDisableSummaryReports(ctx, order, *match)
	} else if match.Settings.ProcedureCodes.CreateAccount != nil && *match.Settings.ProcedureCodes.CreateAccount == procedureCode {
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

func (o *newOrderProcessor) handleEnableSummaryReports(ctx context.Context, envelope models.MessageEnvelope, order models.NewOrder, match clinics.EHRMatchResponse) error {
	if match.Patients == nil || len(*match.Patients) == 0 {
		return o.handleNoMatchingPatients(ctx, order, match)
	} else if len(*match.Patients) > 1 {
		return o.handleMultipleMatchingPatients(ctx, order, match)
	}

	patient := (*match.Patients)[0]
	o.logger.Infow("successfully matched clinic and patient", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
	if err := o.handleSuccessfulPatientMatch(ctx, order, match); err != nil {
		return err
	}

	return o.SendSummaryAndReport(ctx, patient, order, match)
}

func (o *newOrderProcessor) handleDisableSummaryReports(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	if match.Patients == nil || len(*match.Patients) == 0 {
		return o.handleNoMatchingPatients(ctx, order, match)
	} else if len(*match.Patients) > 1 {
		return o.handleMultipleMatchingPatients(ctx, order, match)
	}

	patient := (*match.Patients)[0]
	o.logger.Infow("successfully matched clinic and patient", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
	return o.handleSuccessfulPatientMatch(ctx, order, match)
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

func (o *newOrderProcessor) SendSummaryAndReport(ctx context.Context, patient clinics.Patient, order models.NewOrder, match clinics.EHRMatchResponse) error {
	flowsheet := o.createSummaryStatisticsFlowsheet(order, patient, match)
	notes, err := o.createReportNote(ctx, order, patient, match)
	if err != nil {
		// return the error so we can retry the request
		return err
	}

	o.logger.Infow("sending flowsheet", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
	if err := o.client.Send(ctx, flowsheet); err != nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to send flowsheet: %w", err)
	}

	if notes != nil {
		o.logger.Infow("sending note", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
		if err := o.client.Send(ctx, *notes); err != nil {
			// Return an error so we can retry the request
			return fmt.Errorf("unable to send notes: %w", err)
		}
	} else {
		o.logger.Infow("the patient has no summary data", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
	}

	return nil
}

func (o *newOrderProcessor) createSummaryStatisticsFlowsheet(order models.NewOrder, patient clinics.Patient, match clinics.EHRMatchResponse) models.NewFlowsheet {
	source := o.client.GetSource()
	destinationId := match.Settings.DestinationIds.Flowsheet
	destinations := []struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	}{{
		ID: &destinationId,
	}}

	flowsheet := NewFlowsheet()
	flowsheet.Meta.Source = &source
	flowsheet.Meta.Destinations = &destinations
	flowsheet.Patient.Identifiers = order.Patient.Identifiers
	flowsheet.Patient.Demographics = order.Patient.Demographics

	SetVisitNumberInFlowsheet(order, &flowsheet)
	PopulateSummaryStatistics(patient, match.Clinic, &flowsheet)

	return flowsheet
}

func (o *newOrderProcessor) createReportNote(ctx context.Context, order models.NewOrder, patient clinics.Patient, match clinics.EHRMatchResponse) (*models.NewNotes, error) {
	reportingPeriod := report.GetReportingPeriodBounds(patient, days14)
	if reportingPeriod == nil {
		return nil, nil
	}

	source := o.client.GetSource()
	destinationId := match.Settings.DestinationIds.Notes

	destinations := []struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	}{{
		ID: &destinationId,
	}}

	notes := NewNotes()
	notes.Meta.Source = &source
	notes.Meta.Destinations = &destinations

	SetNotesPatientFromOrder(order, &notes)
	SetOrderIdInNotes(order, &notes)
	SetVisitNumberInNotes(order, &notes)
	SetReportMetadata(match.Clinic, patient, &notes)

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
	if match.Clinic.Id != nil {
		reportParameters.ClinicId = *match.Clinic.Id
	}
	if match.Clinic.Timezone != nil {
		reportParameters.ReportDetail.TimezoneName = string(*match.Clinic.Timezone)
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
	if match.Clinic.PreferredBgUnits != "" {
		reportParameters.ReportDetail.BgUnits = string(match.Clinic.PreferredBgUnits)
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
		if err := SetUploadReferenceInNote(NoteReportFileName, NoteReportFileType, *upload, &notes); err != nil {
			return nil, fmt.Errorf("unable to set upload reference in notes: %w", err)
		}
	} else {
		err = EmbedFileInNotes(NoteReportFileName, NoteReportFileType, rprt.Document, &notes)
		if err != nil {
			return nil, fmt.Errorf("unable to embed report in notes: %w", err)
		}
	}

	return &notes, nil
}

func (o *newOrderProcessor) handleUnknownProcedure(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Unknown procedure code. Ignoring order.", "order", order.Meta, "settings", match.Settings)
	return nil
}

func (o *newOrderProcessor) handleNoMatchingPatients(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("No patients matched.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, ResultsNotification{
		IsSuccess: false,
		Message:   NoMatchingPatientsMessage,
	}, order, match)
}

func (o *newOrderProcessor) handleMultipleMatchingPatients(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Multiple patients matched.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, ResultsNotification{
		IsSuccess: false,
		Message:   MultipleMatchingPatientsMessage,
	}, order, match)
}

func (o *newOrderProcessor) handleSuccessfulPatientMatch(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Found matching patient.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, ResultsNotification{
		IsSuccess: true,
		Message:   SuccessfulMatchingMessage,
	}, order, match)
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

func (o *newOrderProcessor) sendMatchingResultsNotification(ctx context.Context, notification ResultsNotification, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Sending matching results notification", "order", order.Meta)
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
	SetMatchingResult(notification, order, &results)

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

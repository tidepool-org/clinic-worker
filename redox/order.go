package redox

import (
	"context"
	"errors"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/redox/models"
	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	EventTypeNewOrder = "New"
	DataModelOrder    = "Order"

	ProcedureCodeSummaryReportsSubscription = "TP_SUBSCRIBE_SUMMARY_REPORTS"

	MrnIdentifierType = "MR"
)

type OrderProcessor interface {
	ProcessOrder(ctx context.Context, order models.NewOrder) error
}

type OrderHandler = func(ctx context.Context, order models.NewOrder) error

type orderProcessor struct {
	logger *zap.SugaredLogger

	clinics         clinics.ClientWithResponsesInterface
	client          Client
	reportGenerator ReportGenerator
}

func NewOrderProcessor(clinics clinics.ClientWithResponsesInterface, redox Client, reportGenerator ReportGenerator, logger *zap.SugaredLogger) OrderProcessor {
	return &orderProcessor{
		logger:          logger,
		clinics:         clinics,
		client:          redox,
		reportGenerator: reportGenerator,
	}
}

func (o *orderProcessor) ProcessOrder(ctx context.Context, order models.NewOrder) error {
	var procedureCode string
	if order.Order.Procedure != nil && order.Order.Procedure.Code != nil {
		procedureCode = *order.Order.Procedure.Code
	}

	handler := o.GetHandlerForProcedure(procedureCode)
	return handler(ctx, order)
}

func (o *orderProcessor) GetHandlerForProcedure(procedureCode string) OrderHandler {
	switch procedureCode {
	case ProcedureCodeSummaryReportsSubscription:
		return o.handleSummaryReportsSubscription
	default:
		return o.handleUnknownProcedure
	}
}

func (o *orderProcessor) handleSummaryReportsSubscription(ctx context.Context, order models.NewOrder) error {
	criteria, err := GetMatchingCriteria(order)
	if err != nil {
		// Return nil because the retrieval of the matching was unsuccessful and retrying will not help
		o.logger.Warnw("invalid matching criteria", "order", order.Meta, zap.Error(err))
		return nil
	}

	response, err := o.clinics.MatchClinicAndPatientWithResponse(ctx, *criteria)
	if err != nil {
		o.logger.Warnw("unable to match clinic and patient", "order", order.Meta, zap.Error(err))
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

	flowsheet := o.createSummaryStatisticsFlowsheet(order, patient, match)
	report, err := o.createReportNote(ctx, order, patient, match)
	if err != nil {
		// return the error so we can retry the request
		return err
	}

	o.logger.Infow("sending flowsheet", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
	if err := o.client.Send(ctx, flowsheet); err != nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to send flowsheet: %w", err)
	}

	if report != nil {
		o.logger.Infow("sending note", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
		if err := o.client.Send(ctx, report); err != nil {
			// Return an error so we can retry the request
			return fmt.Errorf("unable to send notes: %w", err)
		}
	} else {
		o.logger.Infow("the patient has no summary data", "order", order.Meta, "clinicId", match.Clinic.Id, "patientId", patient.Id)
	}

	return nil
}

func (o *orderProcessor) createSummaryStatisticsFlowsheet(order models.NewOrder, patient clinics.Patient, match *clinics.EHRMatchResponse) models.NewFlowsheet {
	source := o.client.GetSource()
	destinationId := match.Settings.DestinationIds.Default
	if match.Settings.DestinationIds.Flowsheet != nil && *match.Settings.DestinationIds.Flowsheet != "" {
		destinationId = *match.Settings.DestinationIds.Flowsheet
	}

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

	PopulateSummaryStatistics(patient, match.Clinic, &flowsheet)

	return flowsheet
}

func (o *orderProcessor) createReportNote(ctx context.Context, order models.NewOrder, patient clinics.Patient, match *clinics.EHRMatchResponse) (*models.NewNotes, error) {
	reportingPeriod := GetReportingPeriodBounds(patient)
	if reportingPeriod == nil {
		return nil, nil
	}

	source := o.client.GetSource()
	destinationId := match.Settings.DestinationIds.Default
	if match.Settings.DestinationIds.Flowsheet != nil && *match.Settings.DestinationIds.Flowsheet != "" {
		destinationId = *match.Settings.DestinationIds.Flowsheet
	}

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
	SetReportMetadata(match.Clinic, patient, &notes)

	reportParameters := ReportParameters{
		UserDetail: UserDetail{
			UserId:      *patient.Id,
			FullName:    patient.FullName,
			DateOfBirth: patient.BirthDate.String(),
		},
		ReportDetail: ReportDetail{
			Reports: []string{"all"},
		},
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

	report, err := o.reportGenerator.GenerateReport(ctx, reportParameters)
	if err != nil {
		return nil, fmt.Errorf("unable to generate report: %w", err)
	}

	err = EmbedFileInNotes(NoteReportFileName, NoteReportFileType, report.Document, &notes)
	if err != nil {
		return nil, fmt.Errorf("unable to embed report in notes: %w", err)
	}

	//upload, err := o.client.UploadFile(ctx, fileName, bytes.NewReader(sampleReport))
	//if err != nil {
	//	return nil, fmt.Errorf("unable to upload report: %w", err)
	//}
	//if err := SetUploadReferenceInNote(fileName, NoteReportFileType, *upload, &notes); err != nil {
	//	return nil, fmt.Errorf("unable to set upload reference in notes: %w", err)
	//}

	return &notes, nil
}

func (o *orderProcessor) handleUnknownProcedure(ctx context.Context, order models.NewOrder) error {
	o.logger.Infow("Unknown procedure code. Ignoring order.", "order", order.Meta)
	return nil
}

func (o *orderProcessor) handleNoMatchingPatients(ctx context.Context, order models.NewOrder, match *clinics.EHRMatchResponse) error {
	o.logger.Infow("No patients matched.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, MatchingResult{
		IsSuccess: false,
		Message:   NoMatchingPatientsMessage,
	}, order, match)
}

func (o *orderProcessor) handleMultipleMatchingPatients(ctx context.Context, order models.NewOrder, match *clinics.EHRMatchResponse) error {
	o.logger.Infow("Multiple patients matched.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, MatchingResult{
		IsSuccess: false,
		Message:   MultipleMatchingPatientsMessage,
	}, order, match)
}

func (o *orderProcessor) handleSuccessfulPatientMatch(ctx context.Context, order models.NewOrder, match *clinics.EHRMatchResponse) error {
	o.logger.Infow("Found matching patient.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, MatchingResult{
		IsSuccess: true,
		Message:   SuccessfulMatchingMessage,
	}, order, match)
}

func (o *orderProcessor) sendMatchingResultsNotification(ctx context.Context, matchingResult MatchingResult, order models.NewOrder, match *clinics.EHRMatchResponse) error {
	o.logger.Infow("Sending results notification", "order", order.Meta)
	source := o.client.GetSource()
	destinationId := match.Settings.DestinationIds.Default
	if match.Settings.DestinationIds.Flowsheet != nil && *match.Settings.DestinationIds.Flowsheet != "" {
		destinationId = *match.Settings.DestinationIds.Flowsheet
	}

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
	SetMatchingResult(matchingResult, order, &results)

	if err := o.client.Send(ctx, results); err != nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to send results: %w", err)
	}

	return nil
}

func GetMatchingCriteria(order models.NewOrder) (*clinics.MatchClinicAndPatientJSONRequestBody, error) {
	if order.Meta.Source == nil || order.Meta.Source.ID == nil {
		return nil, errors.New("missing source ID")
	}

	criteria := clinics.MatchClinicAndPatientJSONRequestBody{
		Clinic: clinics.EHRMatchClinicRequest{
			SourceId: *order.Meta.Source.ID,
		},
		Patient: &clinics.EHRMatchPatientRequest{},
	}
	if order.Order.OrderingFacility != nil && order.Order.OrderingFacility.Name != nil {
		criteria.Clinic.FacilityName = order.Order.OrderingFacility.Name
	}

	mrn := GetMRNFromOrder(order)
	if mrn == nil {
		return nil, errors.New("unable to find MRN in order")
	}
	if order.Patient.Demographics == nil {
		return nil, errors.New("missing patient demographics")
	}
	if order.Patient.Demographics.DOB == nil || *order.Patient.Demographics.DOB == "" {
		return nil, errors.New("missing patient date of birth")
	}
	if order.Patient.Demographics.FirstName == nil || *order.Patient.Demographics.FirstName == "" {
		return nil, errors.New("missing patient first name")
	}
	if order.Patient.Demographics.LastName == nil || *order.Patient.Demographics.LastName == "" {
		return nil, errors.New("missing patient last name")
	}

	criteria.Patient.Mrn = *mrn
	criteria.Patient.DateOfBirth = *order.Patient.Demographics.DOB
	criteria.Patient.FirstName = order.Patient.Demographics.FirstName
	criteria.Patient.LastName = order.Patient.Demographics.LastName
	criteria.Patient.MiddleName = order.Patient.Demographics.MiddleName

	return &criteria, nil
}

func GetMRNFromOrder(order models.NewOrder) *string {
	for _, identifier := range order.Patient.Identifiers {
		if identifier.IDType == MrnIdentifierType {
			return &identifier.ID
		}
	}

	return nil
}

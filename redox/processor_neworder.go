package redox

import (
	"context"
	"errors"
	"fmt"
	"github.com/tidepool-org/clinic-worker/report"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	EventTypeNewOrder = "New"
	DataModelOrder    = "Order"
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
}

func NewNewOrderProcessor(clinics clinics.ClientWithResponsesInterface, redox Client, reportGenerator report.Generator, logger *zap.SugaredLogger) NewOrderProcessor {
	return &newOrderProcessor{
		logger:          logger,
		clinics:         clinics,
		client:          redox,
		reportGenerator: reportGenerator,
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

	switch procedureCode {
	case match.Settings.ProcedureCodes.EnableSummaryReports:
		return o.handleEnableSummaryReports(ctx, envelope, order, *match)
	default:
		return o.handleUnknownProcedure(ctx, order, *match)
	}
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
		if err := o.client.Send(ctx, notes); err != nil {
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
	return o.sendMatchingResultsNotification(ctx, MatchingResult{
		IsSuccess: false,
		Message:   NoMatchingPatientsMessage,
	}, order, match)
}

func (o *newOrderProcessor) handleMultipleMatchingPatients(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Multiple patients matched.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, MatchingResult{
		IsSuccess: false,
		Message:   MultipleMatchingPatientsMessage,
	}, order, match)
}

func (o *newOrderProcessor) handleSuccessfulPatientMatch(ctx context.Context, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Found matching patient.", "order", order.Meta)
	return o.sendMatchingResultsNotification(ctx, MatchingResult{
		IsSuccess: true,
		Message:   SuccessfulMatchingMessage,
	}, order, match)
}

func (o *newOrderProcessor) sendMatchingResultsNotification(ctx context.Context, matchingResult MatchingResult, order models.NewOrder, match clinics.EHRMatchResponse) error {
	o.logger.Infow("Sending results notification", "order", order.Meta)
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
	SetMatchingResult(matchingResult, order, &results)

	if err := o.client.Send(ctx, results); err != nil {
		// Return an error so we can retry the request
		return fmt.Errorf("unable to send results: %w", err)
	}

	return nil
}

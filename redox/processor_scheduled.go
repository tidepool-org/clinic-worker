package redox

import (
	"context"
	"fmt"
	"net/http"
	"time"

	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

const recentDataCutoff = 14 * 24 * time.Hour

type ScheduledSummaryAndReport struct {
	Id                primitive.ObjectID     `json:"_id" bson:"_id"`
	UserId            string                 `json:"userId"`
	ClinicId          primitive.ObjectID     `json:"clinicId"`
	LastMatchedOrder  models.MessageEnvelope `json:"lastMatchedOrder"`
	PrecedingDocument *PrecedingDocument     `json:"precedingDocument"`
	CreatedTime       time.Time              `json:"createdTime"`
	DecodedOrder      models.NewOrder        `json:"-"`
}

type PrecedingDocument struct {
	Id          primitive.ObjectID `json:"_id" bson:"_id"`
	CreatedTime time.Time          `json:"createdTime"`
}

type ScheduledSummaryAndReportProcessor interface {
	ProcessOrder(ctx context.Context, scheduled ScheduledSummaryAndReport) error
}

type scheduledSummaryAndReportProcessor struct {
	clinics        clinics.ClientWithResponsesInterface
	orderProcessor NewOrderProcessor
	logger         *zap.SugaredLogger
}

func NewScheduledSummaryAndReportProcessor(orderProcessor NewOrderProcessor, clinics clinics.ClientWithResponsesInterface, logger *zap.SugaredLogger) ScheduledSummaryAndReportProcessor {
	return &scheduledSummaryAndReportProcessor{
		clinics:        clinics,
		orderProcessor: orderProcessor,
		logger:         logger,
	}
}

func (r *scheduledSummaryAndReportProcessor) ProcessOrder(ctx context.Context, scheduled ScheduledSummaryAndReport) error {
	clinicId := scheduled.ClinicId.Hex()
	patient, err := r.getPatient(ctx, clinicId, scheduled.UserId)
	if err != nil {
		return fmt.Errorf("unable to get patient: %w", err)
	}
	// The patient may have been deleted after the message was produced
	if patient == nil {
		r.logger.Infow("the patient doesn't exist, ignoring scheduled order", "clinicId", clinicId, "userId", scheduled.UserId)
		return nil
	}

	clinic, err := r.getClinic(ctx, clinicId)
	if err != nil {
		return fmt.Errorf("unable to get clinic: %w", err)
	}
	// The clinic may have been deleted after the message was produced
	if clinic == nil {
		r.logger.Infow("the clinic doesn't exist, ignoring scheduled order", "clinicId", clinicId)
		return nil
	}

	settings, err := r.getClinicSettings(ctx, clinicId)
	if err != nil {
		return fmt.Errorf("unable to get clinic settings: %w", err)
	}
	// The settings may have been deleted after the message was produced
	if settings == nil || !settings.Enabled {
		r.logger.Infow("EHR integration is not enabled, ignoring scheduled order", "clinicId", clinicId, "settings", settings)
		return nil
	}

	cutoffTime := getOrderTime(scheduled.DecodedOrder)
	if scheduled.PrecedingDocument != nil {
		// Check that there is new data uploaded after the previous scheduled message if one exists
		cutoffTime = scheduled.PrecedingDocument.CreatedTime
	}
	if !patientHasUploadedDataRecently(*patient, cutoffTime) {
		r.logger.Infow(
			"ignoring scheduled summary and report request because the user doesn't have recent data",
			"clinicId", clinicId,
			"userId", scheduled.UserId,
			"cutoffDate", cutoffTime.Format(time.RFC3339),
		)
		return nil
	}

	match := clinics.EhrMatchResponseV1{
		Clinic:   *clinic,
		Patients: &clinics.PatientsV1{*patient},
		Settings: *settings,
	}

	params := SummaryAndReportParameters{
		Match:             match,
		Order:             scheduled.DecodedOrder,
		DocumentId:        scheduled.Id.Hex(),
		PrecedingDocument: scheduled.PrecedingDocument,
	}

	return r.orderProcessor.SendSummaryAndReport(ctx, params)
}

func (r *scheduledSummaryAndReportProcessor) getPatient(ctx context.Context, clinicId, userId string) (*clinics.PatientV1, error) {
	resp, err := r.clinics.GetPatientWithResponse(ctx, clinicId, userId)
	if err != nil {
		return nil, fmt.Errorf("unable to get patient: %w", err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	} else if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from %s: %d", resp.HTTPResponse.Request.URL, resp.StatusCode())
	}

	return resp.JSON200, nil
}

func (r *scheduledSummaryAndReportProcessor) getClinic(ctx context.Context, clinicId string) (*clinics.ClinicV1, error) {
	resp, err := r.clinics.GetClinicWithResponse(ctx, clinicId)
	if err != nil {
		return nil, fmt.Errorf("unable to get clinic: %w", err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	} else if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from %s: %d", resp.HTTPResponse.Request.URL, resp.StatusCode())
	}

	return resp.JSON200, nil
}

func (r *scheduledSummaryAndReportProcessor) getClinicSettings(ctx context.Context, clinicId string) (*clinics.EhrSettingsV1, error) {
	resp, err := r.clinics.GetEHRSettingsWithResponse(ctx, clinicId)
	if err != nil {
		return nil, fmt.Errorf("unable to get clinic ehr settings: %w", err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	} else if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from %s: %d", resp.HTTPResponse.Request.URL, resp.StatusCode())
	}

	return resp.JSON200, nil
}

func patientHasUploadedDataRecently(patient clinics.PatientV1, cutoffTime time.Time) bool {
	mostRecentUploadDate := getMostRecentUploadDate(patient)
	return mostRecentUploadDate.After(cutoffTime)
}

func getMostRecentUploadDate(patient clinics.PatientV1) time.Time {
	var mostRecentUpload time.Time
	if patient.Summary != nil && patient.Summary.CgmStats != nil && patient.Summary.CgmStats.Dates.LastUploadDate != nil {
		mostRecentUpload = *patient.Summary.CgmStats.Dates.LastUploadDate
	}
	if patient.Summary != nil && patient.Summary.BgmStats != nil && patient.Summary.BgmStats.Dates.LastUploadDate != nil && patient.Summary.BgmStats.Dates.LastUploadDate.After(mostRecentUpload) {
		mostRecentUpload = *patient.Summary.BgmStats.Dates.LastUploadDate
	}
	return mostRecentUpload
}

func getOrderTime(order models.NewOrder) (result time.Time) {
	if order.Meta.EventDateTime == nil {
		return
	}

	// result will be zero if parsing fails, which is what we want to fallback to
	result, _ = time.Parse(time.RFC3339, *order.Meta.EventDateTime)
	return
}

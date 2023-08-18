package redox

import (
	"context"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"net/http"
)

type ScheduledSummaryAndReport struct {
	UserId           string                 `json:"userId"`
	ClinicId         primitive.ObjectID     `json:"clinicId"`
	LastMatchedOrder models.MessageEnvelope `json:"lastMatchedOrder"`
	DecodedOrder     models.NewOrder        `json:"-"`
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

	match := clinics.EHRMatchResponse{
		Clinic:   *clinic,
		Patients: &clinics.Patients{*patient},
		Settings: *settings,
	}

	return r.orderProcessor.SendSummaryAndReport(ctx, *patient, scheduled.DecodedOrder, match)
}

func (r *scheduledSummaryAndReportProcessor) getPatient(ctx context.Context, clinicId, userId string) (*clinics.Patient, error) {
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

func (r *scheduledSummaryAndReportProcessor) getClinic(ctx context.Context, clinicId string) (*clinics.Clinic, error) {
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

func (r *scheduledSummaryAndReportProcessor) getClinicSettings(ctx context.Context, clinicId string) (*clinics.EHRSettings, error) {
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

package redox

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/kelseyhightower/envconfig"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"io"
	"time"
)

//go:embed test/sample-report.pdf
var sampleReport []byte

const (
	tidepoolTokenHeader = "x-tidepool-session-token"
)

type ReportGeneratorConfig struct {
	ExportServiceHost string `envconfig:"TIDEPOOL_EXPORT_CLIENT_ADDRESS" default:"http://export:9300"`
}

type ReportGenerator interface {
	GenerateReport(ctx context.Context, params ReportParameters) (*Report, error)
}

type reportGenerator struct {
	restyClient     *resty.Client
	shorelineClient shoreline.Client
}

func (r *reportGenerator) GenerateReport(ctx context.Context, params ReportParameters) (*Report, error) {
	token := r.shorelineClient.TokenProvide()
	if token == "" {
		return nil, fmt.Errorf("unable to get token from shoreline client")
	}

	resp, err := r.restyClient.R().
		SetContext(ctx).
		SetHeader(tidepoolTokenHeader, token).
		SetPathParams(map[string]string{
			"userId": params.UserDetail.UserId,
		}).
		SetBody(params).
		Post("/export/report/{userId}")
	if err != nil {
		return nil, fmt.Errorf("error generating report: %w", err)
	}
	if resp.IsSuccess() {
		return &Report{Document: resp.RawBody()}, nil
	}
	return nil, fmt.Errorf("received unexected %s response when generating report: %s", resp.Status(), resp.Body())
}

func NewReportGenerator(shorelineClient shoreline.Client) (ReportGenerator, error) {
	config := ReportGeneratorConfig{}
	if err := envconfig.Process("", &config); err != nil {
		return nil, err
	}

	return &reportGenerator{
		restyClient:     resty.New().SetBaseURL(config.ExportServiceHost),
		shorelineClient: shorelineClient,
	}, nil
}

type SampleReportGenerator struct{}

func NewSampleReportGenerator() ReportGenerator {
	return &SampleReportGenerator{}
}

func (s SampleReportGenerator) GenerateReport(ctx context.Context, params ReportParameters) (*Report, error) {
	return &Report{Document: bytes.NewReader(sampleReport)}, nil
}

type ReportParameters struct {
	UserDetail   UserDetail   `json:"userDetail"`
	ReportDetail ReportDetail `json:"reportDetail"`
}

type UserDetail struct {
	UserId      string `json:"userId"`
	FullName    string `json:"fullName"`
	DateOfBirth string `json:"dob"`
	MRN         string `json:"mrn,omitempty"`
}

type ReportDetail struct {
	TimezoneName string   `json:"tzName,omitempty"`
	BgUnits      string   `json:"bgUnits,omitempty"`
	Reports      []string `json:"reports,omitempty"`
	StartDate    string   `json:"startDate,omitempty"`
	EndDate      string   `json:"endDate,omitempty"`
}

type Report struct {
	Document io.Reader
}

func GetPeriodBounds(dates *clinics.PatientSummaryDates, duration time.Duration) *PeriodBounds {
	if dates == nil {
		return nil
	}
	if dates.LastData == nil {
		return nil
	}
	return &PeriodBounds{
		Start: dates.LastData.Add(-duration),
		End:   *dates.LastData,
	}
}

// GetReportingPeriodBounds returns the reporting period bounds for a patient. If the patient has both CGM and BGM data,
// the reporting period will be based on the most recent data from either source.
// If the patient has only CGM data, the reporting period will be based on the CGM data.
// Otherwise, if the patient has only BGM data, the reporting period will be based on
// the BGM data. If the patient has neither CGM nor BGM data, nil will be returned.
func GetReportingPeriodBounds(patient clinics.Patient) *PeriodBounds {
	cgmDates := GetCGMStatsDates(patient)
	bgmDates := GetBGMStatsDates(patient)
	bounds := GetPeriodBounds(cgmDates, days14)

	if bounds == nil || (bgmDates != nil && bgmDates.LastData != nil && bgmDates.LastData.After(bounds.End)) {
		bounds = GetPeriodBounds(bgmDates, days14)
	}
	return bounds
}

func GetCGMStatsDates(patient clinics.Patient) *clinics.PatientSummaryDates {
	if patient.Summary == nil || patient.Summary.CgmStats == nil {
		return nil
	}
	return patient.Summary.CgmStats.Dates
}

func GetBGMStatsDates(patient clinics.Patient) *clinics.PatientSummaryDates {
	if patient.Summary == nil || patient.Summary.BgmStats == nil {
		return nil
	}
	return patient.Summary.BgmStats.Dates
}

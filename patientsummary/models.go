package patientsummary

import (
	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"time"
)

type CDCEvent struct {
	Offset        int64   `json:"-"`
	FullDocument  Summary `json:"fullDocument"`
	OperationType string  `json:"operationType"`
}

func (p CDCEvent) ShouldApplyUpdates() bool {
	if p.OperationType != cdc.OperationTypeInsert &&
		p.OperationType != cdc.OperationTypeUpdate &&
		p.OperationType != cdc.OperationTypeReplace {
		return false
	}

	if p.FullDocument.UserId == nil {
		return false
	}

	return true
}

type Summary struct {
	Id                   *cdc.ObjectId           `json:"_id"`
	UserId               *string                 `json:"userId"`
	LastUpdated          *cdc.Date               `json:"lastUpdated,omitempty"`
	FirstData            *cdc.Date               `json:"firstData,omitempty"`
	LastData             *cdc.Date               `json:"lastData,omitempty"`
	LastUpload           *cdc.Date               `json:"lastUpload,omitempty"`
	OutdatedSince        *cdc.Date               `json:"outdatedSince"`
	AverageGlucose       *clinics.AverageGlucose `json:"avgGlucose,omitempty"`
	GlucoseMgmtIndicator *float64                `json:"glucoseMgmtIndicator,omitempty"`
	TimeInRange          *float64                `json:"timeInRange,omitempty"`
	TimeBelowRange       *float64                `json:"timeBelowRange,omitempty"`
	TimeVeryBelowRange   *float64                `json:"timeVeryBelowRange,omitempty"`
	TimeAboveRange       *float64                `json:"timeAboveRange,omitempty"`
	TimeVeryAboveRange   *float64                `json:"timeVeryAboveRange,omitempty"`
	TimeCGMUse           *float64                `json:"timeCGMUse,omitempty"`
	HighGlucoseThreshold *float64                `json:"highGlucoseThreshold,omitempty"`
	LowGlucoseThreshold  *float64                `json:"lowGlucoseThreshold,omitempty"`
}

func (p CDCEvent) CreateUpdateBody() clinics.UpdatePatientSummaryJSONRequestBody {
	var firstData *time.Time
	var lastData *time.Time
	var lastUpdated *time.Time
	var lastUpload *time.Time
	var outdatedSince *time.Time

	if p.FullDocument.FirstData != nil {
		firstDataVal := time.UnixMilli(p.FullDocument.FirstData.Value)
		firstData = &firstDataVal
	}
	if p.FullDocument.LastData != nil {
		lastDataVal := time.UnixMilli(p.FullDocument.LastData.Value)
		lastData = &lastDataVal
	}
	if p.FullDocument.LastUpdated != nil {
		lastUpdatedVal := time.UnixMilli(p.FullDocument.LastUpdated.Value)
		lastUpdated = &lastUpdatedVal
	}
	if p.FullDocument.LastUpload != nil {
		lastUploadVal := time.UnixMilli(p.FullDocument.LastUpload.Value)
		lastUpload = &lastUploadVal
	}
	if p.FullDocument.OutdatedSince != nil {
		outdatedSinceVal := time.UnixMilli(p.FullDocument.LastData.Value)
		outdatedSince = &outdatedSinceVal
	}

	return clinics.UpdatePatientSummaryJSONRequestBody{
		AverageGlucose:             p.FullDocument.AverageGlucose,
		FirstData:                  firstData,
		GlucoseManagementIndicator: p.FullDocument.GlucoseMgmtIndicator,
		HighGlucoseThreshold:       p.FullDocument.HighGlucoseThreshold,
		LastData:                   lastData,
		LastUpdatedDate:            lastUpdated,
		LastUploadDate:             lastUpload,
		LowGlucoseThreshold:        p.FullDocument.LowGlucoseThreshold,
		OutdatedSince:              outdatedSince,
		PercentTimeCGMUse:          p.FullDocument.TimeCGMUse,
		PercentTimeInHigh:          p.FullDocument.TimeAboveRange,
		PercentTimeInLow:           p.FullDocument.TimeBelowRange,
		PercentTimeInTarget:        p.FullDocument.TimeInRange,
		PercentTimeInVeryHigh:      p.FullDocument.TimeVeryAboveRange,
		PercentTimeInVeryLow:       p.FullDocument.TimeVeryBelowRange,
	}
}

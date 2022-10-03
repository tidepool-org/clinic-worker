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

	if p.FullDocument.UserID == nil {
		return false
	}

	return true
}

type Stats struct {
	DeviceID string    `json:"deviceId"`
	Date     *cdc.Date `json:"date"`

	TargetMinutes int `json:"targetMinutes"`
	TargetRecords int `json:"targetRecords"`

	LowMinutes int `json:"LowMinutes"`
	LowRecords int `json:"LowRecords"`

	VeryLowMinutes int `json:"veryLowMinutes"`
	VeryLowRecords int `json:"veryLowRecords"`

	HighMinutes int `json:"highMinutes"`
	HighRecords int `json:"highRecords"`

	VeryHighMinutes int `json:"veryHighMinutes"`
	VeryHighRecords int `json:"veryHighRecords"`

	TotalGlucose    float64   `json:"totalGlucose"`
	TotalCGMMinutes int       `json:"totalCGMMinutes"`
	TotalCGMRecords int       `json:"totalCGMRecords"`
	LastRecordTime  *cdc.Date `json:"lastRecordTime"`
}

type Period struct {
	TimeCGMUsePercent    *float64 `json:"timeCGMUsePercent"`
	HasTimeCGMUsePercent *bool    `json:"hasTimeCGMUsePercent"`
	TimeCGMUseMinutes    *int     `json:"timeCGMUseMinutes"`
	TimeCGMUseRecords    *int     `json:"timeCGMUseRecords"`

	// actual values
	AverageGlucose    *clinics.AverageGlucose `json:"avgGlucose"`
	HasAverageGlucose *bool                   `json:"hasAverageGlucose"`

	GlucoseManagementIndicator    *float64 `json:"glucoseManagementIndicator"`
	HasGlucoseManagementIndicator *bool    `json:"hasGlucoseManagementIndicator"`

	TimeInTargetPercent    *float64 `json:"timeInTargetPercent"`
	HasTimeInTargetPercent *bool    `json:"hasTimeInTargetPercent"`
	TimeInTargetMinutes    *int     `json:"timeInTargetMinutes"`
	TimeInTargetRecords    *int     `json:"timeInTargetRecords"`

	TimeInLowPercent    *float64 `json:"timeInLowPercent"`
	HasTimeInLowPercent *bool    `json:"hasTimeInLowPercent"`
	TimeInLowMinutes    *int     `json:"timeInLowMinutes"`
	TimeInLowRecords    *int     `json:"timeInLowRecords"`

	TimeInVeryLowPercent    *float64 `json:"timeInVeryLowPercent"`
	HasTimeInVeryLowPercent *bool    `json:"hasTimeInVeryLowPercent"`
	TimeInVeryLowMinutes    *int     `json:"timeInVeryLowMinutes"`
	TimeInVeryLowRecords    *int     `json:"timeInVeryLowRecords"`

	TimeInHighPercent    *float64 `json:"timeInHighPercent"`
	HasTimeInHighPercent *bool    `json:"hasTimeInHighPercent"`
	TimeInHighMinutes    *int     `json:"timeInHighMinutes"`
	TimeInHighRecords    *int     `json:"timeInHighRecords"`

	TimeInVeryHighPercent    *float64 `json:"timeInVeryHighPercent"`
	HasTimeInVeryHighPercent *bool    `json:"hasTimeInVeryHighPercent"`
	TimeInVeryHighMinutes    *int     `json:"timeInVeryHighMinutes"`
	TimeInVeryHighRecords    *int     `json:"timeInVeryHighRecords"`
}

type Summary struct {
	ID     *cdc.ObjectId `json:"_id"`
	UserID *string       `json:"userId"`

	HourlyStats []*Stats           `json:"hourlyStats"`
	Periods     map[string]*Period `json:"periods"`

	// date tracking
	LastUpdatedDate   *cdc.Date `json:"lastUpdatedDate"`
	FirstData         *cdc.Date `json:"firstData"`
	LastData          *cdc.Date `json:"lastData"`
	LastUploadDate    *cdc.Date `json:"lastUploadDate"`
	HasLastUploadDate *bool     `json:"hasLastUploadDate"`
	OutdatedSince     *cdc.Date `json:"outdatedSince"`

	TotalHours *int `json:"totalHours"`

	// these are just constants right now.
	HighGlucoseThreshold     *float64 `json:"highGlucoseThreshold"`
	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold"`
	LowGlucoseThreshold      *float64 `json:"lowGlucoseThreshold"`
	VeryLowGlucoseThreshold  *float64 `json:"VeryLowGlucoseThreshold"`
}

func (p CDCEvent) CreateUpdateBody() clinics.UpdatePatientSummaryJSONRequestBody {
	var firstData *time.Time
	var lastData *time.Time
	var lastUpdatedDate *time.Time
	var lastUploadDate *time.Time
	var outdatedSince *time.Time

	if p.FullDocument.FirstData != nil {
		firstDataVal := time.UnixMilli(p.FullDocument.FirstData.Value)
		firstData = &firstDataVal
	}
	if p.FullDocument.LastData != nil {
		lastDataVal := time.UnixMilli(p.FullDocument.LastData.Value)
		lastData = &lastDataVal
	}
	if p.FullDocument.LastUpdatedDate != nil {
		lastUpdatedDateVal := time.UnixMilli(p.FullDocument.LastUpdatedDate.Value)
		lastUpdatedDate = &lastUpdatedDateVal
	}
	if p.FullDocument.LastUploadDate != nil {
		lastUploadDateVal := time.UnixMilli(p.FullDocument.LastUploadDate.Value)
		lastUploadDate = &lastUploadDateVal
	}
	if p.FullDocument.OutdatedSince != nil {
		outdatedSinceVal := time.UnixMilli(p.FullDocument.OutdatedSince.Value)
		outdatedSince = &outdatedSinceVal
	}

	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{
		FirstData:                firstData,
		LastData:                 lastData,
		LastUpdatedDate:          lastUpdatedDate,
		LastUploadDate:           lastUploadDate,
		HasLastUploadDate:        p.FullDocument.HasLastUploadDate,
		OutdatedSince:            outdatedSince,
		TotalHours:               p.FullDocument.TotalHours,
		LowGlucoseThreshold:      p.FullDocument.LowGlucoseThreshold,
		VeryLowGlucoseThreshold:  p.FullDocument.VeryLowGlucoseThreshold,
		HighGlucoseThreshold:     p.FullDocument.HighGlucoseThreshold,
		VeryHighGlucoseThreshold: p.FullDocument.VeryHighGlucoseThreshold,
	}

	if p.FullDocument.Periods != nil {
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*Period{}
		destPeriods := map[string]*clinics.PatientSummaryPeriod{}
		if _, exists := p.FullDocument.Periods["1d"]; exists {
			sourcePeriods["1d"] = p.FullDocument.Periods["1d"]

			patientUpdate.Periods.N1d = &clinics.PatientSummaryPeriod{}
			destPeriods["1d"] = patientUpdate.Periods.N1d
		}
		if _, exists := p.FullDocument.Periods["7d"]; exists {
			sourcePeriods["7d"] = p.FullDocument.Periods["7d"]

			patientUpdate.Periods.N7d = &clinics.PatientSummaryPeriod{}
			destPeriods["7d"] = patientUpdate.Periods.N7d
		}
		if _, exists := p.FullDocument.Periods["14d"]; exists {
			sourcePeriods["14d"] = p.FullDocument.Periods["14d"]

			patientUpdate.Periods.N14d = &clinics.PatientSummaryPeriod{}
			destPeriods["14d"] = patientUpdate.Periods.N14d
		}
		if _, exists := p.FullDocument.Periods["30d"]; exists {
			sourcePeriods["30d"] = p.FullDocument.Periods["30d"]

			patientUpdate.Periods.N30d = &clinics.PatientSummaryPeriod{}
			destPeriods["30d"] = patientUpdate.Periods.N30d
		}

		for period := range sourcePeriods {
			destPeriods[period].AverageGlucose = sourcePeriods[period].AverageGlucose
			destPeriods[period].HasAverageGlucose = sourcePeriods[period].HasAverageGlucose

			destPeriods[period].GlucoseManagementIndicator = sourcePeriods[period].GlucoseManagementIndicator
			destPeriods[period].HasGlucoseManagementIndicator = sourcePeriods[period].HasGlucoseManagementIndicator

			destPeriods[period].TimeCGMUseMinutes = sourcePeriods[period].TimeCGMUseMinutes
			destPeriods[period].TimeCGMUsePercent = sourcePeriods[period].TimeCGMUsePercent
			destPeriods[period].HasTimeCGMUsePercent = sourcePeriods[period].HasTimeCGMUsePercent
			destPeriods[period].TimeCGMUseRecords = sourcePeriods[period].TimeCGMUseRecords

			destPeriods[period].TimeInHighMinutes = sourcePeriods[period].TimeInHighMinutes
			destPeriods[period].TimeInHighPercent = sourcePeriods[period].TimeInHighPercent
			destPeriods[period].HasTimeInHighPercent = sourcePeriods[period].HasTimeInHighPercent
			destPeriods[period].TimeInHighRecords = sourcePeriods[period].TimeInHighRecords

			destPeriods[period].TimeInLowMinutes = sourcePeriods[period].TimeInLowMinutes
			destPeriods[period].TimeInLowPercent = sourcePeriods[period].TimeInLowPercent
			destPeriods[period].HasTimeInLowPercent = sourcePeriods[period].HasTimeInLowPercent
			destPeriods[period].TimeInLowRecords = sourcePeriods[period].TimeInLowRecords

			destPeriods[period].TimeInTargetMinutes = sourcePeriods[period].TimeInTargetMinutes
			destPeriods[period].TimeInTargetPercent = sourcePeriods[period].TimeInTargetPercent
			destPeriods[period].HasTimeInTargetPercent = sourcePeriods[period].HasTimeInTargetPercent
			destPeriods[period].TimeInTargetRecords = sourcePeriods[period].TimeInTargetRecords

			destPeriods[period].TimeInVeryHighMinutes = sourcePeriods[period].TimeInVeryHighMinutes
			destPeriods[period].TimeInVeryHighPercent = sourcePeriods[period].TimeInVeryHighPercent
			destPeriods[period].HasTimeInVeryHighPercent = sourcePeriods[period].HasTimeInVeryHighPercent
			destPeriods[period].TimeInVeryHighRecords = sourcePeriods[period].TimeInVeryHighRecords

			destPeriods[period].TimeInVeryLowMinutes = sourcePeriods[period].TimeInVeryLowMinutes
			destPeriods[period].TimeInVeryLowPercent = sourcePeriods[period].TimeInVeryLowPercent
			destPeriods[period].HasTimeInVeryLowPercent = sourcePeriods[period].HasTimeInVeryLowPercent
			destPeriods[period].TimeInVeryLowRecords = sourcePeriods[period].TimeInVeryLowRecords
		}
	}
	return patientUpdate
}

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
	TimeCGMUsePercent *float64 `json:"timeCGMUsePercent"`
	TimeCGMUseMinutes *int     `json:"timeCGMUseMinutes"`
	TimeCGMUseRecords *int     `json:"timeCGMUseRecords"`

	// actual values
	AverageGlucose             *clinics.AverageGlucose `json:"avgGlucose"`
	GlucoseManagementIndicator *float64                `json:"glucoseManagementIndicator"`

	TimeInTargetPercent *float64 `json:"timeInTargetPercent"`
	TimeInTargetMinutes *int     `json:"timeInTargetMinutes"`
	TimeInTargetRecords *int     `json:"timeInTargetRecords"`

	TimeInLowPercent *float64 `json:"timeInLowPercent"`
	TimeInLowMinutes *int     `json:"timeInLowMinutes"`
	TimeInLowRecords *int     `json:"timeInLowRecords"`

	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent"`
	TimeInVeryLowMinutes *int     `json:"timeInVeryLowMinutes"`
	TimeInVeryLowRecords *int     `json:"timeInVeryLowRecords"`

	TimeInHighPercent *float64 `json:"timeInHighPercent"`
	TimeInHighMinutes *int     `json:"timeInHighMinutes"`
	TimeInHighRecords *int     `json:"timeInHighRecords"`

	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent"`
	TimeInVeryHighMinutes *int     `json:"timeInVeryHighMinutes"`
	TimeInVeryHighRecords *int     `json:"timeInVeryHighRecords"`
}

type Summary struct {
	ID     *cdc.ObjectId `json:"_id"`
	UserID *string       `json:"userId"`

	HourlyStats []*Stats           `json:"hourlyStats"`
	Periods     map[string]*Period `json:"periods"`

	// date tracking
	LastUpdatedDate *cdc.Date `json:"lastUpdatedDate"`
	FirstData       *cdc.Date `json:"firstData"`
	LastData        *cdc.Date `json:"lastData"`
	LastUploadDate  *cdc.Date `json:"lastUploadDate"`
	OutdatedSince   *cdc.Date `json:"outdatedSince"`

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
		OutdatedSince:            outdatedSince,
		TotalHours:               p.FullDocument.TotalHours,
		LowGlucoseThreshold:      p.FullDocument.LowGlucoseThreshold,
		VeryLowGlucoseThreshold:  p.FullDocument.VeryLowGlucoseThreshold,
		HighGlucoseThreshold:     p.FullDocument.HighGlucoseThreshold,
		VeryHighGlucoseThreshold: p.FullDocument.VeryHighGlucoseThreshold,
	}

	var periodExists = false
	var period1dExists = false
	var period7dExists = false
	var period14dExists = false
	var period30dExists = false
	if p.FullDocument.Periods != nil {
		periodExists = true
		_, period1dExists = p.FullDocument.Periods["1d"]
		_, period7dExists = p.FullDocument.Periods["7d"]
		_, period14dExists = p.FullDocument.Periods["14d"]
		_, period30dExists = p.FullDocument.Periods["30d"]
	}

	if periodExists && period1dExists {
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{N1d: &clinics.PatientSummaryPeriod{
			AverageGlucose:             p.FullDocument.Periods["1d"].AverageGlucose,
			GlucoseManagementIndicator: p.FullDocument.Periods["1d"].GlucoseManagementIndicator,

			TimeCGMUseMinutes: p.FullDocument.Periods["1d"].TimeCGMUseMinutes,
			TimeCGMUsePercent: p.FullDocument.Periods["1d"].TimeCGMUsePercent,
			TimeCGMUseRecords: p.FullDocument.Periods["1d"].TimeCGMUseRecords,

			TimeInHighMinutes: p.FullDocument.Periods["1d"].TimeInHighMinutes,
			TimeInHighPercent: p.FullDocument.Periods["1d"].TimeInHighPercent,
			TimeInHighRecords: p.FullDocument.Periods["1d"].TimeInHighRecords,

			TimeInLowMinutes: p.FullDocument.Periods["1d"].TimeInLowMinutes,
			TimeInLowPercent: p.FullDocument.Periods["1d"].TimeInLowPercent,
			TimeInLowRecords: p.FullDocument.Periods["1d"].TimeInLowRecords,

			TimeInTargetMinutes: p.FullDocument.Periods["1d"].TimeInTargetMinutes,
			TimeInTargetPercent: p.FullDocument.Periods["1d"].TimeInTargetPercent,
			TimeInTargetRecords: p.FullDocument.Periods["1d"].TimeInTargetRecords,

			TimeInVeryHighMinutes: p.FullDocument.Periods["1d"].TimeInVeryHighMinutes,
			TimeInVeryHighPercent: p.FullDocument.Periods["1d"].TimeInVeryHighPercent,
			TimeInVeryHighRecords: p.FullDocument.Periods["1d"].TimeInVeryHighRecords,

			TimeInVeryLowMinutes: p.FullDocument.Periods["1d"].TimeInVeryLowMinutes,
			TimeInVeryLowPercent: p.FullDocument.Periods["1d"].TimeInVeryLowPercent,
			TimeInVeryLowRecords: p.FullDocument.Periods["1d"].TimeInVeryLowRecords,
		}}
	}

	if periodExists && period7dExists {
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{N7d: &clinics.PatientSummaryPeriod{
			AverageGlucose:             p.FullDocument.Periods["7d"].AverageGlucose,
			GlucoseManagementIndicator: p.FullDocument.Periods["7d"].GlucoseManagementIndicator,

			TimeCGMUseMinutes: p.FullDocument.Periods["7d"].TimeCGMUseMinutes,
			TimeCGMUsePercent: p.FullDocument.Periods["7d"].TimeCGMUsePercent,
			TimeCGMUseRecords: p.FullDocument.Periods["7d"].TimeCGMUseRecords,

			TimeInHighMinutes: p.FullDocument.Periods["7d"].TimeInHighMinutes,
			TimeInHighPercent: p.FullDocument.Periods["7d"].TimeInHighPercent,
			TimeInHighRecords: p.FullDocument.Periods["7d"].TimeInHighRecords,

			TimeInLowMinutes: p.FullDocument.Periods["7d"].TimeInLowMinutes,
			TimeInLowPercent: p.FullDocument.Periods["7d"].TimeInLowPercent,
			TimeInLowRecords: p.FullDocument.Periods["7d"].TimeInLowRecords,

			TimeInTargetMinutes: p.FullDocument.Periods["7d"].TimeInTargetMinutes,
			TimeInTargetPercent: p.FullDocument.Periods["7d"].TimeInTargetPercent,
			TimeInTargetRecords: p.FullDocument.Periods["7d"].TimeInTargetRecords,

			TimeInVeryHighMinutes: p.FullDocument.Periods["7d"].TimeInVeryHighMinutes,
			TimeInVeryHighPercent: p.FullDocument.Periods["7d"].TimeInVeryHighPercent,
			TimeInVeryHighRecords: p.FullDocument.Periods["7d"].TimeInVeryHighRecords,

			TimeInVeryLowMinutes: p.FullDocument.Periods["7d"].TimeInVeryLowMinutes,
			TimeInVeryLowPercent: p.FullDocument.Periods["7d"].TimeInVeryLowPercent,
			TimeInVeryLowRecords: p.FullDocument.Periods["7d"].TimeInVeryLowRecords,
		}}
	}

	if periodExists && period14dExists {
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{N14d: &clinics.PatientSummaryPeriod{
			AverageGlucose:             p.FullDocument.Periods["14d"].AverageGlucose,
			GlucoseManagementIndicator: p.FullDocument.Periods["14d"].GlucoseManagementIndicator,

			TimeCGMUseMinutes: p.FullDocument.Periods["14d"].TimeCGMUseMinutes,
			TimeCGMUsePercent: p.FullDocument.Periods["14d"].TimeCGMUsePercent,
			TimeCGMUseRecords: p.FullDocument.Periods["14d"].TimeCGMUseRecords,

			TimeInHighMinutes: p.FullDocument.Periods["14d"].TimeInHighMinutes,
			TimeInHighPercent: p.FullDocument.Periods["14d"].TimeInHighPercent,
			TimeInHighRecords: p.FullDocument.Periods["14d"].TimeInHighRecords,

			TimeInLowMinutes: p.FullDocument.Periods["14d"].TimeInLowMinutes,
			TimeInLowPercent: p.FullDocument.Periods["14d"].TimeInLowPercent,
			TimeInLowRecords: p.FullDocument.Periods["14d"].TimeInLowRecords,

			TimeInTargetMinutes: p.FullDocument.Periods["14d"].TimeInTargetMinutes,
			TimeInTargetPercent: p.FullDocument.Periods["14d"].TimeInTargetPercent,
			TimeInTargetRecords: p.FullDocument.Periods["14d"].TimeInTargetRecords,

			TimeInVeryHighMinutes: p.FullDocument.Periods["14d"].TimeInVeryHighMinutes,
			TimeInVeryHighPercent: p.FullDocument.Periods["14d"].TimeInVeryHighPercent,
			TimeInVeryHighRecords: p.FullDocument.Periods["14d"].TimeInVeryHighRecords,

			TimeInVeryLowMinutes: p.FullDocument.Periods["14d"].TimeInVeryLowMinutes,
			TimeInVeryLowPercent: p.FullDocument.Periods["14d"].TimeInVeryLowPercent,
			TimeInVeryLowRecords: p.FullDocument.Periods["14d"].TimeInVeryLowRecords,
		}}
	}

	if periodExists && period30dExists {
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{N30d: &clinics.PatientSummaryPeriod{
			AverageGlucose:             p.FullDocument.Periods["30d"].AverageGlucose,
			GlucoseManagementIndicator: p.FullDocument.Periods["30d"].GlucoseManagementIndicator,

			TimeCGMUseMinutes: p.FullDocument.Periods["30d"].TimeCGMUseMinutes,
			TimeCGMUsePercent: p.FullDocument.Periods["30d"].TimeCGMUsePercent,
			TimeCGMUseRecords: p.FullDocument.Periods["30d"].TimeCGMUseRecords,

			TimeInHighMinutes: p.FullDocument.Periods["30d"].TimeInHighMinutes,
			TimeInHighPercent: p.FullDocument.Periods["30d"].TimeInHighPercent,
			TimeInHighRecords: p.FullDocument.Periods["30d"].TimeInHighRecords,

			TimeInLowMinutes: p.FullDocument.Periods["30d"].TimeInLowMinutes,
			TimeInLowPercent: p.FullDocument.Periods["30d"].TimeInLowPercent,
			TimeInLowRecords: p.FullDocument.Periods["30d"].TimeInLowRecords,

			TimeInTargetMinutes: p.FullDocument.Periods["30d"].TimeInTargetMinutes,
			TimeInTargetPercent: p.FullDocument.Periods["30d"].TimeInTargetPercent,
			TimeInTargetRecords: p.FullDocument.Periods["30d"].TimeInTargetRecords,

			TimeInVeryHighMinutes: p.FullDocument.Periods["30d"].TimeInVeryHighMinutes,
			TimeInVeryHighPercent: p.FullDocument.Periods["30d"].TimeInVeryHighPercent,
			TimeInVeryHighRecords: p.FullDocument.Periods["30d"].TimeInVeryHighRecords,

			TimeInVeryLowMinutes: p.FullDocument.Periods["30d"].TimeInVeryLowMinutes,
			TimeInVeryLowPercent: p.FullDocument.Periods["30d"].TimeInVeryLowPercent,
			TimeInVeryLowRecords: p.FullDocument.Periods["30d"].TimeInVeryLowRecords,
		}}
	}
	return patientUpdate
}

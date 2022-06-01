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

	DailyStats []*Stats           `json:"dailyStats"`
	Periods    map[string]*Period `json:"periods"`

	// date tracking
	LastUpdatedDate *cdc.Date `json:"lastUpdatedDate"`
	FirstData       *cdc.Date `json:"firstData"`
	LastData        *cdc.Date `json:"lastData"`
	LastUploadDate  *cdc.Date `json:"lastUploadDate"`
	OutdatedSince   *cdc.Date `json:"outdatedSince"`

	TotalDays *int `json:"totalDays"`

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
		TotalDays:                p.FullDocument.TotalDays,
		LowGlucoseThreshold:      p.FullDocument.LowGlucoseThreshold,
		VeryLowGlucoseThreshold:  p.FullDocument.VeryLowGlucoseThreshold,
		HighGlucoseThreshold:     p.FullDocument.HighGlucoseThreshold,
		VeryHighGlucoseThreshold: p.FullDocument.VeryHighGlucoseThreshold,
	}

	var periodExists = false
	var period14dExists = false
	if p.FullDocument.Periods != nil {
		periodExists = true
		_, period14dExists = p.FullDocument.Periods["14d"]
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
	return patientUpdate
}

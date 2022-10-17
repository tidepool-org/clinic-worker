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

type CGMStats struct {
	Date *cdc.Date `json:"date" bson:"date"`

	TargetMinutes *int `json:"targetMinutes" bson:"targetMinutes"`
	TargetRecords *int `json:"targetRecords" bson:"targetRecords"`

	LowMinutes *int `json:"lowMinutes" bson:"lowMinutes"`
	LowRecords *int `json:"lowRecords" bson:"lowRecords"`

	VeryLowMinutes *int `json:"veryLowMinutes" bson:"veryLowMinutes"`
	VeryLowRecords *int `json:"veryLowRecords" bson:"veryLowRecords"`

	HighMinutes *int `json:"highMinutes" bson:"highMinutes"`
	HighRecords *int `json:"highRecords" bson:"highRecords"`

	VeryHighMinutes *int `json:"veryHighMinutes" bson:"veryHighMinutes"`
	VeryHighRecords *int `json:"veryHighRecords" bson:"veryHighRecords"`

	TotalGlucose *float64 `json:"totalGlucose" bson:"totalGlucose"`
	TotalMinutes *int     `json:"totalMinutes" bson:"totalMinutes"`
	TotalRecords *int     `json:"totalRecords" bson:"totalRecords"`

	LastRecordTime *cdc.Date `json:"lastRecordTime" bson:"lastRecordTime"`
}

type CGMPeriod struct {
	HasAverageGlucose             *bool `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	HasGlucoseManagementIndicator *bool `json:"hasGlucoseManagementIndicator" bson:"hasGlucoseManagementIndicator"`
	HasTimeCGMUsePercent          *bool `json:"hasTimeCGMUsePercent" bson:"hasTimeCGMUsePercent"`
	HasTimeInTargetPercent        *bool `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	HasTimeInHighPercent          *bool `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	HasTimeInVeryHighPercent      *bool `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	HasTimeInLowPercent           *bool `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	HasTimeInVeryLowPercent       *bool `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`

	// actual values
	TimeCGMUsePercent *float64 `json:"timeCGMUsePercent" bson:"timeCGMUsePercent"`
	TimeCGMUseMinutes *int     `json:"timeCGMUseMinutes" bson:"timeCGMUseMinutes"`
	TimeCGMUseRecords *int     `json:"timeCGMUseRecords" bson:"timeCGMUseRecords"`

	AverageGlucose             *clinics.AverageGlucose `json:"averageGlucose" bson:"avgGlucose"`
	GlucoseManagementIndicator *float64                `json:"glucoseManagementIndicator" bson:"glucoseManagementIndicator"`

	TimeInTargetPercent *float64 `json:"timeInTargetPercent" bson:"timeInTargetPercent"`
	TimeInTargetMinutes *int     `json:"timeInTargetMinutes" bson:"timeInTargetMinutes"`
	TimeInTargetRecords *int     `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	TimeInLowPercent *float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`
	TimeInLowMinutes *int     `json:"timeInLowMinutes" bson:"timeInLowMinutes"`
	TimeInLowRecords *int     `json:"timeInLowRecords" bson:"timeInLowRecords"`

	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`
	TimeInVeryLowMinutes *int     `json:"timeInVeryLowMinutes" bson:"timeInVeryLowMinutes"`
	TimeInVeryLowRecords *int     `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	TimeInHighPercent *float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`
	TimeInHighMinutes *int     `json:"timeInHighMinutes" bson:"timeInHighMinutes"`
	TimeInHighRecords *int     `json:"timeInHighRecords" bson:"timeInHighRecords"`

	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`
	TimeInVeryHighMinutes *int     `json:"timeInVeryHighMinutes" bson:"timeInVeryHighMinutes"`
	TimeInVeryHighRecords *int     `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
}

type CGMSummary struct {
	Periods     map[string]*CGMPeriod `json:"periods" bson:"periods"`
	HourlyStats []*CGMStats           `json:"hourlyStats" bson:"hourlyStats"`
	TotalHours  *int                  `json:"totalHours" bson:"totalHours"`

	// date tracking
	HasLastUploadDate *bool     `json:"hasLastUploadDate" bson:"hasLastUploadDate"`
	LastUploadDate    *cdc.Date `json:"lastUploadDate" bson:"lastUploadDate"`
	LastUpdatedDate   *cdc.Date `json:"lastUpdatedDate" bson:"lastUpdatedDate"`
	FirstData         *cdc.Date `json:"firstData" bson:"firstData"`
	LastData          *cdc.Date `json:"lastData" bson:"lastData"`
	OutdatedSince     *cdc.Date `json:"outdatedSince" bson:"outdatedSince"`
}

type BGMStats struct {
	Date *cdc.Date `json:"date" bson:"date"`

	TargetRecords   *int `json:"targetRecords" bson:"targetRecords"`
	LowRecords      *int `json:"lowRecords" bson:"lowRecords"`
	VeryLowRecords  *int `json:"veryLowRecords" bson:"veryLowRecords"`
	HighRecords     *int `json:"highRecords" bson:"highRecords"`
	VeryHighRecords *int `json:"veryHighRecords" bson:"veryHighRecords"`

	TotalGlucose *float64 `json:"totalGlucose" bson:"totalGlucose"`
	TotalRecords *int     `json:"totalRecords" bson:"totalRecords"`

	LastRecordTime *cdc.Date `json:"lastRecordTime" bson:"lastRecordTime"`
}

type BGMPeriod struct {
	HasAverageGlucose        *bool `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	HasTimeInTargetPercent   *bool `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	HasTimeInHighPercent     *bool `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	HasTimeInVeryHighPercent *bool `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	HasTimeInLowPercent      *bool `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	HasTimeInVeryLowPercent  *bool `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`

	// actual values
	AverageGlucose *clinics.AverageGlucose `json:"averageGlucose" bson:"avgGlucose"`

	TimeInTargetPercent *float64 `json:"timeInTargetPercent" bson:"timeInTargetPercent"`
	TimeInTargetRecords *int     `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	TimeInLowPercent *float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`
	TimeInLowRecords *int     `json:"timeInLowRecords" bson:"timeInLowRecords"`

	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`
	TimeInVeryLowRecords *int     `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	TimeInHighPercent *float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`
	TimeInHighRecords *int     `json:"timeInHighRecords" bson:"timeInHighRecords"`

	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`
	TimeInVeryHighRecords *int     `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
}

type BGMSummary struct {
	Periods     map[string]*BGMPeriod `json:"periods" bson:"periods"`
	HourlyStats []*BGMStats           `json:"hourlyStats" bson:"hourlyStats"`
	TotalHours  *int                  `json:"totalHours" bson:"totalHours"`

	// date tracking
	HasLastUploadDate *bool     `json:"hasLastUploadDate" bson:"hasLastUploadDate"`
	LastUploadDate    *cdc.Date `json:"lastUploadDate" bson:"lastUploadDate"`
	LastUpdatedDate   *cdc.Date `json:"lastUpdatedDate" bson:"lastUpdatedDate"`
	OutdatedSince     *cdc.Date `json:"outdatedSince" bson:"outdatedSince"`
	FirstData         *cdc.Date `json:"firstData" bson:"firstData"`
	LastData          *cdc.Date `json:"lastData" bson:"lastData"`
}

type Config struct {
	SchemaVersion *int `json:"schemaVersion" bson:"schemaVersion"`

	// these are just constants right now.
	HighGlucoseThreshold     *float64 `json:"highGlucoseThreshold" bson:"highGlucoseThreshold"`
	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold" bson:"veryHighGlucoseThreshold"`
	LowGlucoseThreshold      *float64 `json:"lowGlucoseThreshold" bson:"lowGlucoseThreshold"`
	VeryLowGlucoseThreshold  *float64 `json:"VeryLowGlucoseThreshold" bson:"VeryLowGlucoseThreshold"`
}

type Summary struct {
	ID     *cdc.ObjectId `json:"_id"`
	UserID *string       `json:"userId" bson:"userId"`

	CGM CGMSummary `json:"cgmSummary" bson:"cgmSummary"`
	BGM BGMSummary `json:"bgmSummary" bson:"bgmSummary"`

	Config Config `json:"config" bson:"config"`
}

func (p CDCEvent) CreateUpdateBody() clinics.UpdatePatientSummaryJSONRequestBody {
	var cgmFirstData *time.Time
	var cgmLastData *time.Time
	var cgmLastUpdatedDate *time.Time
	var cgmLastUploadDate *time.Time
	var cgmOutdatedSince *time.Time

	var bgmFirstData *time.Time
	var bgmLastData *time.Time
	var bgmLastUpdatedDate *time.Time
	var bgmLastUploadDate *time.Time
	var bgmOutdatedSince *time.Time

	if p.FullDocument.CGM.FirstData != nil {
		firstDataVal := time.UnixMilli(p.FullDocument.CGM.FirstData.Value)
		cgmFirstData = &firstDataVal
	}
	if p.FullDocument.CGM.LastData != nil {
		lastDataVal := time.UnixMilli(p.FullDocument.CGM.LastData.Value)
		cgmLastData = &lastDataVal
	}
	if p.FullDocument.CGM.LastUpdatedDate != nil {
		lastUpdatedDateVal := time.UnixMilli(p.FullDocument.CGM.LastUpdatedDate.Value)
		cgmLastUpdatedDate = &lastUpdatedDateVal
	}
	if p.FullDocument.CGM.LastUploadDate != nil {
		lastUploadDateVal := time.UnixMilli(p.FullDocument.CGM.LastUploadDate.Value)
		cgmLastUploadDate = &lastUploadDateVal
	}
	if p.FullDocument.CGM.OutdatedSince != nil {
		outdatedSinceVal := time.UnixMilli(p.FullDocument.CGM.OutdatedSince.Value)
		cgmOutdatedSince = &outdatedSinceVal
	}

	if p.FullDocument.BGM.FirstData != nil {
		firstDataVal := time.UnixMilli(p.FullDocument.BGM.FirstData.Value)
		bgmFirstData = &firstDataVal
	}
	if p.FullDocument.BGM.LastData != nil {
		lastDataVal := time.UnixMilli(p.FullDocument.BGM.LastData.Value)
		bgmLastData = &lastDataVal
	}
	if p.FullDocument.BGM.LastUpdatedDate != nil {
		lastUpdatedDateVal := time.UnixMilli(p.FullDocument.BGM.LastUpdatedDate.Value)
		bgmLastUpdatedDate = &lastUpdatedDateVal
	}
	if p.FullDocument.BGM.LastUploadDate != nil {
		lastUploadDateVal := time.UnixMilli(p.FullDocument.BGM.LastUploadDate.Value)
		bgmLastUploadDate = &lastUploadDateVal
	}
	if p.FullDocument.BGM.OutdatedSince != nil {
		outdatedSinceVal := time.UnixMilli(p.FullDocument.BGM.OutdatedSince.Value)
		bgmOutdatedSince = &outdatedSinceVal
	}

	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{
		CgmSummary: &clinics.PatientCGMSummary{
			FirstData:         cgmFirstData,
			HasLastUploadDate: p.FullDocument.CGM.HasLastUploadDate,
			LastData:          cgmLastData,
			LastUpdatedDate:   cgmLastUpdatedDate,
			LastUploadDate:    cgmLastUploadDate,
			OutdatedSince:     cgmOutdatedSince,
			TotalHours:        p.FullDocument.CGM.TotalHours,
		},
		BgmSummary: &clinics.PatientBGMSummary{
			FirstData:         bgmFirstData,
			HasLastUploadDate: p.FullDocument.BGM.HasLastUploadDate,
			LastData:          bgmLastData,
			LastUpdatedDate:   bgmLastUpdatedDate,
			LastUploadDate:    bgmLastUploadDate,
			OutdatedSince:     bgmOutdatedSince,
			TotalHours:        p.FullDocument.BGM.TotalHours,
		},
		Config: &clinics.PatientSummaryConfig{
			HighGlucoseThreshold:     p.FullDocument.Config.HighGlucoseThreshold,
			LowGlucoseThreshold:      p.FullDocument.Config.LowGlucoseThreshold,
			SchemaVersion:            p.FullDocument.Config.SchemaVersion,
			VeryHighGlucoseThreshold: p.FullDocument.Config.VeryHighGlucoseThreshold,
			VeryLowGlucoseThreshold:  p.FullDocument.Config.VeryLowGlucoseThreshold,
		},
	}

	if p.FullDocument.CGM.Periods != nil {
		patientUpdate.CgmSummary.Periods = &clinics.PatientCGMPeriods{}
		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*CGMPeriod{}
		destPeriods := map[string]*clinics.PatientCGMPeriod{}
		if _, exists := p.FullDocument.CGM.Periods["1d"]; exists {
			sourcePeriods["1d"] = p.FullDocument.CGM.Periods["1d"]

			patientUpdate.CgmSummary.Periods.N1d = &clinics.PatientCGMPeriod{}
			destPeriods["1d"] = patientUpdate.CgmSummary.Periods.N1d
		}
		if _, exists := p.FullDocument.CGM.Periods["7d"]; exists {
			sourcePeriods["7d"] = p.FullDocument.CGM.Periods["7d"]

			patientUpdate.CgmSummary.Periods.N7d = &clinics.PatientCGMPeriod{}
			destPeriods["7d"] = patientUpdate.CgmSummary.Periods.N7d
		}
		if _, exists := p.FullDocument.CGM.Periods["14d"]; exists {
			sourcePeriods["14d"] = p.FullDocument.CGM.Periods["14d"]

			patientUpdate.CgmSummary.Periods.N14d = &clinics.PatientCGMPeriod{}
			destPeriods["14d"] = patientUpdate.CgmSummary.Periods.N14d
		}
		if _, exists := p.FullDocument.CGM.Periods["30d"]; exists {
			sourcePeriods["30d"] = p.FullDocument.CGM.Periods["30d"]

			patientUpdate.CgmSummary.Periods.N30d = &clinics.PatientCGMPeriod{}
			destPeriods["30d"] = patientUpdate.CgmSummary.Periods.N30d
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

	if p.FullDocument.BGM.Periods != nil {
		patientUpdate.BgmSummary.Periods = &clinics.PatientBGMPeriods{}
		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*BGMPeriod{}
		destPeriods := map[string]*clinics.PatientBGMPeriod{}
		if _, exists := p.FullDocument.BGM.Periods["1d"]; exists {
			sourcePeriods["1d"] = p.FullDocument.BGM.Periods["1d"]

			patientUpdate.BgmSummary.Periods.N1d = &clinics.PatientBGMPeriod{}
			destPeriods["1d"] = patientUpdate.BgmSummary.Periods.N1d
		}
		if _, exists := p.FullDocument.BGM.Periods["7d"]; exists {
			sourcePeriods["7d"] = p.FullDocument.BGM.Periods["7d"]

			patientUpdate.BgmSummary.Periods.N7d = &clinics.PatientBGMPeriod{}
			destPeriods["7d"] = patientUpdate.BgmSummary.Periods.N7d
		}
		if _, exists := p.FullDocument.BGM.Periods["14d"]; exists {
			sourcePeriods["14d"] = p.FullDocument.BGM.Periods["14d"]

			patientUpdate.BgmSummary.Periods.N14d = &clinics.PatientBGMPeriod{}
			destPeriods["14d"] = patientUpdate.BgmSummary.Periods.N14d
		}
		if _, exists := p.FullDocument.BGM.Periods["30d"]; exists {
			sourcePeriods["30d"] = p.FullDocument.BGM.Periods["30d"]

			patientUpdate.BgmSummary.Periods.N30d = &clinics.PatientBGMPeriod{}
			destPeriods["30d"] = patientUpdate.BgmSummary.Periods.N30d
		}

		for period := range sourcePeriods {
			destPeriods[period].AverageGlucose = sourcePeriods[period].AverageGlucose
			destPeriods[period].HasAverageGlucose = sourcePeriods[period].HasAverageGlucose

			destPeriods[period].TimeInHighPercent = sourcePeriods[period].TimeInHighPercent
			destPeriods[period].HasTimeInHighPercent = sourcePeriods[period].HasTimeInHighPercent
			destPeriods[period].TimeInHighRecords = sourcePeriods[period].TimeInHighRecords

			destPeriods[period].TimeInLowPercent = sourcePeriods[period].TimeInLowPercent
			destPeriods[period].HasTimeInLowPercent = sourcePeriods[period].HasTimeInLowPercent
			destPeriods[period].TimeInLowRecords = sourcePeriods[period].TimeInLowRecords

			destPeriods[period].TimeInTargetPercent = sourcePeriods[period].TimeInTargetPercent
			destPeriods[period].HasTimeInTargetPercent = sourcePeriods[period].HasTimeInTargetPercent
			destPeriods[period].TimeInTargetRecords = sourcePeriods[period].TimeInTargetRecords

			destPeriods[period].TimeInVeryHighPercent = sourcePeriods[period].TimeInVeryHighPercent
			destPeriods[period].HasTimeInVeryHighPercent = sourcePeriods[period].HasTimeInVeryHighPercent
			destPeriods[period].TimeInVeryHighRecords = sourcePeriods[period].TimeInVeryHighRecords

			destPeriods[period].TimeInVeryLowPercent = sourcePeriods[period].TimeInVeryLowPercent
			destPeriods[period].HasTimeInVeryLowPercent = sourcePeriods[period].HasTimeInVeryLowPercent
			destPeriods[period].TimeInVeryLowRecords = sourcePeriods[period].TimeInVeryLowRecords
		}
	}

	return patientUpdate
}

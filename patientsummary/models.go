package patientsummary

import (
	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"time"
)

type CDCEvent[T Stats] struct {
	Offset        int64      `json:"-"`
	FullDocument  Summary[T] `json:"fullDocument"`
	OperationType string     `json:"operationType"`
}

type StaticCDCEvent struct {
	Offset        int64         `json:"-"`
	FullDocument  StaticSummary `json:"fullDocument"`
	OperationType string        `json:"operationType"`
}

func (p StaticCDCEvent) ShouldApplyUpdates() bool {
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

type Glucose struct {
	Units *string  `json:"units"`
	Value *float64 `json:"value"`
}

type Dates struct {
	LastUpdatedDate *cdc.Date `json:"lastUpdatedDate"`

	HasLastUploadDate *bool     `json:"hasLastUploadDate"`
	LastUploadDate    *cdc.Date `json:"lastUploadDate"`

	HasFirstData *bool     `json:"hasFirstData"`
	FirstData    *cdc.Date `json:"firstData"`

	HasLastData *bool     `json:"hasLastData"`
	LastData    *cdc.Date `json:"lastData"`

	HasOutdatedSince *bool     `json:"hasOutdatedSince"`
	OutdatedSince    *cdc.Date `json:"outdatedSince"`
}

// BGMPeriods
// For the moment, the period structure matches between the clinic and data service. We don't need to repeat these here.
// we use the clinic side instead of the summary side to guard against any additional fields the clinic service isn't
// ready to handle.
type BGMPeriods map[string]clinics.PatientBGMPeriod
type CGMPeriods map[string]clinics.PatientCGMPeriod

type CGMStats struct {
	Periods    *CGMPeriods `json:"periods"`
	TotalHours *int        `json:"totalHours"`
}

type BGMStats struct {
	Periods    *BGMPeriods `json:"periods"`
	TotalHours *int        `json:"totalHours"`
}

func (s BGMStats) GetTotalHours() *int {
	return s.TotalHours
}

func (s CGMStats) GetTotalHours() *int {
	return s.TotalHours
}

type Stats interface {
	CGMStats | BGMStats

	ExportPeriods(stats interface{})
	GetTotalHours() *int
}

type Summary[T Stats] struct {
	ID     cdc.ObjectId `json:"_id"`
	Type   *string      `json:"type"`
	UserID *string      `json:"userId"`

	Config *clinics.PatientSummaryConfig `json:"config"`

	Dates *Dates `json:"dates"`
	Stats *T     `json:"stats"`
}

type StaticSummary struct {
	ID     cdc.ObjectId `json:"_id"`
	Type   *string      `json:"type"`
	UserID *string      `json:"userId"`

	Config *clinics.PatientSummaryConfig `json:"config"`

	Dates *Dates `json:"dates"`
}

func (p CDCEvent[T]) CreateUpdateBody() clinics.UpdatePatientSummaryJSONRequestBody {
	var firstData *time.Time
	var lastData *time.Time
	var lastUpdatedDate *time.Time
	var lastUploadDate *time.Time
	var outdatedSince *time.Time

	if p.FullDocument.Dates.FirstData != nil {
		firstDataVal := time.UnixMilli(p.FullDocument.Dates.FirstData.Value)
		firstData = &firstDataVal
	}
	if p.FullDocument.Dates.LastData != nil {
		lastDataVal := time.UnixMilli(p.FullDocument.Dates.LastData.Value)
		lastData = &lastDataVal
	}
	if p.FullDocument.Dates.LastUpdatedDate != nil {
		lastUpdatedDateVal := time.UnixMilli(p.FullDocument.Dates.LastUpdatedDate.Value)
		lastUpdatedDate = &lastUpdatedDateVal
	}
	if p.FullDocument.Dates.LastUploadDate != nil {
		lastUploadDateVal := time.UnixMilli(p.FullDocument.Dates.LastUploadDate.Value)
		lastUploadDate = &lastUploadDateVal
	}
	if p.FullDocument.Dates.OutdatedSince != nil {
		outdatedSinceVal := time.UnixMilli(p.FullDocument.Dates.OutdatedSince.Value)
		outdatedSince = &outdatedSinceVal
	}

	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{}
	if *p.FullDocument.Type == "cgm" {
		patientUpdate.CgmStats = &clinics.PatientCGMStats{}

		patientUpdate.CgmStats.Dates = &clinics.PatientSummaryDates{
			LastUpdatedDate: lastUpdatedDate,

			HasLastUploadDate: p.FullDocument.Dates.HasLastUploadDate,
			LastUploadDate:    lastUploadDate,

			HasFirstData: p.FullDocument.Dates.HasFirstData,
			FirstData:    firstData,

			HasLastData: p.FullDocument.Dates.HasLastData,
			LastData:    lastData,

			HasOutdatedSince: p.FullDocument.Dates.HasOutdatedSince,
			OutdatedSince:    outdatedSince,
		}

		patientUpdate.CgmStats.Config = p.FullDocument.Config

		if p.FullDocument.Stats != nil {
			patientUpdate.CgmStats.TotalHours = (*p.FullDocument.Stats).GetTotalHours()
			(*p.FullDocument.Stats).ExportPeriods(patientUpdate.CgmStats)
		}

	} else if *p.FullDocument.Type == "bgm" {
		patientUpdate.BgmStats = &clinics.PatientBGMStats{}

		patientUpdate.BgmStats.Dates = &clinics.PatientSummaryDates{
			LastUpdatedDate: lastUpdatedDate,

			HasLastUploadDate: p.FullDocument.Dates.HasLastUploadDate,
			LastUploadDate:    lastUploadDate,

			HasFirstData: p.FullDocument.Dates.HasFirstData,
			FirstData:    firstData,

			HasLastData: p.FullDocument.Dates.HasLastData,
			LastData:    lastData,

			HasOutdatedSince: p.FullDocument.Dates.HasOutdatedSince,
			OutdatedSince:    outdatedSince,
		}

		patientUpdate.BgmStats.Config = p.FullDocument.Config

		if p.FullDocument.Stats != nil {
			patientUpdate.BgmStats.TotalHours = (*p.FullDocument.Stats).GetTotalHours()
			(*p.FullDocument.Stats).ExportPeriods(patientUpdate.BgmStats)
		}
	}

	return patientUpdate
}

func (s CGMStats) ExportPeriods(destStatsInt interface{}) {
	var destStats = destStatsInt.(*clinics.PatientCGMStats)

	if s.Periods != nil {
		destStats.Periods = &clinics.PatientCGMPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		if v, exists := (*s.Periods)["1d"]; exists {
			destStats.Periods.N1d = &v
		}
		if v, exists := (*s.Periods)["7d"]; exists {
			destStats.Periods.N7d = &v
		}
		if v, exists := (*s.Periods)["14d"]; exists {
			destStats.Periods.N14d = &v
		}
		if v, exists := (*s.Periods)["30d"]; exists {
			destStats.Periods.N30d = &v
		}
	}
}

func (s BGMStats) ExportPeriods(destStatsInt interface{}) {
	var destStats = destStatsInt.(*clinics.PatientBGMStats)

	if s.Periods != nil {
		destStats.Periods = &clinics.PatientBGMPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		if v, exists := (*s.Periods)["1d"]; exists {
			destStats.Periods.N1d = &v
		}
		if v, exists := (*s.Periods)["7d"]; exists {
			destStats.Periods.N7d = &v
		}
		if v, exists := (*s.Periods)["14d"]; exists {
			destStats.Periods.N14d = &v
		}
		if v, exists := (*s.Periods)["30d"]; exists {
			destStats.Periods.N30d = &v
		}
	}
}

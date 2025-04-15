package patientsummary

import (
	"regexp"
	"strconv"
	"time"

	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	summaries "github.com/tidepool-org/go-common/clients/summary"
)

type DocumentKey struct {
	cdc.ObjectId `json:"_id"`
}

type CDCEvent struct {
	Offset        int64       `json:"-"`
	FullDocument  Summary     `json:"fullDocument"`
	OperationType string      `json:"operationType"`
	DocumentKey   DocumentKey `json:"documentKey"`
}

var empty any
var supportedOps = map[string]any{
	cdc.OperationTypeInsert:  empty,
	cdc.OperationTypeUpdate:  empty,
	cdc.OperationTypeReplace: empty,
	cdc.OperationTypeDelete:  empty,
}

func (p CDCEvent) ShouldApplyUpdates() bool {
	if _, ok := supportedOps[p.OperationType]; !ok {
		return false
	}

	if p.FullDocument.UserID == "" {
		return false
	}

	return true
}

type Glucose struct {
	Units *string  `json:"units"`
	Value *float64 `json:"value"`
}

type Dates struct {
	FirstData         *cdc.Date `json:"firstData"`
	LastData          *cdc.Date `json:"lastData"`
	LastUpdatedDate   *cdc.Date `json:"lastUpdatedDate"`
	LastUpdatedReason []string  `json:"lastUpdatedReason,omitempty"`
	LastUploadDate    *cdc.Date `json:"lastUploadDate"`
	OutdatedReason    []string  `json:"outdatedReason,omitempty"`
	OutdatedSince     *cdc.Date `json:"outdatedSince,omitempty"`
}

type BaseSummary struct {
	ID     cdc.ObjectId       `json:"_id"`
	Type   string             `json:"type"`
	UserID string             `json:"userId"`
	Config summaries.ConfigV1 `json:"config"`
	Dates  Dates              `json:"dates"`
}

type Summary struct {
	BaseSummary `json:",inline"`
	Periods     *summaries.SummaryV5_Periods `json:"periods"`
}

func (p CDCEvent) CreateUpdateBody() (*clinics.UpdatePatientSummaryJSONRequestBody, error) {
	var firstData *time.Time
	if p.FullDocument.Dates.FirstData != nil {
		firstDataVal := time.UnixMilli(p.FullDocument.Dates.FirstData.Value)
		firstData = &firstDataVal
	}

	var lastData *time.Time
	if p.FullDocument.Dates.LastData != nil {
		lastDataVal := time.UnixMilli(p.FullDocument.Dates.LastData.Value)
		lastData = &lastDataVal
	}

	var lastUpdatedDate *time.Time
	if p.FullDocument.Dates.LastUpdatedDate != nil {
		lastUpdatedDateVal := time.UnixMilli(p.FullDocument.Dates.LastUpdatedDate.Value)
		lastUpdatedDate = &lastUpdatedDateVal
	}

	var lastUploadDate *time.Time
	if p.FullDocument.Dates.LastUploadDate != nil {
		lastUploadDateVal := time.UnixMilli(p.FullDocument.Dates.LastUploadDate.Value)
		lastUploadDate = &lastUploadDateVal
	}

	var outdatedSince *time.Time
	if p.FullDocument.Dates.OutdatedSince != nil {
		outdatedSinceVal := time.UnixMilli(p.FullDocument.Dates.OutdatedSince.Value)
		outdatedSince = &outdatedSinceVal
	}

	if p.FullDocument.Dates.OutdatedReason == nil {
		p.FullDocument.Dates.OutdatedReason = []string{}
	}

	if p.FullDocument.Dates.LastUpdatedReason == nil {
		p.FullDocument.Dates.LastUpdatedReason = []string{}
	}

	dates := clinics.PatientSummaryDates{
		LastUpdatedDate:   lastUpdatedDate,
		LastUpdatedReason: &p.FullDocument.Dates.LastUpdatedReason,
		OutdatedReason:    &p.FullDocument.Dates.OutdatedReason,
		HasLastUploadDate: lastUploadDate != nil,
		LastUploadDate:    lastUploadDate,
		HasFirstData:      firstData != nil,
		FirstData:         firstData,
		HasLastData:       lastData != nil,
		LastData:          lastData,
		HasOutdatedSince:  outdatedSince != nil,
		OutdatedSince:     outdatedSince,
	}

	config := clinics.PatientSummaryConfig(p.FullDocument.Config)

	patientUpdate := &clinics.UpdatePatientSummaryJSONRequestBody{}
	if p.FullDocument.Type == "cgm" {
		patientUpdate.CgmStats = &clinics.PatientCGMStats{
			Id:     &p.FullDocument.ID.Value,
			Dates:  dates,
			Config: config,
		}

		if p.FullDocument.Periods != nil {
			sourceCGMPeriod, err := p.FullDocument.Periods.AsCgmperiodsV5()
			if err != nil {
				return nil, err
			}
			ExportCGMPeriods(sourceCGMPeriod, patientUpdate.CgmStats)
		}

	} else if p.FullDocument.Type == "bgm" {
		patientUpdate.BgmStats = &clinics.PatientBGMStats{
			Id:     &p.FullDocument.ID.Value,
			Dates:  dates,
			Config: config,
		}

		if p.FullDocument.Periods != nil {
			sourceBGMPeriod, err := p.FullDocument.Periods.AsBgmperiodsV5()
			if err != nil {
				return nil, err
			}
			ExportBGMPeriods(sourceBGMPeriod, patientUpdate.BgmStats)
		}
	}

	return patientUpdate, nil
}

func ExportCGMPeriods(sourcePeriods summaries.CgmperiodsV5, destPeriods *clinics.PatientCGMStats) {
	daysRe := regexp.MustCompile("(\\d+)d")

	if sourcePeriods != nil {
		destPeriods.Periods = clinics.PatientCGMPeriods{}
		for k := range sourcePeriods {
			// get integer portion of 1d/7d/14d/30d map string
			m := daysRe.FindStringSubmatch(k)
			if len(m) >= 2 {
				i, _ := strconv.Atoi(m[1])
				destPeriods.Periods[k] = ExportCGMPeriod(sourcePeriods[k], i)
			}
		}
	}
}

func ExportCGMPeriod(period summaries.GlucoseperiodV5, i int) clinics.PatientCGMPeriod {
	destPeriod := clinics.PatientCGMPeriod{
		AverageDailyRecords:           &period.AverageDailyRecords,
		AverageDailyRecordsDelta:      &period.Delta.AverageDailyRecords,
		DaysWithData:                  period.DaysWithData,
		DaysWithDataDelta:             period.Delta.DaysWithData,
		HasAverageDailyRecords:        period.AverageDailyRecords != 0,
		HasTimeCGMUseMinutes:          period.Total.Minutes != 0,
		HasTimeCGMUseRecords:          period.Total.Records != 0,
		HasTimeInAnyHighMinutes:       period.InAnyHigh.Minutes != 0,
		HasTimeInAnyHighRecords:       period.InAnyHigh.Records != 0,
		HasTimeInAnyLowMinutes:        period.InAnyLow.Minutes != 0,
		HasTimeInAnyLowRecords:        period.InAnyLow.Records != 0,
		HasTimeInExtremeHighMinutes:   period.InExtremeHigh.Minutes != 0,
		HasTimeInExtremeHighRecords:   period.InExtremeHigh.Records != 0,
		HasTimeInHighMinutes:          period.InHigh.Minutes != 0,
		HasTimeInHighRecords:          period.InHigh.Records != 0,
		HasTimeInLowMinutes:           period.InLow.Minutes != 0,
		HasTimeInLowRecords:           period.InLow.Records != 0,
		HasTimeInTargetMinutes:        period.InTarget.Minutes != 0,
		HasTimeInTargetRecords:        period.InTarget.Records != 0,
		HasTimeInVeryHighMinutes:      period.InVeryHigh.Minutes != 0,
		HasTimeInVeryHighRecords:      period.InVeryHigh.Records != 0,
		HasTimeInVeryLowMinutes:       period.InVeryLow.Minutes != 0,
		HasTimeInVeryLowRecords:       period.InVeryLow.Records != 0,
		HasTotalRecords:               period.Total.Records != 0,
		HoursWithData:                 period.HoursWithData,
		HoursWithDataDelta:            period.Delta.HoursWithData,
		TimeCGMUseMinutes:             &period.Total.Minutes,
		TimeCGMUseMinutesDelta:        &period.Delta.Total.Minutes,
		TimeCGMUseRecords:             &period.Total.Records,
		TimeCGMUseRecordsDelta:        &period.Delta.Total.Records,
		TimeInAnyHighMinutes:          &period.InAnyHigh.Minutes,
		TimeInAnyHighMinutesDelta:     &period.Delta.InAnyHigh.Minutes,
		TimeInAnyHighRecords:          &period.InAnyHigh.Records,
		TimeInAnyHighRecordsDelta:     &period.Delta.InAnyHigh.Records,
		TimeInAnyLowMinutes:           &period.InAnyLow.Minutes,
		TimeInAnyLowMinutesDelta:      &period.Delta.InAnyLow.Minutes,
		TimeInAnyLowRecords:           &period.InAnyLow.Records,
		TimeInAnyLowRecordsDelta:      &period.Delta.InAnyLow.Records,
		TimeInExtremeHighMinutes:      &period.InExtremeHigh.Minutes,
		TimeInExtremeHighMinutesDelta: &period.Delta.InExtremeHigh.Minutes,
		TimeInExtremeHighRecords:      &period.InExtremeHigh.Records,
		TimeInExtremeHighRecordsDelta: &period.Delta.InExtremeHigh.Records,
		TimeInHighMinutes:             &period.InHigh.Minutes,
		TimeInHighMinutesDelta:        &period.Delta.InHigh.Minutes,
		TimeInHighRecords:             &period.InHigh.Records,
		TimeInHighRecordsDelta:        &period.Delta.InHigh.Records,
		TimeInLowMinutes:              &period.InLow.Minutes,
		TimeInLowMinutesDelta:         &period.Delta.InLow.Minutes,
		TimeInLowRecords:              &period.InLow.Records,
		TimeInLowRecordsDelta:         &period.Delta.InLow.Records,
		TimeInTargetMinutes:           &period.InTarget.Minutes,
		TimeInTargetMinutesDelta:      &period.Delta.InTarget.Minutes,
		TimeInTargetRecords:           &period.InTarget.Records,
		TimeInTargetRecordsDelta:      &period.Delta.InTarget.Records,
		TimeInVeryHighMinutes:         &period.InVeryHigh.Minutes,
		TimeInVeryHighMinutesDelta:    &period.Delta.InVeryHigh.Minutes,
		TimeInVeryHighRecords:         &period.InVeryHigh.Records,
		TimeInVeryHighRecordsDelta:    &period.Delta.InVeryHigh.Records,
		TimeInVeryLowMinutes:          &period.InVeryLow.Minutes,
		TimeInVeryLowMinutesDelta:     &period.Delta.InVeryLow.Minutes,
		TimeInVeryLowRecords:          &period.InVeryLow.Records,
		TimeInVeryLowRecordsDelta:     &period.Delta.InVeryLow.Records,
		TotalRecords:                  &period.Total.Records,
		TotalRecordsDelta:             &period.Delta.Total.Records,
	}

	// The following provides concessions to allow patient list sorting and filtering according to
	// certain eligibility requirements, notably:
	// - TIR percent only is visible in the frontend if >1d of data, or 70% cgm use on single day metrics
	// - GMI requires >70% cgm use
	// - All percentages should be nil if 0 TotalRecords, as they would have been before schema v5
	if *destPeriod.TotalRecords != 0 {
		destPeriod.HasTimeCGMUsePercent = true
		destPeriod.TimeCGMUsePercent = &period.Total.Percent

		// if we are storing under 1d, apply 70% rule to TimeIn*
		// if we are storing over 1d, check for 24h cgm use
		if (i <= 1 && *destPeriod.TimeCGMUsePercent > 0.7) || (i > 1 && *destPeriod.TimeCGMUseMinutes > 1440) {
			destPeriod.HasTimeInTargetPercent = true
			destPeriod.TimeInTargetPercent = &period.InTarget.Percent
			destPeriod.TimeInTargetPercentDelta = &period.Delta.InTarget.Percent

			destPeriod.HasTimeInLowPercent = true
			destPeriod.TimeInLowPercent = &period.InLow.Percent
			destPeriod.TimeInLowPercentDelta = &period.Delta.InLow.Percent

			destPeriod.HasTimeInVeryLowPercent = true
			destPeriod.TimeInVeryLowPercent = &period.InVeryLow.Percent
			destPeriod.TimeInVeryLowPercentDelta = &period.Delta.InVeryLow.Percent

			destPeriod.HasTimeInAnyLowPercent = true
			destPeriod.TimeInAnyLowPercent = &period.InAnyLow.Percent
			destPeriod.TimeInAnyLowPercentDelta = &period.Delta.InAnyLow.Percent

			destPeriod.HasTimeInHighPercent = true
			destPeriod.TimeInHighPercent = &period.InHigh.Percent
			destPeriod.TimeInHighPercentDelta = &period.Delta.InHigh.Percent

			destPeriod.HasTimeInVeryHighPercent = true
			destPeriod.TimeInVeryHighPercent = &period.InVeryHigh.Percent
			destPeriod.TimeInVeryHighPercentDelta = &period.Delta.InVeryHigh.Percent

			destPeriod.HasTimeInExtremeHighPercent = true
			destPeriod.TimeInExtremeHighPercent = &period.InExtremeHigh.Percent
			destPeriod.TimeInExtremeHighPercentDelta = &period.Delta.InExtremeHigh.Percent

			destPeriod.HasTimeInAnyHighPercent = true
			destPeriod.TimeInAnyHighPercent = &period.InAnyHigh.Percent
			destPeriod.TimeInAnyHighPercentDelta = &period.Delta.InAnyHigh.Percent
		}

		destPeriod.HasAverageGlucoseMmol = true
		destPeriod.AverageGlucoseMmol = &period.AverageGlucoseMmol
		destPeriod.AverageGlucoseMmolDelta = &period.Delta.AverageGlucoseMmol

		// GMI should only be present if CGM use % is >70% so that they are filtered to the bottom on GMI queries.
		if *destPeriod.TimeCGMUsePercent > 0.7 {
			destPeriod.HasGlucoseManagementIndicator = true
			destPeriod.GlucoseManagementIndicator = &period.GlucoseManagementIndicator
			destPeriod.GlucoseManagementIndicatorDelta = &period.Delta.GlucoseManagementIndicator
		}

		destPeriod.StandardDeviation = period.StandardDeviation
		destPeriod.StandardDeviationDelta = period.Delta.StandardDeviation

		destPeriod.CoefficientOfVariation = period.CoefficientOfVariation
		destPeriod.CoefficientOfVariationDelta = period.Delta.CoefficientOfVariation
	}

	return destPeriod
}

func ExportBGMPeriods(sourcePeriods summaries.BgmperiodsV5, destPeriods *clinics.PatientBGMStats) {
	daysRe := regexp.MustCompile("(\\d+)d")

	if sourcePeriods != nil {
		destPeriods.Periods = clinics.PatientBGMPeriods{}
		for k := range sourcePeriods {
			// get integer portion of 1d/7d/14d/30d map string
			m := daysRe.FindStringSubmatch(k)
			if len(m) >= 2 {
				destPeriods.Periods[k] = ExportBGMPeriod(sourcePeriods[k])
			}
		}
	}
}

func ExportBGMPeriod(period summaries.GlucoseperiodV5) clinics.PatientBGMPeriod {
	destPeriod := clinics.PatientBGMPeriod{
		AverageDailyRecords:           &period.AverageDailyRecords,
		AverageDailyRecordsDelta:      &period.Delta.AverageDailyRecords,
		HasAverageDailyRecords:        period.AverageDailyRecords != 0,
		HasTimeInAnyHighRecords:       period.InAnyHigh.Records != 0,
		HasTimeInAnyLowRecords:        period.InAnyLow.Records != 0,
		HasTimeInExtremeHighRecords:   period.InExtremeHigh.Records != 0,
		HasTimeInHighRecords:          period.InHigh.Records != 0,
		HasTimeInLowRecords:           period.InLow.Records != 0,
		HasTimeInTargetRecords:        period.InTarget.Records != 0,
		HasTimeInVeryHighRecords:      period.InVeryHigh.Records != 0,
		HasTimeInVeryLowRecords:       period.InVeryLow.Records != 0,
		HasTotalRecords:               period.Total.Records != 0,
		TimeInAnyHighRecords:          &period.InAnyHigh.Records,
		TimeInAnyHighRecordsDelta:     &period.Delta.InAnyHigh.Records,
		TimeInAnyLowRecords:           &period.InAnyLow.Records,
		TimeInAnyLowRecordsDelta:      &period.Delta.InAnyLow.Records,
		TimeInExtremeHighRecords:      &period.InExtremeHigh.Records,
		TimeInExtremeHighRecordsDelta: &period.Delta.InExtremeHigh.Records,
		TimeInHighRecords:             &period.InHigh.Records,
		TimeInHighRecordsDelta:        &period.Delta.InHigh.Records,
		TimeInLowRecords:              &period.InLow.Records,
		TimeInLowRecordsDelta:         &period.Delta.InLow.Records,
		TimeInTargetRecords:           &period.InTarget.Records,
		TimeInTargetRecordsDelta:      &period.Delta.InTarget.Records,
		TimeInVeryHighRecords:         &period.InVeryHigh.Records,
		TimeInVeryHighRecordsDelta:    &period.Delta.InVeryHigh.Records,
		TimeInVeryLowRecords:          &period.InVeryLow.Records,
		TimeInVeryLowRecordsDelta:     &period.Delta.InVeryLow.Records,
		TotalRecords:                  &period.Total.Records,
		TotalRecordsDelta:             &period.Delta.Total.Records,
	}

	// percentages should stay nil unless there is records, but schema >5 removed all optional pointers
	if *destPeriod.TotalRecords != 0 {
		destPeriod.HasTimeInTargetPercent = true
		destPeriod.TimeInTargetPercent = &period.InTarget.Percent
		destPeriod.TimeInTargetPercentDelta = &period.Delta.InTarget.Percent

		destPeriod.HasTimeInLowPercent = true
		destPeriod.TimeInLowPercent = &period.InLow.Percent
		destPeriod.TimeInLowPercentDelta = &period.Delta.InLow.Percent

		destPeriod.HasTimeInVeryLowPercent = true
		destPeriod.TimeInVeryLowPercent = &period.InVeryLow.Percent
		destPeriod.TimeInVeryLowPercentDelta = &period.Delta.InVeryLow.Percent

		destPeriod.HasTimeInAnyLowPercent = true
		destPeriod.TimeInAnyLowPercent = &period.InAnyLow.Percent
		destPeriod.TimeInAnyLowPercentDelta = &period.Delta.InAnyLow.Percent

		destPeriod.HasTimeInHighPercent = true
		destPeriod.TimeInHighPercent = &period.InHigh.Percent
		destPeriod.TimeInHighPercentDelta = &period.Delta.InHigh.Percent

		destPeriod.HasTimeInVeryHighPercent = true
		destPeriod.TimeInVeryHighPercent = &period.InVeryHigh.Percent
		destPeriod.TimeInVeryHighPercentDelta = &period.Delta.InVeryHigh.Percent

		destPeriod.HasTimeInExtremeHighPercent = true
		destPeriod.TimeInExtremeHighPercent = &period.InExtremeHigh.Percent
		destPeriod.TimeInExtremeHighPercentDelta = &period.Delta.InExtremeHigh.Percent

		destPeriod.HasTimeInAnyHighPercent = true
		destPeriod.TimeInAnyHighPercent = &period.InAnyHigh.Percent
		destPeriod.TimeInAnyHighPercentDelta = &period.Delta.InAnyHigh.Percent

		destPeriod.HasAverageGlucoseMmol = true
		destPeriod.AverageGlucoseMmol = &period.AverageGlucoseMmol
		destPeriod.AverageGlucoseMmolDelta = &period.Delta.AverageGlucoseMmol

	}

	return destPeriod
}

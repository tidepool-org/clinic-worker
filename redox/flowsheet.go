package redox

import (
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/redox/models"
	"time"
)

const (
	missingValue = "NOT AVAILABLE"
	days14       = 14 * 24 * time.Hour
)

func NewFlowsheet() models.NewFlowsheet {
	flowsheet := models.NewFlowsheet{}
	now := time.Now().Format(time.RFC3339)

	flowsheet.Meta.EventType = "New"
	flowsheet.Meta.DataModel = "Flowsheet"
	flowsheet.Meta.EventDateTime = &now
	return flowsheet
}

// PopulateSummaryStatistics populates a flowsheet with patient summary statistics. If summary statistics are not available,
// the flowsheet items will be populated with 'NOT AVAILABLE'.
func PopulateSummaryStatistics(patient clinics.Patient, flowsheet *models.NewFlowsheet) {
	var cgmStats *clinics.PatientCGMStats
	var bgmStats *clinics.PatientBGMStats
	if patient.Summary != nil {
		cgmStats = patient.Summary.CgmStats
		bgmStats = patient.Summary.BgmStats
	}
	PopulateCGMObservations(cgmStats, flowsheet)
	PopulateBGMObservations(bgmStats, flowsheet)
}

func PopulateCGMObservations(stats *clinics.PatientCGMStats, f *models.NewFlowsheet) {
	now := time.Now()

	var period *clinics.PatientCGMPeriod
	periodDuration := days14
	reportingTime := formatTime(&now)
	var firstData, periodEnd, periodStart *time.Time

	if stats != nil && stats.Dates != nil {
		reportingTime = formatTime(stats.Dates.LastUpdatedDate)
		if stats.Periods != nil {
			period = stats.Periods.N14d
		}

		firstData = stats.Dates.FirstData
		periodEnd = stats.Dates.LastData
		if periodEnd != nil {
			start := periodEnd.Add(-periodDuration)
			periodStart = &start
			if firstData.Before(start) {
				firstData = periodStart
			}
		}
	}

	var cgmUsePercent *float64
	var averageGlucose *float64
	var averageGlucoseUnits *string
	var gmi *float64
	var timeInVeryLow *float64
	var timeInLow *float64
	var timeInTarget *float64
	var timeInHigh *float64
	var timeInVeryHigh *float64

	if period != nil {
		if period.AverageGlucose != nil {
			val := float64(period.AverageGlucose.Value)
			units := string(period.AverageGlucose.Units)

			averageGlucose = &val
			averageGlucoseUnits = &units
		}
		cgmUsePercent = period.TimeCGMUsePercent
		gmi = period.GlucoseManagementIndicator
		timeInVeryLow = period.TimeInVeryLowPercent
		timeInLow = period.TimeInLowPercent
		timeInTarget = period.TimeInTargetPercent
		timeInHigh = period.TimeInHighPercent
		timeInVeryHigh = period.TimeInVeryHighPercent
	}

	f.Observations = append(f.Observations,
		CreateObservation("REPORTING_PERIOD_START_CGM", formatTime(periodStart), "DateTime", nil, "CGM Reporting Period Start", reportingTime),
		CreateObservation("REPORTING_PERIOD_END_CGM", formatTime(periodEnd), "DateTime", nil, "CGM Reporting Period End", reportingTime),
		CreateObservation("REPORTING_PERIOD_START_CGM_DATA", formatTime(firstData), "DateTime", nil, "CGM Reporting Period Start Date of actual Data", reportingTime),

		CreateObservation("ACTIVE_WEAR_TIME_CGM", formatFloat(cgmUsePercent), "Number", nil, "Percentage of time CGM worn during reporting period", reportingTime),
		CreateObservation("AVERAGE_CGM", formatFloat(averageGlucose), "Number", averageGlucoseUnits, "CGM Average Glucose during reporting period", reportingTime),
		CreateObservation("GLUCOSE_MANAGEMENT_INDICATOR", formatFloat(gmi), "Number", nil, "CGM Glucose Management Indicator during reporting period", reportingTime),

		CreateObservation("TIME_BELOW_RANGE_VERY_LOW_CGM", formatFloat(timeInVeryLow), "Number", nil, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)", reportingTime),
		CreateObservation("TIME_BELOW_RANGE_LOW_CGM", formatFloat(timeInLow), "Number", nil, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)", reportingTime),
		CreateObservation("TIME_IN_RANGE_CGM", formatFloat(timeInTarget), "Number", nil, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)", reportingTime),
		CreateObservation("TIME_ABOVE_RANGE_HIGH_CGM", formatFloat(timeInHigh), "Number", nil, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)", reportingTime),
		CreateObservation("TIME_ABOVE_RANGE_VERY_HIGH_CGM", formatFloat(timeInVeryHigh), "Number", nil, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)", reportingTime),
	)
}

func PopulateBGMObservations(stats *clinics.PatientBGMStats, f *models.NewFlowsheet) {
	now := time.Now()

	var period *clinics.PatientBGMPeriod
	periodDuration := days14
	reportingTime := formatTime(&now)

	var firstData, periodEnd, periodStart *time.Time
	if stats.Dates != nil {
		reportingTime = formatTime(stats.Dates.LastUpdatedDate)
		if stats.Periods != nil {
			period = stats.Periods.N14d
		}

		firstData = stats.Dates.FirstData
		periodEnd = stats.Dates.LastData
		if periodEnd != nil {
			start := periodEnd.Add(-periodDuration)
			periodStart = &start
			if firstData.Before(start) {
				firstData = periodStart
			}
		}
	}

	var averageDailyRecords *float64
	var averageGlucose *float64
	var averageGlucoseUnits *string
	var timeInVeryLowRecords *int
	var timeInVeryHighRecords *int

	if period != nil {
		if period.AverageGlucose != nil {
			val := float64(period.AverageGlucose.Value)
			units := string(period.AverageGlucose.Units)

			averageGlucose = &val
			averageGlucoseUnits = &units
		}
		averageDailyRecords = period.AverageDailyRecords

		timeInVeryLowRecords = period.TimeInVeryLowRecords
		timeInVeryHighRecords = period.TimeInVeryHighRecords
	}

	f.Observations = append(f.Observations,
		CreateObservation("REPORTING_PERIOD_START_SMBG", formatTime(periodStart), "DateTime", nil, "SMBG Reporting Period Start", reportingTime),
		CreateObservation("REPORTING_PERIOD_END_SMBG", formatTime(periodEnd), "DateTime", nil, "SMBG Reporting Period End", reportingTime),
		CreateObservation("REPORTING_PERIOD_START_SMBG_DATA", formatTime(firstData), "DateTime", nil, "SMBG Reporting Period Start Date of actual Data", reportingTime),

		CreateObservation("CHECK_RATE_READINGS_DAY_SMBG", formatFloat(averageDailyRecords), "Number", nil, "Average number of SMBG readings per day during reporting period", reportingTime),
		CreateObservation("AVERAGE_SMBG", formatFloat(averageGlucose), "Number", averageGlucoseUnits, "SMBG Average Glucose during reporting period", reportingTime),

		CreateObservation("READINGS_BELOW_RANGE_VERY_LOW_SMBG", formatInt(timeInVeryLowRecords), "Number", nil, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)", reportingTime),
		CreateObservation("READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", formatInt(timeInVeryHighRecords), "Number", nil, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)", reportingTime),
	)
}

func CreateObservation(code, value, valueType string, units *string, description, dateTime string) (res struct {
	AbnormalFlag *string        `json:"AbnormalFlag"`
	Code         string         `json:"Code"`
	Codeset      *string        `json:"Codeset"`
	DateTime     string         `json:"DateTime"`
	Description  *string        `json:"Description"`
	Notes        *[]interface{} `json:"Notes,omitempty"`
	Observer     *struct {
		FirstName *string `json:"FirstName"`
		ID        *string `json:"ID"`
		IDType    *string `json:"IDType"`
		LastName  *string `json:"LastName"`
	} `json:"Observer,omitempty"`
	ReferenceRange *struct {
		High *float32 `json:"High"`
		Low  *float32 `json:"Low"`
		Text *string  `json:"Text"`
	} `json:"ReferenceRange,omitempty"`
	Status    *string `json:"Status"`
	Units     *string `json:"Units"`
	Value     string  `json:"Value"`
	ValueType string  `json:"ValueType"`
}) {
	res.Code = code
	res.Value = value
	res.ValueType = valueType
	res.Units = units
	res.Description = &description
	res.DateTime = dateTime
	return
}

func formatTime(t *time.Time) string {
	if t == nil {
		return missingValue
	}
	return t.Format(time.RFC3339)
}

func formatInt(val *int) string {
	if val == nil {
		return missingValue
	}
	return fmt.Sprintf("%d", *val)
}

func formatFloat(val *float64) string {
	if val == nil {
		return missingValue
	}
	return fmt.Sprintf("%.4f", *val)
}

package redox

import (
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"strings"
	"time"
)

const (
	EventTypeNewFlowsheet = "New"
	DataModelFlowsheet    = "Flowsheet"

	MmolLToMgdLConversionFactor float64 = 18.01559
	MmolLToMgdLPrecisionFactor  float64 = 100000.0

	missingValue = "NOT AVAILABLE"
	days14       = 14 * 24 * time.Hour
	percentage   = "%"
)

func NewFlowsheet() models.NewFlowsheet {
	flowsheet := models.NewFlowsheet{}
	now := time.Now().Format(time.RFC3339)

	flowsheet.Meta.EventType = EventTypeNewFlowsheet
	flowsheet.Meta.DataModel = DataModelFlowsheet
	flowsheet.Meta.EventDateTime = &now
	return flowsheet
}

// PopulateSummaryStatistics populates a flowsheet with patient summary statistics. If summary statistics are not available,
// the flowsheet items will be populated with 'NOT AVAILABLE'.
func PopulateSummaryStatistics(patient clinics.Patient, clinic clinics.Clinic, flowsheet *models.NewFlowsheet) {
	var cgmStats *clinics.PatientCGMStats
	var bgmStats *clinics.PatientBGMStats
	if patient.Summary != nil {
		cgmStats = patient.Summary.CgmStats
		bgmStats = patient.Summary.BgmStats
	}

	PopulateCGMObservations(cgmStats, clinic.PreferredBgUnits, flowsheet)
	PopulateBGMObservations(bgmStats, clinic.PreferredBgUnits, flowsheet)
}

func PopulateCGMObservations(stats *clinics.PatientCGMStats, preferredBgUnits clinics.ClinicPreferredBgUnits, f *models.NewFlowsheet) {
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

	unitsPercentage := percentage

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

			// Convert blood glucose to preferred units
			val, units = bgInUnits(val, units, string(preferredBgUnits))

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

		CreateObservation("ACTIVE_WEAR_TIME_CGM", formatFloat(unitIntervalToPercent(cgmUsePercent)), "Numeric", &unitsPercentage, "Percentage of time CGM worn during reporting period", reportingTime),
		CreateObservation("AVERAGE_CGM", formatFloat(averageGlucose), "Numeric", averageGlucoseUnits, "CGM Average Glucose during reporting period", reportingTime),
		CreateObservation("GLUCOSE_MANAGEMENT_INDICATOR", formatFloat(gmi), "Numeric", nil, "CGM Glucose Management Indicator during reporting period", reportingTime),

		CreateObservation("TIME_BELOW_RANGE_VERY_LOW_CGM", formatFloat(unitIntervalToPercent(timeInVeryLow)), "Numeric", &unitsPercentage, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)", reportingTime),
		CreateObservation("TIME_BELOW_RANGE_LOW_CGM", formatFloat(unitIntervalToPercent(timeInLow)), "Numeric", &unitsPercentage, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)", reportingTime),
		CreateObservation("TIME_IN_RANGE_CGM", formatFloat(unitIntervalToPercent(timeInTarget)), "Numeric", &unitsPercentage, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)", reportingTime),
		CreateObservation("TIME_ABOVE_RANGE_HIGH_CGM", formatFloat(unitIntervalToPercent(timeInHigh)), "Numeric", &unitsPercentage, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)", reportingTime),
		CreateObservation("TIME_ABOVE_RANGE_VERY_HIGH_CGM", formatFloat(unitIntervalToPercent(timeInVeryHigh)), "Numeric", &unitsPercentage, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)", reportingTime),
	)
}

func PopulateBGMObservations(stats *clinics.PatientBGMStats, preferredBgUnits clinics.ClinicPreferredBgUnits, f *models.NewFlowsheet) {
	now := time.Now()

	var period *clinics.PatientBGMPeriod
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

	var averageDailyRecords *float64
	var averageGlucose *float64
	var averageGlucoseUnits *string
	var timeInVeryLowRecords *int
	var timeInVeryHighRecords *int

	if period != nil {
		if period.AverageGlucose != nil {
			val := float64(period.AverageGlucose.Value)
			units := string(period.AverageGlucose.Units)

			// Convert blood glucose to preferred units
			val, units = bgInUnits(val, units, string(preferredBgUnits))

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

		CreateObservation("CHECK_RATE_READINGS_DAY_SMBG", formatFloat(averageDailyRecords), "Numeric", nil, "Average Numeric of SMBG readings per day during reporting period", reportingTime),
		CreateObservation("AVERAGE_SMBG", formatFloat(averageGlucose), "Numeric", averageGlucoseUnits, "SMBG Average Glucose during reporting period", reportingTime),

		CreateObservation("READINGS_BELOW_RANGE_VERY_LOW_SMBG", formatInt(timeInVeryLowRecords), "Numeric", nil, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period", reportingTime),
		CreateObservation("READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", formatInt(timeInVeryHighRecords), "Numeric", nil, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period", reportingTime),
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

func SetVisitNumberInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Visit != nil && order.Visit.VisitNumber != nil && *order.Visit.VisitNumber != "" {
		visit := struct {
			AccountNumber *string `json:"AccountNumber"`
			Location      *struct {
				Bed                   *string `json:"Bed"`
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			VisitDateTime *string `json:"VisitDateTime"`
			VisitNumber   *string `json:"VisitNumber"`
		}{
			VisitNumber: order.Visit.VisitNumber,
		}
		flowsheet.Visit = &visit
	}
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

// unitIntervalToPercent converts a unit interval (0.0 - 1.0) to a percentage (0.0 - 100.0)
func unitIntervalToPercent(val *float64) *float64 {
	if val == nil {
		return nil
	}

	res := *val * 100
	return &res
}

func bgInUnits(val float64, sourceUnits string, targetUnits string) (float64, string) {
	if strings.ToLower(sourceUnits) == "mmol/l" && strings.ToLower(targetUnits) == "mg/dl" {
		intValue := int(val*MmolLToMgdLConversionFactor*MmolLToMgdLPrecisionFactor + 0.5)
		floatValue := float64(intValue) / MmolLToMgdLPrecisionFactor
		return floatValue, targetUnits
	}

	return val, sourceUnits
}

package redox

import (
	"fmt"
	"strings"
	"time"

	"github.com/tidepool-org/clinic-worker/types"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
)

const (
	EventTypeNewFlowsheet = "New"
	DataModelFlowsheet    = "Flowsheet"

	MmolLToMgdLConversionFactor float64 = 18.01559
	MmolLToMgdLPrecisionFactor  float64 = 100000.0

	AdditionalIdentifierExtensionId string = "additional-identifier"
	AdditionalIdentifierURI         string = "https://api.redoxengine.com/extensions/additional-identifier"
	AdditionalIdentifierTypeOrderId string = "orderId"

	AdditionalProviderInfoExtensionId string = "additional-provider-info"
	AdditionalProviderInfoURI         string = "https://api.redoxengine.com/extensions/additional-provider-info"

	missingValue = "NOT AVAILABLE"
	days14       = 14 * 24 * time.Hour
	percentage   = "%"
	day          = "day"
	hour         = "hour"
)

type AdditionalIdentifierExtension struct {
	URL        string               `json:"url"`
	Identifier AdditionalIdentifier `json:"identifier"`
}

type AdditionalIdentifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type AdditionalProviderExtension struct {
	URL          string                          `json:"url"`
	Participants []AdditionalProviderParticipant `json:"participants"`
}

type AdditionalProviderParticipant struct {
	Id     string `json:"id"`
	IdType string `json:"idType"`
	Person struct {
		Name struct {
			Given  []string `json:"given"`
			Family string   `json:"family"`
		} `json:"name"`
	} `json:"person"`
}

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

	if stats != nil {
		reportingTime = formatTime(stats.Dates.LastUpdatedDate)
		if stats.Periods != nil {
			if v, ok := stats.Periods["14d"]; ok {
				period = &v
			}
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
	unitsDay := day
	unitsHour := hour

	var cgmUsePercent *float64
	var averageGlucose *float64
	var averageGlucoseUnits *string
	var gmi *float64
	var cgmStdDev *float64
	var cgmStdDevUnits *string
	var cgmCoeffVar *float64
	var cgmDaysWithData *int
	var cgmHoursWithData *int
	var timeInVeryLow *float64
	var timeInLow *float64
	var timeInTarget *float64
	var timeInHigh *float64
	var timeInVeryHigh *float64

	if period != nil {
		if period.AverageGlucoseMmol != nil {
			val := float64(*period.AverageGlucoseMmol)
			units := string(clinics.MmolL)

			// Convert blood glucose to preferred units
			val, units = bgInUnits(val, units, string(preferredBgUnits))

			averageGlucose = &val
			averageGlucoseUnits = &units
		}

		{
			// Convert standard deviation to preferred units
			val, units := bgInUnits(period.StandardDeviation, string(clinics.MmolL), string(preferredBgUnits))
			cgmStdDev = &val
			cgmStdDevUnits = &units
		}

		cgmUsePercent = period.TimeCGMUsePercent
		cgmCoeffVar = &period.CoefficientOfVariation
		cgmDaysWithData = &period.DaysWithData
		cgmHoursWithData = &period.HoursWithData
		gmi = period.GlucoseManagementIndicator
		timeInVeryLow = period.TimeInVeryLowPercent
		timeInLow = period.TimeInLowPercent
		timeInTarget = period.TimeInTargetPercent
		timeInHigh = period.TimeInHighPercent
		timeInVeryHigh = period.TimeInVeryHighPercent
	}

	AppendObservation(f, "REPORTING_PERIOD_START_CGM", formatTime(periodStart), "DateTime", nil, "CGM Reporting Period Start", reportingTime)
	AppendObservation(f, "REPORTING_PERIOD_END_CGM", formatTime(periodEnd), "DateTime", nil, "CGM Reporting Period End", reportingTime)
	AppendObservation(f, "REPORTING_PERIOD_START_CGM_DATA", formatTime(firstData), "DateTime", nil, "CGM Reporting Period Start Date of actual Data", reportingTime)

	AppendObservation(f, "ACTIVE_WEAR_TIME_CGM", formatFloat(unitIntervalToPercent(cgmUsePercent)), "Numeric", &unitsPercentage, "Percentage of time CGM worn during reporting period", reportingTime)
	AppendObservation(f, "AVERAGE_CGM", formatFloat(averageGlucose), "Numeric", averageGlucoseUnits, "CGM Average Glucose during reporting period", reportingTime)
	AppendObservation(f, "STANDARD_DEVIATION_CGM", formatFloat(cgmStdDev), "Numeric", cgmStdDevUnits, "The standard deviation of CGM measurements during the reporting period", reportingTime)
	AppendObservation(f, "COEFFICIENT_OF_VARIATION_CGM", formatFloat(cgmCoeffVar), "Numeric", nil, "The coefficient of variation (standard deviation * 100 / mean) of CGM measurements during the reporting period", reportingTime)
	AppendObservation(f, "DAYS_WITH_DATA_CGM", formatInt(cgmDaysWithData), "Numeric", &unitsDay, "Number of days with at least one CGM datum during the reporting period", reportingTime)
	AppendObservation(f, "HOURS_WITH_DATA_CGM", formatInt(cgmHoursWithData), "Numeric", &unitsHour, "Number of hours with at least one CGM datum during the reporting period", reportingTime)
	AppendObservation(f, "GLUCOSE_MANAGEMENT_INDICATOR", formatFloat(gmi), "Numeric", nil, "CGM Glucose Management Indicator during reporting period", reportingTime)

	AppendObservation(f, "TIME_BELOW_RANGE_VERY_LOW_CGM", formatFloat(unitIntervalToPercent(timeInVeryLow)), "Numeric", &unitsPercentage, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)", reportingTime)
	AppendObservation(f, "TIME_BELOW_RANGE_LOW_CGM", formatFloat(unitIntervalToPercent(timeInLow)), "Numeric", &unitsPercentage, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)", reportingTime)
	AppendObservation(f, "TIME_IN_RANGE_CGM", formatFloat(unitIntervalToPercent(timeInTarget)), "Numeric", &unitsPercentage, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)", reportingTime)
	AppendObservation(f, "TIME_ABOVE_RANGE_HIGH_CGM", formatFloat(unitIntervalToPercent(timeInHigh)), "Numeric", &unitsPercentage, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)", reportingTime)
	AppendObservation(f, "TIME_ABOVE_RANGE_VERY_HIGH_CGM", formatFloat(unitIntervalToPercent(timeInVeryHigh)), "Numeric", &unitsPercentage, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)", reportingTime)
}

func PopulateBGMObservations(stats *clinics.PatientBGMStats, preferredBgUnits clinics.ClinicPreferredBgUnits, f *models.NewFlowsheet) {
	now := time.Now()

	var period *clinics.PatientBGMPeriod
	periodDuration := days14
	reportingTime := formatTime(&now)

	var firstData, periodEnd, periodStart *time.Time
	if stats != nil {
		reportingTime = formatTime(stats.Dates.LastUpdatedDate)
		if stats.Periods != nil {
			if v, ok := stats.Periods["14d"]; ok {
				period = &v
			}
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
		if period.AverageGlucoseMmol != nil {
			val := float64(*period.AverageGlucoseMmol)
			units := string(clinics.MmolL)

			// Convert blood glucose to preferred units
			val, units = bgInUnits(val, units, string(preferredBgUnits))

			averageGlucose = &val
			averageGlucoseUnits = &units
		}
		averageDailyRecords = period.AverageDailyRecords

		timeInVeryLowRecords = period.TimeInVeryLowRecords
		timeInVeryHighRecords = period.TimeInVeryHighRecords
	}

	AppendObservation(f, "REPORTING_PERIOD_START_SMBG", formatTime(periodStart), "DateTime", nil, "SMBG Reporting Period Start", reportingTime)
	AppendObservation(f, "REPORTING_PERIOD_END_SMBG", formatTime(periodEnd), "DateTime", nil, "SMBG Reporting Period End", reportingTime)
	AppendObservation(f, "REPORTING_PERIOD_START_SMBG_DATA", formatTime(firstData), "DateTime", nil, "SMBG Reporting Period Start Date of actual Data", reportingTime)

	AppendObservation(f, "CHECK_RATE_READINGS_DAY_SMBG", formatFloat(averageDailyRecords), "Numeric", nil, "Average Numeric of SMBG readings per day during reporting period", reportingTime)
	AppendObservation(f, "AVERAGE_SMBG", formatFloat(averageGlucose), "Numeric", averageGlucoseUnits, "SMBG Average Glucose during reporting period", reportingTime)

	AppendObservation(f, "READINGS_BELOW_RANGE_VERY_LOW_SMBG", formatInt(timeInVeryLowRecords), "Numeric", nil, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period", reportingTime)
	AppendObservation(f, "READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", formatInt(timeInVeryHighRecords), "Numeric", nil, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period", reportingTime)
}

func AppendObservation(f *models.NewFlowsheet, code, value, valueType string, units *string, description, dateTime string) {
	observation := types.NewItemForSlice(f.Observations)
	observation.Code = code
	observation.Value = value
	observation.ValueType = valueType
	observation.Units = units
	observation.Description = &description
	observation.DateTime = dateTime
	f.Observations = append(f.Observations, observation)
}

func SetVisitNumberInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Visit != nil && order.Visit.VisitNumber != nil {
		if flowsheet.Visit == nil {
			flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
		}
		flowsheet.Visit.VisitNumber = order.Visit.VisitNumber
	}
}

func SetVisitLocationInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Visit == nil {
		return
	}

	if flowsheet.Visit == nil {
		flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
	}
	flowsheet.Visit.Location = order.Visit.Location
}

func SetAccountNumberInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Visit != nil && order.Visit.AccountNumber != nil {
		if flowsheet.Visit == nil {
			flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
		}
		flowsheet.Visit.AccountNumber = order.Visit.AccountNumber
	}
}

func SetOrderIdInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Order.ID == "" {
		return
	}

	initVisitExtensions(flowsheet)
	(*flowsheet.Visit.Extensions)[AdditionalIdentifierExtensionId] = AdditionalIdentifierExtension{
		URL: AdditionalIdentifierURI,
		Identifier: AdditionalIdentifier{
			Type:  AdditionalIdentifierTypeOrderId,
			Value: order.Order.ID,
		},
	}
}

func SetProviderInFlowsheet(order models.NewOrder, flowsheet *models.NewFlowsheet) {
	if order.Order.Provider == nil {
		return
	}
	if order.Order.Provider.ID == nil {
		return
	}
	if order.Order.Provider.FirstName == nil && order.Order.Provider.LastName == nil {
		return
	}

	participant := AdditionalProviderParticipant{
		Id: *order.Order.Provider.ID,
	}

	if order.Order.Provider.IDType != nil {
		participant.IdType = *order.Order.Provider.IDType
	}
	if order.Order.Provider.FirstName != nil {
		participant.Person.Name.Given = []string{*order.Order.Provider.FirstName}
	}
	if order.Order.Provider.LastName != nil {
		participant.Person.Name.Family = *order.Order.Provider.LastName
	}

	initVisitExtensions(flowsheet)
	(*flowsheet.Visit.Extensions)[AdditionalProviderInfoExtensionId] = AdditionalProviderExtension{
		URL:          AdditionalProviderInfoURI,
		Participants: []AdditionalProviderParticipant{participant},
	}
}

func initVisitExtensions(flowsheet *models.NewFlowsheet) {
	if flowsheet.Visit == nil {
		flowsheet.Visit = types.NewStructPtr(flowsheet.Visit)
	}
	if flowsheet.Visit.Extensions == nil {
		extensions := make(map[string]any)
		flowsheet.Visit.Extensions = &extensions
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

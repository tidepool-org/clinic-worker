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

type FlowsheetSettings struct {
	PreferredBGUnits string
	ICode            bool
}

type Observation struct {
	Code        string
	Value       string
	ValueType   string
	Units       *string
	DateTime    string
	Description string
}

// PopulateSummaryStatistics populates a flowsheet with patient summary statistics. If summary statistics are not available,
// the flowsheet items will be populated with 'NOT AVAILABLE'.
func PopulateSummaryStatistics(patient clinics.PatientV1, settings FlowsheetSettings, flowsheet *models.NewFlowsheet) {
	var cgmStats *clinics.CgmStatsV1
	var bgmStats *clinics.BgmStatsV1
	if patient.Summary != nil {
		cgmStats = patient.Summary.CgmStats
		bgmStats = patient.Summary.BgmStats
	}

	PopulateCGMObservations(cgmStats, settings, flowsheet)
	PopulateBGMObservations(bgmStats, settings, flowsheet)
}

func PopulateCGMObservations(stats *clinics.CgmStatsV1, settings FlowsheetSettings, f *models.NewFlowsheet) {
	now := time.Now()

	var period *clinics.CgmPeriodV1
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
	sourceGlucoseUnits := string(clinics.MmolL)
	destGlucoseUnits := settings.PreferredBGUnits

	var cgmUsePercent *float64
	var averageGlucose *float64
	var gmi *float64
	var cgmStdDev *float64
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
			var val float64
			// Convert blood glucose to preferred units, store unit result overriding preference if unit is not convertable
			val, destGlucoseUnits = bgInUnits(float64(*period.AverageGlucoseMmol), sourceGlucoseUnits, destGlucoseUnits)
			averageGlucose = &val
		}

		{ // scope to contain val / units to Ptr
			// Convert standard deviation to preferred units
			val, _ := bgInUnits(period.StandardDeviation, sourceGlucoseUnits, destGlucoseUnits)
			cgmStdDev = &val
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

	observations := map[string]*Observation{
		"REPORTING_PERIOD_START_CGM":      {"REPORTING_PERIOD_START_CGM", formatTime(periodStart), "DateTime", nil, reportingTime, "CGM Reporting Period Start"},
		"REPORTING_PERIOD_END_CGM":        {"REPORTING_PERIOD_END_CGM", formatTime(periodEnd), "DateTime", nil, reportingTime, "CGM Reporting Period End"},
		"REPORTING_PERIOD_START_CGM_DATA": {"REPORTING_PERIOD_START_CGM_DATA", formatTime(firstData), "DateTime", nil, reportingTime, "CGM Reporting Period Start Date of actual Data"},
		"TIME_ABOVE_RANGE_VERY_HIGH_CGM":  {"TIME_ABOVE_RANGE_VERY_HIGH_CGM", formatFloat(unitIntervalToPercent(timeInVeryHigh)), "Numeric", &unitsPercentage, reportingTime, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)"},
		"TIME_ABOVE_RANGE_HIGH_CGM":       {"TIME_ABOVE_RANGE_HIGH_CGM", formatFloat(unitIntervalToPercent(timeInHigh)), "Numeric", &unitsPercentage, reportingTime, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)"},
		"TIME_IN_RANGE_CGM":               {"TIME_IN_RANGE_CGM", formatFloat(unitIntervalToPercent(timeInTarget)), "Numeric", &unitsPercentage, reportingTime, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)"},
		"TIME_BELOW_RANGE_LOW_CGM":        {"TIME_BELOW_RANGE_LOW_CGM", formatFloat(unitIntervalToPercent(timeInLow)), "Numeric", &unitsPercentage, reportingTime, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)"},
		"TIME_BELOW_RANGE_VERY_LOW_CGM":   {"TIME_BELOW_RANGE_VERY_LOW_CGM", formatFloat(unitIntervalToPercent(timeInVeryLow)), "Numeric", &unitsPercentage, reportingTime, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)"},
		"GLUCOSE_MANAGEMENT_INDICATOR":    {"GLUCOSE_MANAGEMENT_INDICATOR", formatFloat(gmi), "Numeric", nil, reportingTime, "CGM Glucose Management Indicator during reporting period"},
		"AVERAGE_CGM":                     {"AVERAGE_CGM", formatFloat(averageGlucose), "Numeric", &destGlucoseUnits, reportingTime, "CGM Average Glucose during reporting period"},
		"STANDARD_DEVIATION_CGM":          {"STANDARD_DEVIATION_CGM", formatFloat(cgmStdDev), "Numeric", &destGlucoseUnits, reportingTime, "The standard deviation of CGM measurements during the reporting period"},
		"COEFFICIENT_OF_VARIATION_CGM":    {"COEFFICIENT_OF_VARIATION_CGM", formatFloat(cgmCoeffVar), "Numeric", nil, reportingTime, "The coefficient of variation (standard deviation * 100 / mean) of CGM measurements during the reporting period"},
		"ACTIVE_WEAR_TIME_CGM":            {"ACTIVE_WEAR_TIME_CGM", formatFloat(unitIntervalToPercent(cgmUsePercent)), "Numeric", &unitsPercentage, reportingTime, "Percentage of time CGM worn during reporting period"},
		"DAYS_WITH_DATA_CGM":              {"DAYS_WITH_DATA_CGM", formatInt(cgmDaysWithData), "Numeric", &unitsDay, reportingTime, "Number of days with at least one CGM datum during the reporting period"},
		"HOURS_WITH_DATA_CGM":             {"HOURS_WITH_DATA_CGM", formatInt(cgmHoursWithData), "Numeric", &unitsHour, reportingTime, "Number of hours with at least one CGM datum during the reporting period"},
	}

	// For clinics flagged as icode, replace certain values with alternative formatting, as defined in BACK-3476
	if settings.ICode {
		observations["COEFFICIENT_OF_VARIATION_CGM"].Value = formatFloatWithPrecision(unitIntervalToPercent(cgmCoeffVar), 1)
		observations["COEFFICIENT_OF_VARIATION_CGM"].Units = &unitsPercentage

		// ICode2 defines whole-number precision for average glucose, this is only accurate enough for mg/dl
		if strings.ToLower(settings.PreferredBGUnits) == "mg/dl" {
			observations["AVERAGE_CGM"].Value = formatFloatConditionalPrecision(averageGlucose)
		} else {
			observations["AVERAGE_CGM"].Value = formatFloatWithPrecision(averageGlucose, 1)
		}

		observations["GLUCOSE_MANAGEMENT_INDICATOR"].Value = formatFloatWithPrecision(gmi, 1)
		observations["ACTIVE_WEAR_TIME_CGM"].Value = formatFloatWithPrecision(unitIntervalToPercent(cgmUsePercent), 2)
		observations["STANDARD_DEVIATION_CGM"].Value = formatFloatWithPrecision(cgmStdDev, 1)
		observations["TIME_BELOW_RANGE_VERY_LOW_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryLow))
		observations["TIME_BELOW_RANGE_LOW_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInLow))
		observations["TIME_IN_RANGE_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInTarget))
		observations["TIME_ABOVE_RANGE_HIGH_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInHigh))
		observations["TIME_ABOVE_RANGE_VERY_HIGH_CGM"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryHigh))
	}

	observationsOrder := []string{
		"REPORTING_PERIOD_START_CGM",
		"REPORTING_PERIOD_END_CGM",
		"REPORTING_PERIOD_START_CGM_DATA",
		"TIME_ABOVE_RANGE_VERY_HIGH_CGM",
		"TIME_ABOVE_RANGE_HIGH_CGM",
		"TIME_IN_RANGE_CGM",
		"TIME_BELOW_RANGE_LOW_CGM",
		"TIME_BELOW_RANGE_VERY_LOW_CGM",
		"GLUCOSE_MANAGEMENT_INDICATOR",
		"AVERAGE_CGM",
		"STANDARD_DEVIATION_CGM",
		"COEFFICIENT_OF_VARIATION_CGM",
		"ACTIVE_WEAR_TIME_CGM",
		"DAYS_WITH_DATA_CGM",
		"HOURS_WITH_DATA_CGM",
	}

	for _, key := range observationsOrder {
		observation := observations[key]
		if observation.Value != missingValue {
			AppendObservation(f, observation)
		}
	}
}

func PopulateBGMObservations(stats *clinics.BgmStatsV1, settings FlowsheetSettings, f *models.NewFlowsheet) {
	now := time.Now()

	var period *clinics.BgmPeriodV1
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

	unitsDay := day
	unitsPercentage := percentage
	sourceGlucoseUnits := string(clinics.MmolL)
	destGlucoseUnits := settings.PreferredBGUnits

	var averageDailyRecords *float64
	var averageGlucose *float64
	var timeInVeryLowRecords *int
	var timeInVeryHighRecords *int
	var timeInVeryLowPercent *float64
	var timeInLowPercent *float64
	var timeInTargetPercent *float64
	var timeInHighPercent *float64
	var timeInVeryHighPercent *float64
	var bgmStdDev *float64
	var bgmCoeffVar *float64
	var bgmDaysWithData *int
	var bgmTotalRecords *int
	var minGlucose *float64
	var maxGlucose *float64

	if period != nil {
		if period.AverageGlucoseMmol != nil {
			// Convert blood glucose to preferred units, store unit result overriding preference if unit is not convertable
			averageGlucoseVal, units := bgInUnits(float64(*period.AverageGlucoseMmol), sourceGlucoseUnits, destGlucoseUnits)
			averageGlucose = &averageGlucoseVal
			destGlucoseUnits = units
		}

		// Convert standard deviation to preferred units
		bgmStdDevVal, _ := bgInUnits(period.StandardDeviation, sourceGlucoseUnits, destGlucoseUnits)
		bgmStdDev = &bgmStdDevVal

		// Convert min glucose to preferred units
		minGlucoseVal, _ := bgInUnits(period.Min, sourceGlucoseUnits, destGlucoseUnits)
		minGlucose = &minGlucoseVal

		// Convert max glucose to preferred units
		maxGlucoseVal, _ := bgInUnits(period.Max, sourceGlucoseUnits, destGlucoseUnits)
		maxGlucose = &maxGlucoseVal

		averageDailyRecords = period.AverageDailyRecords
		timeInVeryLowRecords = period.TimeInVeryLowRecords
		timeInVeryHighRecords = period.TimeInVeryHighRecords
		bgmCoeffVar = &period.CoefficientOfVariation
		bgmDaysWithData = &period.DaysWithData
		bgmTotalRecords = period.TotalRecords
		timeInVeryLowPercent = period.TimeInVeryLowPercent
		timeInLowPercent = period.TimeInLowPercent
		timeInTargetPercent = period.TimeInTargetPercent
		timeInHighPercent = period.TimeInHighPercent
		timeInVeryHighPercent = period.TimeInVeryHighPercent
	}

	observations := map[string]*Observation{
		"REPORTING_PERIOD_START_SMBG":         {"REPORTING_PERIOD_START_SMBG", formatTime(periodStart), "DateTime", nil, reportingTime, "SMBG Reporting Period Start"},
		"REPORTING_PERIOD_END_SMBG":           {"REPORTING_PERIOD_END_SMBG", formatTime(periodEnd), "DateTime", nil, reportingTime, "SMBG Reporting Period End"},
		"REPORTING_PERIOD_START_SMBG_DATA":    {"REPORTING_PERIOD_START_SMBG_DATA", formatTime(firstData), "DateTime", nil, reportingTime, "SMBG Reporting Period Start Date of actual Data"},
		"TIME_ABOVE_RANGE_VERY_HIGH_SMBG":     {"TIME_ABOVE_RANGE_VERY_HIGH_SMBG", formatFloat(unitIntervalToPercent(timeInVeryHighPercent)), "Numeric", &unitsPercentage, reportingTime, "% of readings > 250 mg/dL (>13.9 mmol/L)"},
		"TIME_ABOVE_RANGE_HIGH_SMBG":          {"TIME_ABOVE_RANGE_HIGH_SMBG", formatFloat(unitIntervalToPercent(timeInHighPercent)), "Numeric", &unitsPercentage, reportingTime, "% of readings between 181–250 mg/dL (10.1–13.9 mmol/L)"},
		"TIME_IN_RANGE_SMBG":                  {"TIME_IN_RANGE_SMBG", formatFloat(unitIntervalToPercent(timeInTargetPercent)), "Numeric", &unitsPercentage, reportingTime, "% of readings between 70–180 mg/dL (3.9–10.0 mmol/L)"},
		"TIME_BELOW_RANGE_LOW_SMBG":           {"TIME_BELOW_RANGE_LOW_SMBG", formatFloat(unitIntervalToPercent(timeInLowPercent)), "Numeric", &unitsPercentage, reportingTime, "% of readings between 54–69 mg/dL (3.0–3.8 mmol/L)"},
		"TIME_BELOW_RANGE_VERY_LOW_SMBG":      {"TIME_BELOW_RANGE_VERY_LOW_SMBG", formatFloat(unitIntervalToPercent(timeInVeryLowPercent)), "Numeric", &unitsPercentage, reportingTime, "% of readings < 54 mg/dL (<3.0 mmol/L)"},
		"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG": {"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", formatInt(timeInVeryHighRecords), "Numeric", nil, reportingTime, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period"},
		"READINGS_BELOW_RANGE_VERY_LOW_SMBG":  {"READINGS_BELOW_RANGE_VERY_LOW_SMBG", formatInt(timeInVeryLowRecords), "Numeric", nil, reportingTime, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period"},
		"MAX_SMBG":                            {"MAX_SMBG", formatFloat(maxGlucose), "Numeric", &destGlucoseUnits, reportingTime, "Maximum blood glucose reading over the time period"},
		"MIN_SMBG":                            {"MIN_SMBG", formatFloat(minGlucose), "Numeric", &destGlucoseUnits, reportingTime, "Minimum blood glucose reading over the time period"},
		"AVERAGE_SMBG":                        {"AVERAGE_SMBG", formatFloat(averageGlucose), "Numeric", &destGlucoseUnits, reportingTime, "SMBG Average Glucose during reporting period"},
		"STANDARD_DEVIATION_SMBG":             {"STANDARD_DEVIATION_SMBG", formatFloat(bgmStdDev), "Numeric", &destGlucoseUnits, reportingTime, "The standard deviation of SMBG measurements during the reporting period"},
		"COEFFICIENT_OF_VARIATION_SMBG":       {"COEFFICIENT_OF_VARIATION_SMBG", formatFloat(bgmCoeffVar), "Numeric", nil, reportingTime, "The coefficient of variation (standard deviation * 100 / mean) of SMBG measurements during the reporting period"},
		"TOTAL_READING_COUNT_SMBG":            {"TOTAL_READING_COUNT_SMBG", formatInt(bgmTotalRecords), "Numeric", nil, reportingTime, "The total number of SMBG readings taken during the SMBG Reporting Period"},
		"CHECK_RATE_READINGS_DAY_SMBG":        {"CHECK_RATE_READINGS_DAY_SMBG", formatFloat(averageDailyRecords), "Numeric", nil, reportingTime, "Average Numeric of SMBG readings per day during reporting period"},
		"DAYS_WITH_DATA_SMBG":                 {"DAYS_WITH_DATA_SMBG", formatInt(bgmDaysWithData), "Numeric", &unitsDay, reportingTime, "The total number of days with at least 1 SMBG reading over the reporting period"},
	}

	// For clinics flagged as icode, replace certain values with alternative formatting, as defined in BACK-3476
	if settings.ICode {
		observations["COEFFICIENT_OF_VARIATION_SMBG"].Value = formatFloatWithPrecision(unitIntervalToPercent(bgmCoeffVar), 1)
		observations["COEFFICIENT_OF_VARIATION_SMBG"].Units = &unitsPercentage

		// ICode2 defines whole-number precision for glucose, this is only accurate enough for mg/dl
		if strings.ToLower(settings.PreferredBGUnits) == "mg/dl" {
			observations["AVERAGE_SMBG"].Value = formatFloatConditionalPrecision(averageGlucose)
			observations["MIN_SMBG"].Value = formatFloatConditionalPrecision(minGlucose)
			observations["MAX_SMBG"].Value = formatFloatConditionalPrecision(maxGlucose)
		} else {
			observations["AVERAGE_SMBG"].Value = formatFloatWithPrecision(averageGlucose, 1)
			observations["MIN_SMBG"].Value = formatFloatWithPrecision(minGlucose, 1)
			observations["MAX_SMBG"].Value = formatFloatWithPrecision(maxGlucose, 1)
		}

		observations["STANDARD_DEVIATION_SMBG"].Value = formatFloatWithPrecision(bgmStdDev, 1)
		observations["TIME_BELOW_RANGE_VERY_LOW_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryLowPercent))
		observations["TIME_BELOW_RANGE_LOW_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInLowPercent))
		observations["TIME_IN_RANGE_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInTargetPercent))
		observations["TIME_ABOVE_RANGE_HIGH_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInHighPercent))
		observations["TIME_ABOVE_RANGE_VERY_HIGH_SMBG"].Value = formatFloatConditionalPrecision(unitIntervalToPercent(timeInVeryHighPercent))
	}

	observationOrder := []string{
		"REPORTING_PERIOD_START_SMBG",
		"REPORTING_PERIOD_END_SMBG",
		"REPORTING_PERIOD_START_SMBG_DATA",
		"TIME_ABOVE_RANGE_VERY_HIGH_SMBG",
		"TIME_ABOVE_RANGE_HIGH_SMBG",
		"TIME_IN_RANGE_SMBG",
		"TIME_BELOW_RANGE_LOW_SMBG",
		"TIME_BELOW_RANGE_VERY_LOW_SMBG",
		"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG",
		"READINGS_BELOW_RANGE_VERY_LOW_SMBG",
		"MAX_SMBG",
		"MIN_SMBG",
		"AVERAGE_SMBG",
		"STANDARD_DEVIATION_SMBG",
		"COEFFICIENT_OF_VARIATION_SMBG",
		"TOTAL_READING_COUNT_SMBG",
		"CHECK_RATE_READINGS_DAY_SMBG",
		"DAYS_WITH_DATA_SMBG",
	}

	for _, key := range observationOrder {
		observation := observations[key]
		if observation.Value != missingValue {
			AppendObservation(f, observation)
		}
	}
}

func AppendObservation(f *models.NewFlowsheet, o *Observation) {
	observation := types.NewItemForSlice(f.Observations)
	observation.Code = o.Code
	observation.Value = o.Value
	observation.ValueType = o.ValueType
	observation.Units = o.Units
	observation.Description = &o.Description
	observation.DateTime = o.DateTime
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
	if order.Order.ID != "" {
		extensions := map[string]any{
			AdditionalIdentifierExtensionId: AdditionalIdentifierExtension{
				URL: AdditionalIdentifierURI,
				Identifier: AdditionalIdentifier{
					Type:  AdditionalIdentifierTypeOrderId,
					Value: order.Order.ID,
				},
			}}
		flowsheet.Visit.Extensions = &extensions
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
	return formatFloatWithPrecision(val, 4)
}

func formatFloatWithPrecision(val *float64, decimalPlaces int) string {
	if val == nil {
		return missingValue
	}
	tpl := fmt.Sprintf("%%.%df", decimalPlaces)
	return fmt.Sprintf(tpl, *val)
}

// unitIntervalToPercent converts a unit interval (0.0 - 1.0) to a percentage (0.0 - 100.0)
func unitIntervalToPercent(val *float64) *float64 {
	if val == nil {
		return nil
	}

	res := *val * 100
	return &res
}

// wholeOrSingleDecimal conditionally removes the decimal only if the number is <1.
func formatFloatConditionalPrecision(val *float64) string {
	if val == nil {
		return missingValue
	}

	if *val < 1 {
		return formatFloatWithPrecision(val, 1)
	}
	return formatFloatWithPrecision(val, 0)
}

func bgInUnits(val float64, sourceUnits string, targetUnits string) (float64, string) {
	if strings.ToLower(sourceUnits) == "mmol/l" && strings.ToLower(targetUnits) == "mg/dl" {
		intValue := int(val*MmolLToMgdLConversionFactor*MmolLToMgdLPrecisionFactor + 0.5)
		floatValue := float64(intValue) / MmolLToMgdLPrecisionFactor
		return floatValue, targetUnits
	}

	return val, sourceUnits
}

package redox_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/tidepool-org/clinic-worker/test"
	api "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"

	//. "github.com/onsi/gomega/gstruct"
	"time"

	"github.com/tidepool-org/clinic-worker/redox"
)

var _ = Describe("Flowsheet", func() {
	Describe("NewFlowsheet", func() {
		It("returns a correctly instantiated flowsheet", func() {
			flowsheet := redox.NewFlowsheet()
			Expect(flowsheet.Meta.DataModel).To(Equal("Flowsheet"))
			Expect(flowsheet.Meta.EventType).To(Equal("New"))
			Expect(flowsheet.Meta.EventDateTime).ToNot(BeNil())

			eventDateTime, err := time.Parse(time.RFC3339, *flowsheet.Meta.EventDateTime)
			Expect(err).ToNot(HaveOccurred())
			Expect(eventDateTime).To(BeTemporally("~", time.Now(), 3*time.Second))
		})
	})

	Describe("SetVisitNumberInFlowsheet", func() {
		var flowsheet models.NewFlowsheet
		var order models.NewOrder

		BeforeEach(func() {
			flowsheet = redox.NewFlowsheet()
			fixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(fixture, &order)).To(Succeed())
		})

		It("sets the visit number from the order", func() {
			redox.SetVisitNumberInFlowsheet(order, &flowsheet)
			Expect(flowsheet.Visit).ToNot(BeNil())
			Expect(flowsheet.Visit.VisitNumber).To(PointTo(Equal(*order.Visit.VisitNumber)))
		})

		Describe("SetVisitLocationFromOrder", func() {
			It("sets the visit location from the order", func() {
				redox.SetVisitLocationInFlowsheet(order, &flowsheet)
				Expect(flowsheet.Visit).ToNot(BeNil())
				Expect(flowsheet.Visit.Location).To(PointTo(Equal(*order.Visit.Location)))
			})
		})

		Describe("SetVisitLocationFromOrder", func() {
			It("sets the visit location from the order", func() {
				expectedProviderExtension := Fields{
					"URL": Equal("https://api.redoxengine.com/extensions/additional-provider-info"),
					"Participants": ContainElements(MatchFields(IgnoreExtras, Fields{
						"Id":     Equal("4356789876"),
						"IdType": Equal("NPI"),
						"Person": MatchFields(IgnoreExtras, Fields{
							"Name": MatchFields(IgnoreExtras, Fields{
								"Given":  ConsistOf(Equal("Pat")),
								"Family": Equal("Granite"),
							}),
						}),
					})),
				}
				redox.SetProviderInFlowsheet(order, &flowsheet)
				Expect(flowsheet.Visit).ToNot(BeNil())
				Expect(flowsheet.Visit.Extensions).To(PointTo(HaveKeyWithValue("additional-provider-info", MatchFields(IgnoreExtras, expectedProviderExtension))))
			})
		})
	})

	Context("With EHR Match Response", func() {
		var response api.EhrMatchResponseV1

		BeforeEach(func() {
			response = api.EhrMatchResponseV1{}
			fixture, err := test.LoadFixture("test/fixtures/subscriptionmatchresponse.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(fixture, &response)).To(Succeed())
			Expect(response.Patients).ToNot(BeNil())
			Expect(response.Patients).To(PointTo(HaveLen(1)))
		})

		Describe("PopulateSummaryStatistics", func() {
			It("populates cgm and bgm observations with icode unset", func() {
				expectedPercentageUnits := "%"
				expectedBgUnits := "mmol/L"
				expectedDayUnits := "day"
				expetedHourUnits := "hour"

				flowsheet := redox.NewFlowsheet()
				settings := redox.FlowsheetSettings{
					PreferredBGUnits: string(response.Clinic.PreferredBgUnits),
					ICode:            false,
				}
				redox.PopulateSummaryStatistics((*response.Patients)[0], settings, &flowsheet)

				Expect(flowsheet.Observations).To(HaveExactElements(
					MatchObservation(Observation{"REPORTING_PERIOD_START_CGM", "2023-04-09T17:44:09Z", "DateTime", nil, "CGM Reporting Period Start"}),
					MatchObservation(Observation{"REPORTING_PERIOD_END_CGM", "2023-04-23T17:44:09Z", "DateTime", nil, "CGM Reporting Period End"}),
					MatchObservation(Observation{"REPORTING_PERIOD_START_CGM_DATA", "2023-04-14T00:00:00Z", "DateTime", nil, "CGM Reporting Period Start Date of actual Data"}),
					MatchObservation(Observation{"TIME_ABOVE_RANGE_VERY_HIGH_CGM", "4.4059", "Numeric", &expectedPercentageUnits, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)"}),
					MatchObservation(Observation{"TIME_ABOVE_RANGE_HIGH_CGM", "25.6436", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)"}),
					MatchObservation(Observation{"TIME_IN_RANGE_CGM", "56.2871", "Numeric", &expectedPercentageUnits, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)"}),
					MatchObservation(Observation{"TIME_BELOW_RANGE_LOW_CGM", "8.6139", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)"}),
					MatchObservation(Observation{"TIME_BELOW_RANGE_VERY_LOW_CGM", "5.0495", "Numeric", &expectedPercentageUnits, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)"}),
					MatchObservation(Observation{"AVERAGE_CGM", "7.9212", "Numeric", &expectedBgUnits, "CGM Average Glucose during reporting period"}),
					MatchObservation(Observation{"STANDARD_DEVIATION_CGM", "1.4697", "Numeric", &expectedBgUnits, "The standard deviation of CGM measurements during the reporting period"}),
					MatchObservation(Observation{"COEFFICIENT_OF_VARIATION_CGM", "0.2004", "Numeric", nil, "The coefficient of variation (standard deviation * 100 / mean) of CGM measurements during the reporting period"}),
					MatchObservation(Observation{"ACTIVE_WEAR_TIME_CGM", "50.1262", "Numeric", &expectedPercentageUnits, "Percentage of time CGM worn during reporting period"}),
					MatchObservation(Observation{"DAYS_WITH_DATA_CGM", "2", "Numeric", &expectedDayUnits, "Number of days with at least one CGM datum during the reporting period"}),
					MatchObservation(Observation{"HOURS_WITH_DATA_CGM", "28", "Numeric", &expetedHourUnits, "Number of hours with at least one CGM datum during the reporting period"}),
					MatchObservation(Observation{"REPORTING_PERIOD_START_SMBG", "2023-04-11T00:57:11Z", "DateTime", nil, "SMBG Reporting Period Start"}),
					MatchObservation(Observation{"REPORTING_PERIOD_END_SMBG", "2023-04-25T00:57:11Z", "DateTime", nil, "SMBG Reporting Period End"}),
					MatchObservation(Observation{"REPORTING_PERIOD_START_SMBG_DATA", "2023-04-11T00:57:11Z", "DateTime", nil, "SMBG Reporting Period Start Date of actual Data"}),
					MatchObservation(Observation{"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "13", "Numeric", nil, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period"}),
					MatchObservation(Observation{"READINGS_BELOW_RANGE_VERY_LOW_SMBG", "4", "Numeric", nil, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period"}),
					MatchObservation(Observation{"AVERAGE_SMBG", "9.5634", "Numeric", &expectedBgUnits, "SMBG Average Glucose during reporting period"}),
					MatchObservation(Observation{"CHECK_RATE_READINGS_DAY_SMBG", "4.9286", "Numeric", nil, "Average Numeric of SMBG readings per day during reporting period"}),
				))
			})

			It("populates cgm and bgm observations with icode set", func() {
				expectedPercentageUnits := "%"
				expectedBgUnits := "mmol/L"
				expectedDayUnits := "day"
				expetedHourUnits := "hour"

				flowsheet := redox.NewFlowsheet()
				settings := redox.FlowsheetSettings{
					PreferredBGUnits: string(response.Clinic.PreferredBgUnits),
					ICode:            true,
				}
				redox.PopulateSummaryStatistics((*response.Patients)[0], settings, &flowsheet)

				Expect(flowsheet.Observations).To(HaveExactElements(
					MatchObservation(Observation{"REPORTING_PERIOD_START_CGM", "2023-04-09T17:44:09Z", "DateTime", nil, "CGM Reporting Period Start"}),
					MatchObservation(Observation{"REPORTING_PERIOD_END_CGM", "2023-04-23T17:44:09Z", "DateTime", nil, "CGM Reporting Period End"}),
					MatchObservation(Observation{"REPORTING_PERIOD_START_CGM_DATA", "2023-04-14T00:00:00Z", "DateTime", nil, "CGM Reporting Period Start Date of actual Data"}),
					MatchObservation(Observation{"TIME_ABOVE_RANGE_VERY_HIGH_CGM", "4", "Numeric", &expectedPercentageUnits, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)"}),
					MatchObservation(Observation{"TIME_ABOVE_RANGE_HIGH_CGM", "26", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)"}),
					MatchObservation(Observation{"TIME_IN_RANGE_CGM", "56", "Numeric", &expectedPercentageUnits, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)"}),
					MatchObservation(Observation{"TIME_BELOW_RANGE_LOW_CGM", "9", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)"}),
					MatchObservation(Observation{"TIME_BELOW_RANGE_VERY_LOW_CGM", "5", "Numeric", &expectedPercentageUnits, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)"}),
					MatchObservation(Observation{"AVERAGE_CGM", "7.9", "Numeric", &expectedBgUnits, "CGM Average Glucose during reporting period"}),
					MatchObservation(Observation{"STANDARD_DEVIATION_CGM", "1.5", "Numeric", &expectedBgUnits, "The standard deviation of CGM measurements during the reporting period"}),
					MatchObservation(Observation{"COEFFICIENT_OF_VARIATION_CGM", "20.0", "Numeric", &expectedPercentageUnits, "The coefficient of variation (standard deviation * 100 / mean) of CGM measurements during the reporting period"}),
					MatchObservation(Observation{"ACTIVE_WEAR_TIME_CGM", "50.13", "Numeric", &expectedPercentageUnits, "Percentage of time CGM worn during reporting period"}),
					MatchObservation(Observation{"DAYS_WITH_DATA_CGM", "2", "Numeric", &expectedDayUnits, "Number of days with at least one CGM datum during the reporting period"}),
					MatchObservation(Observation{"HOURS_WITH_DATA_CGM", "28", "Numeric", &expetedHourUnits, "Number of hours with at least one CGM datum during the reporting period"}),
					MatchObservation(Observation{"REPORTING_PERIOD_START_SMBG", "2023-04-11T00:57:11Z", "DateTime", nil, "SMBG Reporting Period Start"}),
					MatchObservation(Observation{"REPORTING_PERIOD_END_SMBG", "2023-04-25T00:57:11Z", "DateTime", nil, "SMBG Reporting Period End"}),
					MatchObservation(Observation{"REPORTING_PERIOD_START_SMBG_DATA", "2023-04-11T00:57:11Z", "DateTime", nil, "SMBG Reporting Period Start Date of actual Data"}),
					MatchObservation(Observation{"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "13", "Numeric", nil, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period"}),
					MatchObservation(Observation{"READINGS_BELOW_RANGE_VERY_LOW_SMBG", "4", "Numeric", nil, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period"}),
					MatchObservation(Observation{"AVERAGE_SMBG", "9.6", "Numeric", &expectedBgUnits, "SMBG Average Glucose during reporting period"}),
					MatchObservation(Observation{"CHECK_RATE_READINGS_DAY_SMBG", "4.9286", "Numeric", nil, "Average Numeric of SMBG readings per day during reporting period"}),
				))
			})

			It("does not populate cgm and bgm observations when summaries are empty", func() {
				flowsheet := redox.NewFlowsheet()
				patient := (*response.Patients)[0]
				patient.Summary.BgmStats = nil
				patient.Summary.CgmStats = nil

				settings := redox.FlowsheetSettings{
					PreferredBGUnits: string(response.Clinic.PreferredBgUnits),
					ICode:            true,
				}
				redox.PopulateSummaryStatistics(patient, settings, &flowsheet)

				observations := Observations(flowsheet)
				Expect(len(observations)).To(Equal(0))
			})

			It("does not populate cgm and bgm observations when summaries placeholders", func() {
				flowsheet := redox.NewFlowsheet()
				patient := (*response.Patients)[0]
				patient.Summary.BgmStats = &api.BgmStatsV1{Dates: api.SummaryDatesV1{LastUpdatedDate: &time.Time{}}}
				patient.Summary.CgmStats = &api.CgmStatsV1{Dates: api.SummaryDatesV1{LastUpdatedDate: &time.Time{}}}

				settings := redox.FlowsheetSettings{
					PreferredBGUnits: string(response.Clinic.PreferredBgUnits),
					ICode:            true,
				}
				redox.PopulateSummaryStatistics(patient, settings, &flowsheet)

				observations := Observations(flowsheet)
				Expect(len(observations)).To(Equal(0))
			})

			It("converts blood glucose units to mg/dL when set as preferred bg units icode set", func() {
				expectedBgUnits := "mg/dL"

				flowsheet := redox.NewFlowsheet()
				patient := (*response.Patients)[0]
				response.Clinic.PreferredBgUnits = api.MgdL

				settings := redox.FlowsheetSettings{
					PreferredBGUnits: string(response.Clinic.PreferredBgUnits),
					ICode:            true,
				}
				redox.PopulateSummaryStatistics(patient, settings, &flowsheet)

				observations := Observations(flowsheet)
				Expect(observations).To(ContainElement(MatchObservation(Observation{"AVERAGE_CGM", "143", "Numeric", &expectedBgUnits, "CGM Average Glucose during reporting period"})))
				Expect(observations).To(ContainElement(MatchObservation(Observation{"AVERAGE_SMBG", "172", "Numeric", &expectedBgUnits, "SMBG Average Glucose during reporting period"})))
				Expect(observations).To(ContainElement(MatchObservation(Observation{"STANDARD_DEVIATION_CGM", "26.5", "Numeric", &expectedBgUnits, "The standard deviation of CGM measurements during the reporting period"})))
			})

			It("converts blood glucose units to mg/dL when set as preferred bg units icode unset", func() {
				expectedBgUnits := "mg/dL"

				flowsheet := redox.NewFlowsheet()
				patient := (*response.Patients)[0]
				response.Clinic.PreferredBgUnits = api.MgdL

				settings := redox.FlowsheetSettings{
					PreferredBGUnits: string(response.Clinic.PreferredBgUnits),
					ICode:            false,
				}
				redox.PopulateSummaryStatistics(patient, settings, &flowsheet)

				observations := Observations(flowsheet)
				Expect(observations).To(ContainElement(MatchObservation(Observation{"AVERAGE_CGM", "142.7052", "Numeric", &expectedBgUnits, "CGM Average Glucose during reporting period"})))
				Expect(observations).To(ContainElement(MatchObservation(Observation{"AVERAGE_SMBG", "172.2908", "Numeric", &expectedBgUnits, "SMBG Average Glucose during reporting period"})))
				Expect(observations).To(ContainElement(MatchObservation(Observation{"STANDARD_DEVIATION_CGM", "26.4774", "Numeric", &expectedBgUnits, "The standard deviation of CGM measurements during the reporting period"})))
			})
		})

	})
})

type Observation struct {
	Code        string
	Value       string
	ValueType   string
	Units       *string
	Description string
}

func Observations(flowsheet models.NewFlowsheet) map[string]any {
	result := make(map[string]any)
	for _, observation := range flowsheet.Observations {
		result[observation.Code] = observation
	}
	return result
}

func MatchObservation(observation Observation) types.GomegaMatcher {
	fields := Fields{
		"Code":        Equal(observation.Code),
		"Value":       Equal(observation.Value),
		"ValueType":   Equal(observation.ValueType),
		"Description": PointTo(Equal(observation.Description)),
		"DateTime":    Not(BeEmpty()),
	}
	if observation.Units != nil {
		fields["Units"] = PointTo(Equal(*observation.Units))
	} else {
		fields["Units"] = BeNil()
	}
	return MatchFields(IgnoreExtras, fields)
}

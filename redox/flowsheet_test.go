package redox_test

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/tidepool-org/clinic-worker/test"
	api "github.com/tidepool-org/clinic/client"

	//. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic-worker/redox"
	"time"
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

	Context("With EHR Match Response", func() {
		var response api.EHRMatchResponse

		BeforeEach(func() {
			response = api.EHRMatchResponse{}
			fixture, err := test.LoadFixture("test/fixtures/ehrmatchresponse.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(fixture, &response)).To(Succeed())
			Expect(response.Patients).ToNot(BeNil())
			Expect(response.Patients).To(PointTo(HaveLen(1)))
		})

		Describe("PopulateSummaryStatistics", func() {
			It("populates cgm and bgm observations", func() {
				expectedPercentageUnits := "%"
				expectedBgUnits := "mmol/L"

				flowsheet := redox.NewFlowsheet()
				redox.PopulateSummaryStatistics((*response.Patients)[0], response.Clinic, &flowsheet)

				Expect(flowsheet.Observations).To(ContainObservations(
					Observation{"REPORTING_PERIOD_START_CGM", "2023-04-09T17:44:09Z", "DateTime", nil, "CGM Reporting Period Start"},
					Observation{"REPORTING_PERIOD_END_CGM", "2023-04-23T17:44:09Z", "DateTime", nil, "CGM Reporting Period End"},
					Observation{"REPORTING_PERIOD_START_CGM_DATA", "2023-04-14T00:00:00Z", "DateTime", nil, "CGM Reporting Period Start Date of actual Data"},
					Observation{"ACTIVE_WEAR_TIME_CGM", "50.1262", "Numeric", &expectedPercentageUnits, "Percentage of time CGM worn during reporting period"},
					Observation{"AVERAGE_CGM", "7.9212", "Numeric", &expectedBgUnits, "CGM Average Glucose during reporting period"},
					Observation{"GLUCOSE_MANAGEMENT_INDICATOR", "NOT AVAILABLE", "Numeric", nil, "CGM Glucose Management Indicator during reporting period"},
					Observation{"TIME_BELOW_RANGE_VERY_LOW_CGM", "5.0495", "Numeric", &expectedPercentageUnits, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)"},
					Observation{"TIME_BELOW_RANGE_LOW_CGM", "8.6139", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)"},
					Observation{"TIME_IN_RANGE_CGM", "56.2871", "Numeric", &expectedPercentageUnits, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)"},
					Observation{"TIME_ABOVE_RANGE_HIGH_CGM", "25.6436", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)"},
					Observation{"TIME_ABOVE_RANGE_VERY_HIGH_CGM", "4.4059", "Numeric", &expectedPercentageUnits, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)"},
					Observation{"REPORTING_PERIOD_START_SMBG", "2023-04-11T00:57:11Z", "DateTime", nil, "SMBG Reporting Period Start"},
					Observation{"REPORTING_PERIOD_END_SMBG", "2023-04-25T00:57:11Z", "DateTime", nil, "SMBG Reporting Period End"},
					Observation{"REPORTING_PERIOD_START_SMBG_DATA", "2023-04-11T00:57:11Z", "DateTime", nil, "SMBG Reporting Period Start Date of actual Data"},
					Observation{"CHECK_RATE_READINGS_DAY_SMBG", "4.9286", "Numeric", nil, "Average Numeric of SMBG readings per day during reporting period"},
					Observation{"AVERAGE_SMBG", "9.5634", "Numeric", &expectedBgUnits, "SMBG Average Glucose during reporting period"},
					Observation{"READINGS_BELOW_RANGE_VERY_LOW_SMBG", "4", "Numeric", nil, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period"},
					Observation{"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "13", "Numeric", nil, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period"},
				))
			})

			It("populates cgm and bgm observations with N/A when summaries are empty", func() {
				expectedPercentageUnits := "%"

				flowsheet := redox.NewFlowsheet()
				patient := (*response.Patients)[0]
				patient.Summary.BgmStats = nil
				patient.Summary.CgmStats = nil

				redox.PopulateSummaryStatistics(patient, response.Clinic, &flowsheet)

				Expect(flowsheet.Observations).To(ContainObservations(
					Observation{"REPORTING_PERIOD_START_CGM", "NOT AVAILABLE", "DateTime", nil, "CGM Reporting Period Start"},
					Observation{"REPORTING_PERIOD_END_CGM", "NOT AVAILABLE", "DateTime", nil, "CGM Reporting Period End"},
					Observation{"REPORTING_PERIOD_START_CGM_DATA", "NOT AVAILABLE", "DateTime", nil, "CGM Reporting Period Start Date of actual Data"},
					Observation{"ACTIVE_WEAR_TIME_CGM", "NOT AVAILABLE", "Numeric", &expectedPercentageUnits, "Percentage of time CGM worn during reporting period"},
					Observation{"AVERAGE_CGM", "NOT AVAILABLE", "Numeric", nil, "CGM Average Glucose during reporting period"},
					Observation{"GLUCOSE_MANAGEMENT_INDICATOR", "NOT AVAILABLE", "Numeric", nil, "CGM Glucose Management Indicator during reporting period"},
					Observation{"TIME_BELOW_RANGE_VERY_LOW_CGM", "NOT AVAILABLE", "Numeric", &expectedPercentageUnits, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)"},
					Observation{"TIME_BELOW_RANGE_LOW_CGM", "NOT AVAILABLE", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)"},
					Observation{"TIME_IN_RANGE_CGM", "NOT AVAILABLE", "Numeric", &expectedPercentageUnits, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)"},
					Observation{"TIME_ABOVE_RANGE_HIGH_CGM", "NOT AVAILABLE", "Numeric", &expectedPercentageUnits, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)"},
					Observation{"TIME_ABOVE_RANGE_VERY_HIGH_CGM", "NOT AVAILABLE", "Numeric", &expectedPercentageUnits, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)"},
					Observation{"REPORTING_PERIOD_START_SMBG", "NOT AVAILABLE", "DateTime", nil, "SMBG Reporting Period Start"},
					Observation{"REPORTING_PERIOD_END_SMBG", "NOT AVAILABLE", "DateTime", nil, "SMBG Reporting Period End"},
					Observation{"REPORTING_PERIOD_START_SMBG_DATA", "NOT AVAILABLE", "DateTime", nil, "SMBG Reporting Period Start Date of actual Data"},
					Observation{"CHECK_RATE_READINGS_DAY_SMBG", "NOT AVAILABLE", "Numeric", nil, "Average Numeric of SMBG readings per day during reporting period"},
					Observation{"AVERAGE_SMBG", "NOT AVAILABLE", "Numeric", nil, "SMBG Average Glucose during reporting period"},
					Observation{"READINGS_BELOW_RANGE_VERY_LOW_SMBG", "NOT AVAILABLE", "Numeric", nil, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period"},
					Observation{"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "NOT AVAILABLE", "Numeric", nil, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period"},
				))
			})

			It("converts blood glucose units to mg/dL when set as preferred bg units", func() {
				expectedBgUnits := "mg/dL"

				flowsheet := redox.NewFlowsheet()
				patient := (*response.Patients)[0]
				response.Clinic.PreferredBgUnits = api.ClinicPreferredBgUnitsMgdL

				redox.PopulateSummaryStatistics(patient, response.Clinic, &flowsheet)

				Expect(flowsheet.Observations).To(ContainObservations(
					Observation{"AVERAGE_CGM", "142.7052", "Numeric", &expectedBgUnits, "CGM Average Glucose during reporting period"},
					Observation{"AVERAGE_SMBG", "172.2908", "Numeric", &expectedBgUnits, "SMBG Average Glucose during reporting period"},
				))
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

func ContainObservations(observations ...Observation) types.GomegaMatcher {
	elements := make([]interface{}, len(observations))
	for i, observation := range observations {
		elements[i] = MatchObservation(observation)
	}
	return ContainElements(elements)
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

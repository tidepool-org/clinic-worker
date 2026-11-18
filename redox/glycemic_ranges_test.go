package redox_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clinics "github.com/tidepool-org/clinic/client"
	summaryTypes "github.com/tidepool-org/platform/summary/types"

	"github.com/tidepool-org/clinic-worker/redox"
)

var _ = Describe("Flowsheet glycemic range descriptions", func() {
	const cgmPrefix = "CGM Time in Range: Time in range (TIR): % of readings and time "

	// observe returns each flowsheet observation of a patient whose CGM and BGM summaries
	// were computed with the range sets identified by the keys, by observation code.
	observe := func(cgmKey, bgmKey string, ranges *clinics.GlycemicRangesV1,
		settings redox.FlowsheetSettings) map[string]*redox.Observation {

		GinkgoHelper()
		now := time.Now()
		dates := clinics.SummaryDatesV1{
			LastUpdatedDate: &now,
			FirstData:       &now,
			LastData:        &now,
		}
		percent, records := 0.2, 3
		cgmPeriod := clinics.CgmPeriodV1{
			TimeInVeryLowPercent:  &percent,
			TimeInLowPercent:      &percent,
			TimeInTargetPercent:   &percent,
			TimeInHighPercent:     &percent,
			TimeInVeryHighPercent: &percent,
		}
		bgmPeriod := clinics.BgmPeriodV1{
			TimeInVeryLowPercent:  &percent,
			TimeInLowPercent:      &percent,
			TimeInTargetPercent:   &percent,
			TimeInHighPercent:     &percent,
			TimeInVeryHighPercent: &percent,
			TimeInVeryLowRecords:  &records,
			TimeInVeryHighRecords: &records,
		}
		patient := clinics.PatientV1{
			GlycemicRanges: ranges,
			Summary: &clinics.PatientSummaryV1{
				CgmStats: &clinics.CgmStatsV1{
					Config:  clinics.SummaryConfigV1{GlycemicRangesKey: cgmKey},
					Dates:   dates,
					Periods: clinics.CgmPeriodsV1{"14d": cgmPeriod},
				},
				BgmStats: &clinics.BgmStatsV1{
					Config:  clinics.SummaryConfigV1{GlycemicRangesKey: bgmKey},
					Dates:   dates,
					Periods: clinics.BgmPeriodsV1{"14d": bgmPeriod},
				},
			},
		}
		flowsheet := redox.NewFlowsheet()
		observations := redox.PopulateSummaryStatistics(patient, settings, &flowsheet)
		Expect(observations).ToNot(BeEmpty())
		Expect(flowsheet.Observations).To(HaveLen(len(observations)))
		result := map[string]*redox.Observation{}
		for _, observation := range observations {
			Expect(result).ToNot(HaveKey(observation.Code))
			result[observation.Code] = observation
		}
		return result
	}

	mgdL := redox.FlowsheetSettings{PreferredBGUnits: "mg/dL"}
	mmolL := redox.FlowsheetSettings{PreferredBGUnits: "mmol/L"}

	// descriptions returns the description of each observation of a patient whose
	// summaries were computed with the range set identified by key, by observation code.
	descriptions := func(key string, ranges *clinics.GlycemicRangesV1) map[string]string {
		GinkgoHelper()
		result := map[string]string{}
		for code, observation := range observe(key, key, ranges, mgdL) {
			result[code] = observation.Description
		}
		return result
	}

	// values returns the value of each observation, by observation code.
	values := func(observations map[string]*redox.Observation) map[string]string {
		result := map[string]string{}
		for code, observation := range observations {
			result[code] = observation.Value
		}
		return result
	}

	thresholdCodes := []string{
		"GLUCOSE_THRESHOLD_VERY_HIGH",
		"GLUCOSE_THRESHOLD_HIGH",
		"GLUCOSE_THRESHOLD_LOW",
		"GLUCOSE_THRESHOLD_VERY_LOW",
	}

	DescribeTable("keeps the fixed descriptions for the standard ranges",
		func(key string) {
			got := descriptions(key, nil)
			Expect(got).To(HaveKeyWithValue("TIME_IN_RANGE_CGM",
				cgmPrefix+"70–180 mg/dL (3.9–10.0 mmol/L)"))
			Expect(got).To(HaveKeyWithValue("TIME_BELOW_RANGE_VERY_LOW_SMBG",
				"% of readings < 54 mg/dL (<3.0 mmol/L)"))
			Expect(got).To(HaveKeyWithValue("READINGS_ABOVE_RANGE_VERY_HIGH_SMBG",
				"SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) "+
					"time >250 mg/dL (>13.9 mmol/L) during reporting period"))
		},
		Entry("when the summary has no key", ""),
		Entry("when the summary has the standard key", "preset:adaStandard"),
	)

	It("describes a preset in both units and names it", func() {
		got := descriptions("preset:adaPregnancyType1", nil)
		const name = "; glycemic ranges: Pregnancy (Type 1)"

		Expect(got).To(HaveKeyWithValue("TIME_IN_RANGE_CGM",
			cgmPrefix+"63–140 mg/dL (3.5–7.8 mmol/L)"+name))
		Expect(got).To(HaveKeyWithValue("TIME_BELOW_RANGE_LOW_CGM",
			"CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings "+
				"and time 54–62 mg/dL (3.0–3.4 mmol/L)"+name))
		Expect(got).To(HaveKeyWithValue("TIME_BELOW_RANGE_VERY_LOW_CGM",
			"CGM Time in Level 2 Hypoglycemia: Time below range (TBR-VL): % of readings "+
				"and time <54 mg/dL (<3.0 mmol/L)"+name))
		Expect(got).To(HaveKeyWithValue("TIME_ABOVE_RANGE_HIGH_SMBG",
			"% of readings >140 mg/dL (>7.8 mmol/L)"+name))
		Expect(got).To(HaveKeyWithValue("TIME_ABOVE_RANGE_VERY_HIGH_SMBG",
			"% of readings in the very high range (not defined)"+name))
		Expect(got).To(HaveKeyWithValue("READINGS_BELOW_RANGE_VERY_LOW_SMBG",
			"SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL "+
				"(<3.0 mmol/L) during reporting period"+name))
	})

	It("describes a preset without a very low range", func() {
		got := descriptions("preset:adaHighRisk", nil)
		const name = "; glycemic ranges: Older/High Risk (Type 1 and 2)"

		Expect(got).To(HaveKeyWithValue("TIME_BELOW_RANGE_LOW_SMBG",
			"% of readings <70 mg/dL (<3.9 mmol/L)"+name))
		Expect(got).To(HaveKeyWithValue("TIME_ABOVE_RANGE_HIGH_SMBG",
			"% of readings 181–250 mg/dL (10.1–13.9 mmol/L)"+name))
	})

	DescribeTable("names and bounds every preset platform defines",
		func(preset string) {
			got := descriptions("preset:"+preset, nil)

			Expect(got).To(HaveKey("TIME_IN_RANGE_CGM"))
			description := got["TIME_IN_RANGE_CGM"]
			Expect(description).To(ContainSubstring(" mg/dL ("))
			Expect(description).To(ContainSubstring(" mmol/L); glycemic ranges: "))
			Expect(description).ToNot(HaveSuffix("; glycemic ranges: " + preset))
		},
		Entry("the high risk preset", summaryTypes.PresetADAHighRisk),
		Entry("the pregnancy type 1 preset", summaryTypes.PresetADAPregnancyType1),
		Entry("the pregnancy type 2 preset", summaryTypes.PresetADAPregnancyType2),
	)

	Context("with custom ranges", func() {
		const key = "custom:mmol/L:,4,8.5,12"

		custom := func(name, key string) *clinics.GlycemicRangesV1 {
			return &clinics.GlycemicRangesV1{
				Type:   clinics.Custom,
				Key:    key,
				Custom: clinics.GlycemicRangesCustomV1{Name: name},
			}
		}

		It("describes them in their own unit, then the other, and names them", func() {
			got := descriptions(key, custom("Clinic tight control", key))
			const name = "; glycemic ranges: Clinic tight control"

			Expect(got).To(HaveKeyWithValue("TIME_IN_RANGE_CGM",
				cgmPrefix+"4.0–8.5 mmol/L (72–153 mg/dL)"+name))
			Expect(got).To(HaveKeyWithValue("TIME_BELOW_RANGE_LOW_SMBG",
				"% of readings <4.0 mmol/L (<72 mg/dL)"+name))
			Expect(got).To(HaveKeyWithValue("TIME_ABOVE_RANGE_HIGH_SMBG",
				"% of readings 8.6–12.0 mmol/L (154–216 mg/dL)"+name))
			Expect(got).To(HaveKeyWithValue("TIME_ABOVE_RANGE_VERY_HIGH_SMBG",
				"% of readings >12.0 mmol/L (>216 mg/dL)"+name))
		})

		It("describes the other unit as a preset with the same thresholds", func() {
			got := descriptions("custom:mg/dL:54,63,140,", nil)

			Expect(got).To(HaveKeyWithValue("TIME_IN_RANGE_CGM",
				cgmPrefix+"63–140 mg/dL (3.5–7.8 mmol/L); glycemic ranges: custom"))
		})

		It("doesn't give a summary the name of ranges it wasn't computed with", func() {
			got := descriptions(key, custom("New ranges", "custom:mmol/L:,4,9,12"))

			Expect(got).To(HaveKeyWithValue("TIME_IN_RANGE_CGM",
				cgmPrefix+"4.0–8.5 mmol/L (72–153 mg/dL); glycemic ranges: custom"))
		})

		It("doesn't name them when the patient has no ranges", func() {
			got := descriptions("custom:mg/dL:54,65,150,", nil)

			Expect(got).To(HaveKeyWithValue("TIME_BELOW_RANGE_LOW_CGM",
				"CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of "+
					"readings and time 54–64 mg/dL (3.0–3.5 mmol/L); glycemic ranges: "+
					"custom"))
		})
	})

	DescribeTable("describes ranges it can't determine without bounds",
		func(key, expected string) {
			Expect(descriptions(key, nil)).To(HaveKeyWithValue("TIME_IN_RANGE_SMBG",
				"% of readings in the target range; glycemic ranges: "+expected))
		},
		Entry("an unknown preset", "preset:somethingNew", "somethingNew"),
		Entry("a custom key in unknown units", "custom:furlongs:1,2,3,4", "custom"),
		Entry("a custom key missing a threshold", "custom:mg/dL:54,,180,250", "custom"),
		Entry("a custom key with a malformed threshold",
			"custom:mg/dL:54,x,180,250", "custom"),
		Entry("a custom key with a threshold that isn't a number",
			"custom:mg/dL:54,NaN,180,250", "custom"),
		Entry("a custom key with an infinite threshold",
			"custom:mg/dL:54,70,180,Inf", "custom"),
		Entry("a custom key with a negative threshold",
			"custom:mg/dL:-54,70,180,250", "custom"),
		Entry("a custom key with thresholds out of order",
			"custom:mg/dL:,180,70,", "custom"),
		Entry("a custom key with a fraction of a mg/dL", "custom:mg/dL:,70.5,180,", "custom"),
		Entry("a custom key with mg/dL thresholds less than 2 apart",
			"custom:mg/dL:,104,105,", "custom"),
		Entry("a key of an unknown kind", "other:thing", "other:thing"),
	)

	Describe("threshold observations", func() {
		DescribeTable("state the standard ranges' thresholds",
			func(cgmKey, bgmKey string) {
				const name = "; glycemic ranges: Standard (Type 1 and 2)"

				got := observe(cgmKey, bgmKey, nil, mgdL)
				Expect(values(got)).To(And(
					HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_HIGH", "250"),
					HaveKeyWithValue("GLUCOSE_THRESHOLD_HIGH", "180"),
					HaveKeyWithValue("GLUCOSE_THRESHOLD_LOW", "70"),
					HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_LOW", "54"),
				))
				Expect(got["GLUCOSE_THRESHOLD_VERY_LOW"].Description).To(Equal(
					"Hypoglycemia level 2: glucose below this threshold is very low" + name))
				Expect(got["GLUCOSE_THRESHOLD_VERY_LOW"].Units).To(
					HaveValue(Equal("mg/dL")))

				got = observe(cgmKey, bgmKey, nil, mmolL)
				Expect(values(got)).To(And(
					HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_HIGH", "13.9"),
					HaveKeyWithValue("GLUCOSE_THRESHOLD_HIGH", "10.0"),
					HaveKeyWithValue("GLUCOSE_THRESHOLD_LOW", "3.9"),
					HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_LOW", "3.0"),
				))
			},
			Entry("when the summaries have no key", "", ""),
			Entry("when the summaries have the standard key",
				"preset:adaStandard", "preset:adaStandard"),
			Entry("when only one summary has the standard key", "", "preset:adaStandard"),
		)

		It("state a preset's thresholds in the clinic's preferred units", func() {
			const key = "preset:adaHighRisk"
			const name = "; glycemic ranges: Older/High Risk (Type 1 and 2)"

			got := observe(key, key, nil, mgdL)
			Expect(values(got)).To(And(
				HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_HIGH", "250"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_HIGH", "180"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_LOW", "70"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_LOW", "NaN"),
			))
			high := got["GLUCOSE_THRESHOLD_HIGH"]
			Expect(high.ValueType).To(Equal("Numeric"))
			Expect(high.Units).To(HaveValue(Equal("mg/dL")))
			Expect(high.DateTime).To(Equal(got["TIME_IN_RANGE_CGM"].DateTime))
			Expect(high.Description).To(Equal(
				"Hyperglycemia level 1: glucose above this threshold is high" + name))
			Expect(got["GLUCOSE_THRESHOLD_VERY_HIGH"].Description).To(Equal(
				"Hyperglycemia level 2: glucose above this threshold is very high" + name))
			Expect(got["GLUCOSE_THRESHOLD_LOW"].Description).To(Equal(
				"Hypoglycemia level 1: glucose below this threshold is low" + name))
			Expect(got["GLUCOSE_THRESHOLD_VERY_LOW"].Description).To(Equal(
				"Hypoglycemia level 2: glucose below this threshold is very low" + name))

			got = observe(key, key, nil, mmolL)
			Expect(values(got)).To(And(
				HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_HIGH", "13.9"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_HIGH", "10.0"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_LOW", "3.9"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_LOW", "NaN"),
			))
			Expect(got["GLUCOSE_THRESHOLD_LOW"].Units).To(
				HaveValue(Equal("mmol/L")))
		})

		It("state a custom range set's thresholds converted to the preferred units", func() {
			const key = "custom:mmol/L:,4,8.5,12"
			ranges := &clinics.GlycemicRangesV1{
				Type:   clinics.Custom,
				Key:    key,
				Custom: clinics.GlycemicRangesCustomV1{Name: "Clinic tight control"},
			}

			got := observe(key, key, ranges, mgdL)
			Expect(values(got)).To(And(
				HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_HIGH", "216"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_HIGH", "153"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_LOW", "72"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_LOW", "NaN"),
			))
			Expect(got["GLUCOSE_THRESHOLD_LOW"].Description).To(
				HaveSuffix("; glycemic ranges: Clinic tight control"))

			got = observe(key, key, ranges, mmolL)
			Expect(values(got)).To(And(
				HaveKeyWithValue("GLUCOSE_THRESHOLD_VERY_HIGH", "12.0"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_HIGH", "8.5"),
				HaveKeyWithValue("GLUCOSE_THRESHOLD_LOW", "4.0"),
			))
		})

		It("aren't sent when the thresholds can't be determined", func() {
			got := observe("preset:somethingNew", "preset:somethingNew", nil, mgdL)
			for _, code := range thresholdCodes {
				Expect(got).ToNot(HaveKey(code))
			}
		})

		It("aren't sent when the summaries were computed with different ranges", func() {
			got := observe("preset:adaHighRisk", "preset:adaPregnancyType1", nil, mgdL)
			for _, code := range thresholdCodes {
				Expect(got).ToNot(HaveKey(code))
			}
			Expect(got).To(HaveKey("TIME_IN_RANGE_CGM"))
			Expect(got).To(HaveKey("TIME_IN_RANGE_SMBG"))
		})
	})

	Describe("ranges the range set doesn't define", func() {
		undefinedVeryLow := []string{
			"TIME_BELOW_RANGE_VERY_LOW_CGM",
			"TIME_BELOW_RANGE_VERY_LOW_SMBG",
			"READINGS_BELOW_RANGE_VERY_LOW_SMBG",
		}
		undefinedVeryHigh := []string{
			"TIME_ABOVE_RANGE_VERY_HIGH_CGM",
			"TIME_ABOVE_RANGE_VERY_HIGH_SMBG",
			"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG",
		}

		DescribeTable("have the value NaN",
			func(key string, settings redox.FlowsheetSettings, undefined,
				defined []string) {

				got := values(observe(key, key, nil, settings))
				for _, code := range undefined {
					Expect(got).To(HaveKeyWithValue(code, "NaN"))
				}
				for _, code := range defined {
					Expect(got).To(HaveKey(code))
					Expect(got[code]).ToNot(Equal("NaN"))
				}
				Expect(got["TIME_IN_RANGE_CGM"]).ToNot(Equal("NaN"))
				Expect(got["TIME_BELOW_RANGE_LOW_CGM"]).ToNot(Equal("NaN"))
				Expect(got["TIME_ABOVE_RANGE_HIGH_SMBG"]).ToNot(Equal("NaN"))
			},
			Entry("the very low range of the high risk preset",
				"preset:adaHighRisk", mgdL, undefinedVeryLow, undefinedVeryHigh),
			Entry("the very high range of a pregnancy preset",
				"preset:adaPregnancyType2", mgdL, undefinedVeryHigh, undefinedVeryLow),
			Entry("both outer ranges of a custom range set",
				"custom:mg/dL:,70,180,", mgdL,
				append(undefinedVeryLow, undefinedVeryHigh...), nil),
			Entry("with icode formatting", "preset:adaHighRisk",
				redox.FlowsheetSettings{PreferredBGUnits: "mg/dL", ICode: true},
				undefinedVeryLow, undefinedVeryHigh),
			Entry("none of the standard ranges", "preset:adaStandard", mgdL,
				nil, append(undefinedVeryLow, undefinedVeryHigh...)),
			Entry("none of ranges that can't be determined", "preset:somethingNew", mgdL,
				nil, append(undefinedVeryLow, undefinedVeryHigh...)),
		)
	})
})

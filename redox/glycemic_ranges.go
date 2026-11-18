package redox

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/platform/data/blood/glucose"
	summaryTypes "github.com/tidepool-org/platform/summary/types"
)

const (
	presetKeyPrefix = "preset:"
	customKeyPrefix = "custom:"

	unitsMgdL  = glucose.MgdL
	unitsMmolL = glucose.MmolL

	// notANumber is the value of an observation about a very low or very high range, or
	// its threshold, when the glycemic ranges define no such range. The readings such a
	// range would hold are counted in the low or high range, so a value of 0 would read
	// as there being none.
	notANumber = "NaN"
)

// glycemicRange names one of the five ranges a summary's readings are divided into.
type glycemicRange int

const (
	rangeVeryLow glycemicRange = iota
	rangeLow
	rangeTarget
	rangeHigh
	rangeVeryHigh
)

// unitThresholds are the thresholds between the glycemic ranges in one unit. A reading at
// a threshold belongs to the range nearer the target range. A nil VeryLow or VeryHigh
// means that the range set has no such range.
type unitThresholds struct {
	Units    string
	VeryLow  *float64
	Low      float64
	High     float64
	VeryHigh *float64
}

// presetGlycemicRangesNames holds the display name of each preset. The names are those
// the clinic chooses from in the web application (blip's SelectGlycemicRanges options).
// They are copied here, as nothing else serves them, so a label reworded in blip stays as
// it is here until this map is edited. A preset without a name here is described by its
// identifier. The standard preset's name appears only in the threshold observations, as
// the descriptions of the other observations are fixed for the standard ranges.
var presetGlycemicRangesNames = map[string]string{
	summaryTypes.PresetADAStandard:       "Standard (Type 1 and 2)",
	summaryTypes.PresetADAHighRisk:       "Older/High Risk (Type 1 and 2)",
	summaryTypes.PresetADAPregnancyType1: "Pregnancy (Type 1)",
	summaryTypes.PresetADAPregnancyType2: "Pregnancy (Gestational and Type 2)",
}

// presetGlycemicRangesThresholds returns the thresholds of a preset, in mg/dL and then in
// mmol/L, or false for a preset that isn't known.
//
// The thresholds are platform's, from [summaryTypes.PresetRangeSet], which the summaries
// are computed with, so they aren't defined a second time here. They can still drift
// from the summaries, though: this service uses the copy of platform that its go.mod
// pins and vendors, while the summaries are computed by whichever platform is deployed.
// If platform changes a preset's thresholds, the descriptions keep the old bounds until
// the pin is bumped here and this service is deployed again; if platform adds a preset,
// it is unknown here until then, and its observations are described without bounds.
// Bump the platform pin whenever platform's presets change.
func presetGlycemicRangesThresholds(preset string) ([]unitThresholds, bool) {
	rangeSet, ok := summaryTypes.PresetRangeSet(preset)
	if !ok {
		return nil, false
	}
	thresholds := rangeSetThresholds(rangeSet, unitsMgdL, unitsMmolL)
	return thresholds, len(thresholds) > 0
}

// customGlycemicRangesThresholds returns the thresholds of the custom range set
// identified by key, in the unit the set is defined in and then in the other unit, or
// false when key doesn't identify a valid custom range set.
//
// The other unit's thresholds are those platform converts the set to, from
// [summaryTypes.NewCustomRangeSet], which the summaries classify readings in that unit
// with. It also validates the thresholds, so a key that parses but names thresholds that
// aren't numbers, aren't ascending, or aren't at the unit's resolution is rejected.
func customGlycemicRangesThresholds(key string) ([]unitThresholds, bool) {
	defined, ok := parseCustomGlycemicRangesKey(key)
	if !ok {
		return nil, false
	}
	rangeSet, err := summaryTypes.NewCustomRangeSet(key, defined.Units,
		summaryTypes.Thresholds{
			VeryLow:  defined.VeryLow,
			Low:      defined.Low,
			High:     defined.High,
			VeryHigh: defined.VeryHigh,
		})
	if err != nil {
		return nil, false
	}
	other := unitsMmolL
	if defined.Units == unitsMmolL {
		other = unitsMgdL
	}
	thresholds := rangeSetThresholds(rangeSet, defined.Units, other)
	return thresholds, len(thresholds) > 0
}

// rangeSetThresholds returns the thresholds of the range set in each of units that it
// defines them in, in the order given.
func rangeSetThresholds(rangeSet summaryTypes.RangeSet, units ...string) []unitThresholds {
	var thresholds []unitThresholds
	for _, unit := range units {
		if inUnits, ok := rangeSet.Thresholds[unit]; ok {
			thresholds = append(thresholds, unitThresholds{
				Units:    unit,
				VeryLow:  inUnits.VeryLow,
				Low:      inUnits.Low,
				High:     inUnits.High,
				VeryHigh: inUnits.VeryHigh,
			})
		}
	}
	return thresholds
}

// A glycemicRangesDescriber words the flowsheet descriptions of the observations that
// depend on a summary's glycemic ranges, and supplies the observations of the ranges'
// thresholds. The zero value describes unnamed ranges whose thresholds aren't known.
// Use [newGlycemicRangesDescriber] to get one for the standard ranges.
type glycemicRangesDescriber struct {
	// standard reports whether the summary was computed with the standard ranges, whose
	// descriptions are fixed.
	standard bool
	// name is the display name of the ranges, or empty when it isn't known.
	name string
	// thresholds are the thresholds in each unit they are known in, or empty when the
	// ranges can't be determined.
	thresholds []unitThresholds
}

// newGlycemicRangesDescriber returns a describer for a summary computed with the range
// set identified by key, the summary's config.glycemicRangesKey. An empty key means the
// standard ranges. The patient's ranges supply the name of a custom range set, but only
// when their key is the summary's: after the ranges change, the summary remains that of
// the previous ranges until it is recomputed, and must not be given the new name. A
// custom range set is described in the unit it is defined in, followed by the other.
func newGlycemicRangesDescriber(key string,
	ranges *clinics.GlycemicRangesV1) glycemicRangesDescriber {

	if key == "" || key == summaryTypes.StandardRangeSetKey {
		describer := glycemicRangesDescriber{
			standard: true,
			name:     presetGlycemicRangesNames[summaryTypes.PresetADAStandard],
		}
		if thresholds, ok := presetGlycemicRangesThresholds(
			summaryTypes.PresetADAStandard); ok {

			describer.thresholds = thresholds
		}
		return describer
	}
	if preset, ok := strings.CutPrefix(key, presetKeyPrefix); ok {
		describer := glycemicRangesDescriber{name: preset}
		if name, ok := presetGlycemicRangesNames[preset]; ok {
			describer.name = name
		}
		if thresholds, ok := presetGlycemicRangesThresholds(preset); ok {
			describer.thresholds = thresholds
		}
		return describer
	}
	if !strings.HasPrefix(key, customKeyPrefix) {
		return glycemicRangesDescriber{name: key}
	}
	describer := glycemicRangesDescriber{}
	if ranges != nil && ranges.Key == key {
		describer.name = ranges.Custom.Name
	}
	if thresholds, ok := customGlycemicRangesThresholds(key); ok {
		describer.thresholds = thresholds
	}
	return describer
}

// thresholdsKnown reports whether the ranges' thresholds could be determined. They are
// always known for the standard ranges.
func (d glycemicRangesDescriber) thresholdsKnown() bool {
	return d.standard || len(d.thresholds) > 0
}

// defines reports whether the ranges define the given range. Only the very low and very
// high ranges can be undefined. Ranges whose thresholds aren't known are taken to define
// every range, as nothing says otherwise.
func (d glycemicRangesDescriber) defines(r glycemicRange) bool {
	if d.standard || len(d.thresholds) == 0 {
		return true
	}
	switch r {
	case rangeVeryLow:
		return d.thresholds[0].VeryLow != nil
	case rangeVeryHigh:
		return d.thresholds[0].VeryHigh != nil
	}
	return true
}

// markUndefinedRanges replaces the value of each observation about a range that the
// ranges don't define with [notANumber]. The observations are named by code in
// rangeByCode. An observation without a value stays without one.
func (d glycemicRangesDescriber) markUndefinedRanges(observations []*Observation,
	rangeByCode map[string]glycemicRange) {

	for _, observation := range observations {
		r, ok := rangeByCode[observation.Code]
		if ok && !d.defines(r) && observation.Value != missingValue {
			observation.Value = notANumber
		}
	}
}

// The codes of the observations of the glycemic ranges' thresholds.
const (
	codeThresholdVeryHigh = "GLUCOSE_THRESHOLD_VERY_HIGH"
	codeThresholdHigh     = "GLUCOSE_THRESHOLD_HIGH"
	codeThresholdLow      = "GLUCOSE_THRESHOLD_LOW"
	codeThresholdVeryLow  = "GLUCOSE_THRESHOLD_VERY_LOW"
)

// thresholdObservations returns an observation of each threshold between the ranges, so
// that a flowsheet states the ranges its values were computed with in rows of their own,
// whether or not the reader sees the descriptions. They are returned for the standard
// ranges as for any other, so that a flowsheet always states its ranges, and a change
// back to the standard ranges shows as one. It returns none when the thresholds aren't
// known.
//
// The thresholds are given in preferredUnits, the clinic's preferred glucose units, when
// they are known in those, and otherwise in the first unit they are known in. A
// threshold that the ranges don't define has the value [notANumber]. Each observation's
// time is dateTime.
func (d glycemicRangesDescriber) thresholdObservations(preferredUnits,
	dateTime string) []*Observation {

	if len(d.thresholds) == 0 {
		return nil
	}
	thresholds := d.thresholds[0]
	for _, candidate := range d.thresholds {
		if strings.EqualFold(candidate.Units, preferredUnits) {
			thresholds = candidate
			break
		}
	}
	name := d.name
	if name == "" {
		name = "custom"
	}
	units := thresholds.Units
	observation := func(code string, value *float64, description string) *Observation {
		formatted := notANumber
		if value != nil {
			formatted = thresholds.format(*value)
		}
		return &Observation{
			Code:        code,
			Value:       formatted,
			ValueType:   "Numeric",
			Units:       &units,
			DateTime:    dateTime,
			Description: description + "; glycemic ranges: " + name,
		}
	}
	return []*Observation{
		observation(codeThresholdVeryHigh, thresholds.VeryHigh,
			"Hyperglycemia level 2: glucose above this threshold is very high"),
		observation(codeThresholdHigh, &thresholds.High,
			"Hyperglycemia level 1: glucose above this threshold is high"),
		observation(codeThresholdLow, &thresholds.Low,
			"Hypoglycemia level 1: glucose below this threshold is low"),
		observation(codeThresholdVeryLow, thresholds.VeryLow,
			"Hypoglycemia level 2: glucose below this threshold is very low"),
	}
}

// glycemicRangesThresholdObservations returns the threshold observations (see
// [glycemicRangesDescriber.thresholdObservations]) for the summaries a flowsheet
// reports, each given by the key of the range set it was computed with and the time of
// its observations. A summary without a key was computed with the standard ranges. It
// returns none when the summaries were computed with different range sets, as one set of
// thresholds would then misdescribe some of the values; the descriptions of the values
// still state each summary's ranges.
func glycemicRangesThresholdObservations(keys, dateTimes []string,
	ranges *clinics.GlycemicRangesV1, preferredUnits string) []*Observation {

	if len(keys) == 0 || len(keys) != len(dateTimes) {
		return nil
	}
	normalized := make([]string, len(keys))
	for i, key := range keys {
		normalized[i] = key
		if key == "" {
			normalized[i] = summaryTypes.StandardRangeSetKey
		}
	}
	if slices.ContainsFunc(normalized, func(key string) bool {
		return key != normalized[0]
	}) {
		return nil
	}
	describer := newGlycemicRangesDescriber(normalized[0], ranges)
	return describer.thresholdObservations(preferredUnits, dateTimes[0])
}

// parseCustomGlycemicRangesKey returns the thresholds named by the key of a custom range
// set, which has the form custom:<units>:<veryLow>,<low>,<high>,<veryHigh>, with the
// optional veryLow and veryHigh left empty when unset. It reports whether key has that
// form.
func parseCustomGlycemicRangesKey(key string) (unitThresholds, bool) {
	rest, ok := strings.CutPrefix(key, customKeyPrefix)
	if !ok {
		return unitThresholds{}, false
	}
	units, list, ok := strings.Cut(rest, ":")
	if !ok || (units != unitsMgdL && units != unitsMmolL) {
		return unitThresholds{}, false
	}
	fields := strings.Split(list, ",")
	if len(fields) != 4 {
		return unitThresholds{}, false
	}
	values := make([]*float64, len(fields))
	for i, field := range fields {
		if field == "" {
			continue
		}
		value, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return unitThresholds{}, false
		}
		values[i] = &value
	}
	if values[1] == nil || values[2] == nil {
		return unitThresholds{}, false
	}
	return unitThresholds{
		Units:    units,
		VeryLow:  values[0],
		Low:      *values[1],
		High:     *values[2],
		VeryHigh: values[3],
	}, true
}

// describe returns the description of an observation about the given range. For the
// standard ranges that is the fixed description standard. For any other ranges it is
// prefix followed by the bounds of the range, such as "63–140 mg/dL (3.5–7.8 mmol/L)",
// then suffix, then a note naming the ranges, so that a reader can tell that the
// observation isn't for the standard ranges, and which ranges it is for.
func (d glycemicRangesDescriber) describe(r glycemicRange, standard, prefix,
	suffix string) string {

	if d.standard {
		return standard
	}
	name := d.name
	if name == "" {
		name = "custom"
	}
	return fmt.Sprintf("%s %s%s; glycemic ranges: %s", prefix, d.bounds(r), suffix, name)
}

// bounds returns the bounds of the range in each unit the thresholds are known in, the
// first unit followed by any other in parentheses. A range whose bounds aren't known is
// named instead, and so is a very low or very high range that the ranges don't define.
func (d glycemicRangesDescriber) bounds(r glycemicRange) string {
	named := map[glycemicRange]string{
		rangeVeryLow:  "in the very low range",
		rangeLow:      "in the low range",
		rangeTarget:   "in the target range",
		rangeHigh:     "in the high range",
		rangeVeryHigh: "in the very high range",
	}[r]
	if len(d.thresholds) == 0 {
		return named
	}
	bounds, ok := d.thresholds[0].bounds(r)
	if !ok {
		return named + " (not defined)"
	}
	for _, thresholds := range d.thresholds[1:] {
		if other, ok := thresholds.bounds(r); ok {
			bounds += " (" + other + ")"
		}
	}
	return bounds
}

// undeterminedGlycemicRangesKeys returns the keys of the range sets the patient's
// summaries were computed with whose thresholds can't be determined, for which the
// flowsheet has neither bounds in its descriptions nor threshold observations. Such a
// key names a preset this service's copy of platform doesn't know, or has a form this
// service doesn't parse.
func undeterminedGlycemicRangesKeys(patient clinics.PatientV1) []string {
	if patient.Summary == nil {
		return nil
	}
	var keys []string
	if stats := patient.Summary.CgmStats; stats != nil {
		keys = append(keys, stats.Config.GlycemicRangesKey)
	}
	if stats := patient.Summary.BgmStats; stats != nil {
		keys = append(keys, stats.Config.GlycemicRangesKey)
	}
	keys = slices.DeleteFunc(keys, func(key string) bool {
		return newGlycemicRangesDescriber(key, patient.GlycemicRanges).thresholdsKnown()
	})
	slices.Sort(keys)
	return slices.Compact(keys)
}

// format returns value in the thresholds' units at the resolution of readings in those
// units: whole mg/dL, or a single decimal of mmol/L.
func (t unitThresholds) format(value float64) string {
	decimals := 0
	if t.Units == unitsMmolL {
		decimals = 1
	}
	return strconv.FormatFloat(value, 'f', decimals, 64)
}

// bounds returns the bounds of the range in the thresholds' units, such as "<54 mg/dL",
// "54–62 mg/dL", or ">140 mg/dL". It reports false for the very low or very high range
// of thresholds that don't define one.
func (t unitThresholds) bounds(r glycemicRange) (string, bool) {
	// A reading at a threshold belongs to the range nearer target, so the range on the
	// far side of a threshold starts one step of the unit's resolution beyond it.
	step := 1.0
	if t.Units == unitsMmolL {
		step = 0.1
	}
	format := t.format
	below := func(value float64) string { return "<" + format(value) + " " + t.Units }
	above := func(value float64) string { return ">" + format(value) + " " + t.Units }
	between := func(low, high float64) string {
		return format(low) + "–" + format(high) + " " + t.Units
	}

	switch r {
	case rangeVeryLow:
		if t.VeryLow == nil {
			return "", false
		}
		return below(*t.VeryLow), true
	case rangeLow:
		if t.VeryLow == nil {
			return below(t.Low), true
		}
		return between(*t.VeryLow, t.Low-step), true
	case rangeTarget:
		return between(t.Low, t.High), true
	case rangeHigh:
		if t.VeryHigh == nil {
			return above(t.High), true
		}
		return between(t.High+step, *t.VeryHigh), true
	case rangeVeryHigh:
		if t.VeryHigh == nil {
			return "", false
		}
		return above(*t.VeryHigh), true
	}
	return "", false
}

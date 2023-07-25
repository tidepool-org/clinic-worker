package patients

import (
	clinics "github.com/tidepool-org/clinic/client"
	summaries "github.com/tidepool-org/go-common/clients/summary"
	"github.com/tidepool-org/go-common/errors"
)

func ApplyPatientChangesToProfile(patient Patient, profile map[string]interface{}) {
	patientProfile := EnsurePatientProfileExists(profile)
	if patient.FullName != nil {
		profile["fullName"] = *patient.FullName
	}
	if patient.BirthDate != nil {
		patientProfile["birthday"] = *patient.BirthDate
	}
	if patient.Mrn != nil {
		patientProfile["mrn"] = *patient.Mrn
	}
	if patient.TargetDevices != nil {
		patientProfile["targetDevices"] = *patient.TargetDevices
	}
	if patient.Email != nil {
		profile["email"] = *patient.Email
		patientProfile["emails"] = []string{*patient.Email}
	}
}

func RemoveFieldsFromProfile(removedFields []string, profile map[string]interface{}) {
	patientProfile := EnsurePatientProfileExists(profile)
	removedFieldsMap := make(map[string]bool, 0)
	for _, field := range removedFields {
		removedFieldsMap[field] = true
	}

	if _, ok := removedFieldsMap["fullName"]; ok {
		delete(profile, "fullName")
	}
	if _, ok := removedFieldsMap["birthDate"]; ok {
		delete(patientProfile, "birthday")
	}
	if _, ok := removedFieldsMap["mrn"]; ok {
		delete(patientProfile, "mrn")
	}
	if _, ok := removedFieldsMap["targetDevices"]; ok {
		delete(patientProfile, "targetDevices")
	}
	if _, ok := removedFieldsMap["email"]; ok {
		delete(profile, "email")
		delete(patientProfile, "emails")
	}
}

func EnsurePatientProfileExists(profile map[string]interface{}) map[string]interface{} {
	switch profile["patient"].(type) {
	case map[string]interface{}:
		return profile["patient"].(map[string]interface{})
	default:
		patientProfile := make(map[string]interface{}, 0)
		profile["patient"] = patientProfile
		return patientProfile
	}
}

func CreateSummaryUpdateBody(cgmSummary *summaries.Summary, bgmSummary *summaries.Summary) (clinics.UpdatePatientSummaryJSONRequestBody, error) {
	// summaries don't exist, return empty body
	if cgmSummary == nil && bgmSummary == nil {
		return clinics.UpdatePatientSummaryJSONRequestBody{}, nil
	}

	cgmStats, err := cgmSummary.Stats.AsCGMStats()
	if err != nil {
		return clinics.UpdatePatientSummaryJSONRequestBody{}, errors.Wrapf(err, "unable to unserialize CGM summary stats for userId %s", cgmSummary.UserId)
	}

	bgmStats, err := bgmSummary.Stats.AsBGMStats()
	if err != nil {
		return clinics.UpdatePatientSummaryJSONRequestBody{}, errors.Wrapf(err, "unable to unserialize BGM summary stats for userId %s", bgmSummary.UserId)
	}

	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{
		CgmStats: &clinics.PatientCGMStats{
			Dates:      (*clinics.PatientSummaryDates)(cgmSummary.Dates),
			TotalHours: cgmStats.TotalHours,
			Config: &clinics.PatientSummaryConfig{
				HighGlucoseThreshold:     cgmSummary.Config.HighGlucoseThreshold,
				LowGlucoseThreshold:      cgmSummary.Config.LowGlucoseThreshold,
				SchemaVersion:            cgmSummary.Config.SchemaVersion,
				VeryHighGlucoseThreshold: cgmSummary.Config.VeryHighGlucoseThreshold,
				VeryLowGlucoseThreshold:  cgmSummary.Config.VeryLowGlucoseThreshold,
			},
		},
		BgmStats: &clinics.PatientBGMStats{
			Dates: &clinics.PatientSummaryDates{
				LastUpdatedDate: bgmSummary.Dates.LastUpdatedDate,

				HasLastUploadDate: bgmSummary.Dates.HasLastUploadDate,
				LastUploadDate:    bgmSummary.Dates.LastUploadDate,

				HasFirstData: bgmSummary.Dates.HasFirstData,
				FirstData:    bgmSummary.Dates.FirstData,

				HasLastData: bgmSummary.Dates.HasLastData,
				LastData:    bgmSummary.Dates.LastData,

				HasOutdatedSince: bgmSummary.Dates.HasOutdatedSince,
				OutdatedSince:    bgmSummary.Dates.OutdatedSince,
			},
			TotalHours: bgmStats.TotalHours,
			Config: &clinics.PatientSummaryConfig{
				HighGlucoseThreshold:     bgmSummary.Config.HighGlucoseThreshold,
				LowGlucoseThreshold:      bgmSummary.Config.LowGlucoseThreshold,
				SchemaVersion:            bgmSummary.Config.SchemaVersion,
				VeryHighGlucoseThreshold: bgmSummary.Config.VeryHighGlucoseThreshold,
				VeryLowGlucoseThreshold:  bgmSummary.Config.VeryLowGlucoseThreshold,
			},
		},
	}

	if cgmStats.Periods != nil {
		patientUpdate.CgmStats.Periods = &clinics.PatientCGMPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*summaries.CGMPeriod{}
		destPeriods := map[string]*clinics.PatientCGMPeriod{}
		if cgmStats.Periods.N1d != nil {
			sourcePeriods["1d"] = cgmStats.Periods.N1d

			patientUpdate.CgmStats.Periods.N1d = &clinics.PatientCGMPeriod{}
			destPeriods["1d"] = patientUpdate.CgmStats.Periods.N1d
		}
		if cgmStats.Periods.N7d != nil {
			sourcePeriods["7d"] = cgmStats.Periods.N7d

			patientUpdate.CgmStats.Periods.N7d = &clinics.PatientCGMPeriod{}
			destPeriods["7d"] = patientUpdate.CgmStats.Periods.N7d
		}
		if cgmStats.Periods.N14d != nil {
			sourcePeriods["14d"] = cgmStats.Periods.N14d

			patientUpdate.CgmStats.Periods.N14d = &clinics.PatientCGMPeriod{}
			destPeriods["14d"] = patientUpdate.CgmStats.Periods.N14d
		}
		if cgmStats.Periods.N30d != nil {
			sourcePeriods["30d"] = cgmStats.Periods.N30d

			patientUpdate.CgmStats.Periods.N30d = &clinics.PatientCGMPeriod{}
			destPeriods["30d"] = patientUpdate.CgmStats.Periods.N30d
		}

		for period := range sourcePeriods {
			if sourcePeriods[period].AverageGlucose != nil {
				destPeriods[period].AverageGlucose = &clinics.AverageGlucose{
					Value: sourcePeriods[period].AverageGlucose.Value,
					Units: clinics.AverageGlucoseUnits(sourcePeriods[period].AverageGlucose.Units)}
			}
			destPeriods[period].HasAverageGlucose = sourcePeriods[period].HasAverageGlucose

			destPeriods[period].GlucoseManagementIndicator = sourcePeriods[period].GlucoseManagementIndicator
			destPeriods[period].HasGlucoseManagementIndicator = sourcePeriods[period].HasGlucoseManagementIndicator

			destPeriods[period].HasTimeCGMUsePercent = sourcePeriods[period].HasTimeCGMUsePercent
			destPeriods[period].TimeCGMUsePercent = sourcePeriods[period].TimeCGMUsePercent

			destPeriods[period].HasTimeCGMUseMinutes = sourcePeriods[period].HasTimeCGMUseMinutes
			destPeriods[period].TimeCGMUseMinutes = sourcePeriods[period].TimeCGMUseMinutes

			destPeriods[period].HasTimeCGMUseRecords = sourcePeriods[period].HasTimeCGMUseRecords
			destPeriods[period].TimeCGMUseRecords = sourcePeriods[period].TimeCGMUseRecords

			destPeriods[period].HasTimeInHighPercent = sourcePeriods[period].HasTimeInHighPercent
			destPeriods[period].TimeInHighPercent = sourcePeriods[period].TimeInHighPercent

			destPeriods[period].HasTimeInHighMinutes = sourcePeriods[period].HasTimeInHighMinutes
			destPeriods[period].TimeInHighMinutes = sourcePeriods[period].TimeInHighMinutes

			destPeriods[period].HasTimeInHighRecords = sourcePeriods[period].HasTimeInHighRecords
			destPeriods[period].TimeInHighRecords = sourcePeriods[period].TimeInHighRecords

			destPeriods[period].HasTimeInLowPercent = sourcePeriods[period].HasTimeInLowPercent
			destPeriods[period].TimeInLowPercent = sourcePeriods[period].TimeInLowPercent

			destPeriods[period].HasTimeInLowMinutes = sourcePeriods[period].HasTimeInLowMinutes
			destPeriods[period].TimeInLowMinutes = sourcePeriods[period].TimeInLowMinutes

			destPeriods[period].HasTimeInLowRecords = sourcePeriods[period].HasTimeInLowRecords
			destPeriods[period].TimeInLowRecords = sourcePeriods[period].TimeInLowRecords

			destPeriods[period].HasTimeInTargetPercent = sourcePeriods[period].HasTimeInTargetPercent
			destPeriods[period].TimeInTargetPercent = sourcePeriods[period].TimeInTargetPercent

			destPeriods[period].HasTimeInTargetMinutes = sourcePeriods[period].HasTimeInTargetMinutes
			destPeriods[period].TimeInTargetMinutes = sourcePeriods[period].TimeInTargetMinutes

			destPeriods[period].HasTimeInTargetRecords = sourcePeriods[period].HasTimeInTargetRecords
			destPeriods[period].TimeInTargetRecords = sourcePeriods[period].TimeInTargetRecords

			destPeriods[period].HasTimeInVeryHighPercent = sourcePeriods[period].HasTimeInVeryHighPercent
			destPeriods[period].TimeInVeryHighPercent = sourcePeriods[period].TimeInVeryHighPercent

			destPeriods[period].HasTimeInVeryHighMinutes = sourcePeriods[period].HasTimeInVeryHighMinutes
			destPeriods[period].TimeInVeryHighMinutes = sourcePeriods[period].TimeInVeryHighMinutes

			destPeriods[period].HasTimeInVeryHighRecords = sourcePeriods[period].HasTimeInVeryHighRecords
			destPeriods[period].TimeInVeryHighRecords = sourcePeriods[period].TimeInVeryHighRecords

			destPeriods[period].HasTimeInVeryLowPercent = sourcePeriods[period].HasTimeInVeryLowPercent
			destPeriods[period].TimeInVeryLowPercent = sourcePeriods[period].TimeInVeryLowPercent

			destPeriods[period].HasTimeInVeryLowMinutes = sourcePeriods[period].HasTimeInVeryLowMinutes
			destPeriods[period].TimeInVeryLowMinutes = sourcePeriods[period].TimeInVeryLowMinutes

			destPeriods[period].HasTimeInVeryLowRecords = sourcePeriods[period].HasTimeInVeryLowRecords
			destPeriods[period].TimeInVeryLowRecords = sourcePeriods[period].TimeInVeryLowRecords

			destPeriods[period].HasAverageDailyRecords = sourcePeriods[period].HasAverageDailyRecords
			destPeriods[period].AverageDailyRecords = sourcePeriods[period].AverageDailyRecords

			destPeriods[period].HasTotalRecords = sourcePeriods[period].HasTotalRecords
			destPeriods[period].TotalRecords = sourcePeriods[period].TotalRecords

		}
	}

	if bgmStats.Periods != nil {
		patientUpdate.BgmStats.Periods = &clinics.PatientBGMPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*summaries.BGMPeriod{}
		destPeriods := map[string]*clinics.PatientBGMPeriod{}
		if bgmStats.Periods.N1d != nil {
			sourcePeriods["1d"] = bgmStats.Periods.N1d

			patientUpdate.BgmStats.Periods.N1d = &clinics.PatientBGMPeriod{}
			destPeriods["1d"] = patientUpdate.BgmStats.Periods.N1d
		}
		if bgmStats.Periods.N7d != nil {
			sourcePeriods["7d"] = bgmStats.Periods.N7d

			patientUpdate.BgmStats.Periods.N7d = &clinics.PatientBGMPeriod{}
			destPeriods["7d"] = patientUpdate.BgmStats.Periods.N7d
		}
		if bgmStats.Periods.N14d != nil {
			sourcePeriods["14d"] = bgmStats.Periods.N14d

			patientUpdate.BgmStats.Periods.N14d = &clinics.PatientBGMPeriod{}
			destPeriods["14d"] = patientUpdate.BgmStats.Periods.N14d
		}
		if bgmStats.Periods.N30d != nil {
			sourcePeriods["30d"] = bgmStats.Periods.N30d

			patientUpdate.BgmStats.Periods.N30d = &clinics.PatientBGMPeriod{}
			destPeriods["30d"] = patientUpdate.BgmStats.Periods.N30d
		}

		for period := range sourcePeriods {
			if sourcePeriods[period].AverageGlucose != nil {
				destPeriods[period].AverageGlucose = &clinics.AverageGlucose{
					Value: sourcePeriods[period].AverageGlucose.Value,
					Units: clinics.AverageGlucoseUnits(sourcePeriods[period].AverageGlucose.Units)}
			}
			destPeriods[period].HasAverageGlucose = sourcePeriods[period].HasAverageGlucose

			destPeriods[period].HasTimeInHighPercent = sourcePeriods[period].HasTimeInHighPercent
			destPeriods[period].TimeInHighPercent = sourcePeriods[period].TimeInHighPercent

			destPeriods[period].HasTimeInHighRecords = sourcePeriods[period].HasTimeInHighRecords
			destPeriods[period].TimeInHighRecords = sourcePeriods[period].TimeInHighRecords

			destPeriods[period].HasTimeInLowPercent = sourcePeriods[period].HasTimeInLowPercent
			destPeriods[period].TimeInLowPercent = sourcePeriods[period].TimeInLowPercent

			destPeriods[period].HasTimeInLowRecords = sourcePeriods[period].HasTimeInLowRecords
			destPeriods[period].TimeInLowRecords = sourcePeriods[period].TimeInLowRecords

			destPeriods[period].HasTimeInTargetPercent = sourcePeriods[period].HasTimeInTargetPercent
			destPeriods[period].TimeInTargetPercent = sourcePeriods[period].TimeInTargetPercent

			destPeriods[period].HasTimeInTargetRecords = sourcePeriods[period].HasTimeInTargetRecords
			destPeriods[period].TimeInTargetRecords = sourcePeriods[period].TimeInTargetRecords

			destPeriods[period].HasTimeInVeryHighPercent = sourcePeriods[period].HasTimeInVeryHighPercent
			destPeriods[period].TimeInVeryHighPercent = sourcePeriods[period].TimeInVeryHighPercent

			destPeriods[period].HasTimeInVeryHighRecords = sourcePeriods[period].HasTimeInVeryHighRecords
			destPeriods[period].TimeInVeryHighRecords = sourcePeriods[period].TimeInVeryHighRecords

			destPeriods[period].HasTimeInVeryLowPercent = sourcePeriods[period].HasTimeInVeryLowPercent
			destPeriods[period].TimeInVeryLowPercent = sourcePeriods[period].TimeInVeryLowPercent

			destPeriods[period].HasTimeInVeryLowRecords = sourcePeriods[period].HasTimeInVeryLowRecords
			destPeriods[period].TimeInVeryLowRecords = sourcePeriods[period].TimeInVeryLowRecords

			destPeriods[period].HasAverageDailyRecords = sourcePeriods[period].HasAverageDailyRecords
			destPeriods[period].AverageDailyRecords = sourcePeriods[period].AverageDailyRecords

			destPeriods[period].HasTotalRecords = sourcePeriods[period].HasTotalRecords
			destPeriods[period].TotalRecords = sourcePeriods[period].TotalRecords
		}
	}

	return patientUpdate, nil
}

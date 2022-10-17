package patients

import (
	clinics "github.com/tidepool-org/clinic/client"
	summaries "github.com/tidepool-org/go-common/clients/summary"
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

func CreateSummaryUpdateBody(summary *summaries.Summary) clinics.UpdatePatientSummaryJSONRequestBody {
	// summary doesn't exist, return empty body
	if summary == nil {
		return clinics.UpdatePatientSummaryJSONRequestBody{}
	}

	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{
		CgmSummary: &clinics.PatientCGMSummary{
			FirstData:         summary.CgmSummary.FirstData,
			HasLastUploadDate: summary.CgmSummary.HasLastUploadDate,
			LastData:          summary.CgmSummary.LastData,
			LastUpdatedDate:   summary.CgmSummary.LastUpdatedDate,
			LastUploadDate:    summary.CgmSummary.LastUploadDate,
			OutdatedSince:     summary.CgmSummary.OutdatedSince,
			TotalHours:        summary.CgmSummary.TotalHours,
		},
		BgmSummary: &clinics.PatientBGMSummary{
			FirstData:         summary.BgmSummary.FirstData,
			HasLastUploadDate: summary.BgmSummary.HasLastUploadDate,
			LastData:          summary.BgmSummary.LastData,
			LastUpdatedDate:   summary.BgmSummary.LastUpdatedDate,
			LastUploadDate:    summary.BgmSummary.LastUploadDate,
			OutdatedSince:     summary.BgmSummary.OutdatedSince,
			TotalHours:        summary.BgmSummary.TotalHours,
		},
		Config: &clinics.PatientSummaryConfig{
			HighGlucoseThreshold:     summary.Config.HighGlucoseThreshold,
			LowGlucoseThreshold:      summary.Config.LowGlucoseThreshold,
			SchemaVersion:            summary.Config.SchemaVersion,
			VeryHighGlucoseThreshold: summary.Config.VeryHighGlucoseThreshold,
			VeryLowGlucoseThreshold:  summary.Config.VeryLowGlucoseThreshold,
		},
	}

	if summary.CgmSummary.Periods != nil {
		patientUpdate.CgmSummary.Periods = &clinics.PatientCGMPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*summaries.CGMPeriod{}
		destPeriods := map[string]*clinics.PatientCGMPeriod{}
		if summary.CgmSummary.Periods.N1d != nil {
			sourcePeriods["1d"] = summary.CgmSummary.Periods.N1d

			patientUpdate.CgmSummary.Periods.N1d = &clinics.PatientCGMPeriod{}
			destPeriods["1d"] = patientUpdate.CgmSummary.Periods.N1d
		}
		if summary.CgmSummary.Periods.N7d != nil {
			sourcePeriods["7d"] = summary.CgmSummary.Periods.N7d

			patientUpdate.CgmSummary.Periods.N7d = &clinics.PatientCGMPeriod{}
			destPeriods["7d"] = patientUpdate.CgmSummary.Periods.N7d
		}
		if summary.CgmSummary.Periods.N14d != nil {
			sourcePeriods["14d"] = summary.CgmSummary.Periods.N14d

			patientUpdate.CgmSummary.Periods.N14d = &clinics.PatientCGMPeriod{}
			destPeriods["14d"] = patientUpdate.CgmSummary.Periods.N14d
		}
		if summary.CgmSummary.Periods.N30d != nil {
			sourcePeriods["30d"] = summary.CgmSummary.Periods.N30d

			patientUpdate.CgmSummary.Periods.N30d = &clinics.PatientCGMPeriod{}
			destPeriods["30d"] = patientUpdate.CgmSummary.Periods.N30d
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

			destPeriods[period].TimeCGMUseMinutes = sourcePeriods[period].TimeCGMUseMinutes
			destPeriods[period].TimeCGMUsePercent = sourcePeriods[period].TimeCGMUsePercent
			destPeriods[period].HasTimeCGMUsePercent = sourcePeriods[period].HasTimeCGMUsePercent
			destPeriods[period].TimeCGMUseRecords = sourcePeriods[period].TimeCGMUseRecords

			destPeriods[period].TimeInHighMinutes = sourcePeriods[period].TimeInHighMinutes
			destPeriods[period].TimeInHighPercent = sourcePeriods[period].TimeInHighPercent
			destPeriods[period].HasTimeInHighPercent = sourcePeriods[period].HasTimeInHighPercent
			destPeriods[period].TimeInHighRecords = sourcePeriods[period].TimeInHighRecords

			destPeriods[period].TimeInLowMinutes = sourcePeriods[period].TimeInLowMinutes
			destPeriods[period].TimeInLowPercent = sourcePeriods[period].TimeInLowPercent
			destPeriods[period].HasTimeInLowPercent = sourcePeriods[period].HasTimeInLowPercent
			destPeriods[period].TimeInLowRecords = sourcePeriods[period].TimeInLowRecords

			destPeriods[period].TimeInTargetMinutes = sourcePeriods[period].TimeInTargetMinutes
			destPeriods[period].TimeInTargetPercent = sourcePeriods[period].TimeInTargetPercent
			destPeriods[period].HasTimeInTargetPercent = sourcePeriods[period].HasTimeInTargetPercent
			destPeriods[period].TimeInTargetRecords = sourcePeriods[period].TimeInTargetRecords

			destPeriods[period].TimeInVeryHighMinutes = sourcePeriods[period].TimeInVeryHighMinutes
			destPeriods[period].TimeInVeryHighPercent = sourcePeriods[period].TimeInVeryHighPercent
			destPeriods[period].HasTimeInVeryHighPercent = sourcePeriods[period].HasTimeInVeryHighPercent
			destPeriods[period].TimeInVeryHighRecords = sourcePeriods[period].TimeInVeryHighRecords

			destPeriods[period].TimeInVeryLowMinutes = sourcePeriods[period].TimeInVeryLowMinutes
			destPeriods[period].TimeInVeryLowPercent = sourcePeriods[period].TimeInVeryLowPercent
			destPeriods[period].HasTimeInVeryLowPercent = sourcePeriods[period].HasTimeInVeryLowPercent
			destPeriods[period].TimeInVeryLowRecords = sourcePeriods[period].TimeInVeryLowRecords
		}
	}

	if summary.BgmSummary.Periods != nil {
		patientUpdate.BgmSummary.Periods = &clinics.PatientBGMPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*summaries.BGMPeriod{}
		destPeriods := map[string]*clinics.PatientBGMPeriod{}
		if summary.BgmSummary.Periods.N1d != nil {
			sourcePeriods["1d"] = summary.BgmSummary.Periods.N1d

			patientUpdate.BgmSummary.Periods.N1d = &clinics.PatientBGMPeriod{}
			destPeriods["1d"] = patientUpdate.BgmSummary.Periods.N1d
		}
		if summary.BgmSummary.Periods.N7d != nil {
			sourcePeriods["7d"] = summary.BgmSummary.Periods.N7d

			patientUpdate.BgmSummary.Periods.N7d = &clinics.PatientBGMPeriod{}
			destPeriods["7d"] = patientUpdate.BgmSummary.Periods.N7d
		}
		if summary.BgmSummary.Periods.N14d != nil {
			sourcePeriods["14d"] = summary.BgmSummary.Periods.N14d

			patientUpdate.BgmSummary.Periods.N14d = &clinics.PatientBGMPeriod{}
			destPeriods["14d"] = patientUpdate.BgmSummary.Periods.N14d
		}
		if summary.BgmSummary.Periods.N30d != nil {
			sourcePeriods["30d"] = summary.BgmSummary.Periods.N30d

			patientUpdate.BgmSummary.Periods.N30d = &clinics.PatientBGMPeriod{}
			destPeriods["30d"] = patientUpdate.BgmSummary.Periods.N30d
		}

		for period := range sourcePeriods {
			if sourcePeriods[period].AverageGlucose != nil {
				destPeriods[period].AverageGlucose = &clinics.AverageGlucose{
					Value: sourcePeriods[period].AverageGlucose.Value,
					Units: clinics.AverageGlucoseUnits(sourcePeriods[period].AverageGlucose.Units)}
			}
			destPeriods[period].HasAverageGlucose = sourcePeriods[period].HasAverageGlucose

			destPeriods[period].TimeInHighPercent = sourcePeriods[period].TimeInHighPercent
			destPeriods[period].HasTimeInHighPercent = sourcePeriods[period].HasTimeInHighPercent
			destPeriods[period].TimeInHighRecords = sourcePeriods[period].TimeInHighRecords

			destPeriods[period].TimeInLowPercent = sourcePeriods[period].TimeInLowPercent
			destPeriods[period].HasTimeInLowPercent = sourcePeriods[period].HasTimeInLowPercent
			destPeriods[period].TimeInLowRecords = sourcePeriods[period].TimeInLowRecords

			destPeriods[period].TimeInTargetPercent = sourcePeriods[period].TimeInTargetPercent
			destPeriods[period].HasTimeInTargetPercent = sourcePeriods[period].HasTimeInTargetPercent
			destPeriods[period].TimeInTargetRecords = sourcePeriods[period].TimeInTargetRecords

			destPeriods[period].TimeInVeryHighPercent = sourcePeriods[period].TimeInVeryHighPercent
			destPeriods[period].HasTimeInVeryHighPercent = sourcePeriods[period].HasTimeInVeryHighPercent
			destPeriods[period].TimeInVeryHighRecords = sourcePeriods[period].TimeInVeryHighRecords

			destPeriods[period].TimeInVeryLowPercent = sourcePeriods[period].TimeInVeryLowPercent
			destPeriods[period].HasTimeInVeryLowPercent = sourcePeriods[period].HasTimeInVeryLowPercent
			destPeriods[period].TimeInVeryLowRecords = sourcePeriods[period].TimeInVeryLowRecords
		}
	}

	return patientUpdate
}

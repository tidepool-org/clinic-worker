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
		FirstData:                summary.FirstData,
		LastData:                 summary.LastData,
		LastUpdatedDate:          summary.LastUpdatedDate,
		LastUploadDate:           summary.LastUploadDate,
		HasLastUploadDate:        summary.HasLastUploadDate,
		OutdatedSince:            summary.OutdatedSince,
		TotalHours:               summary.TotalHours,
		LowGlucoseThreshold:      summary.LowGlucoseThreshold,
		VeryLowGlucoseThreshold:  summary.VeryLowGlucoseThreshold,
		HighGlucoseThreshold:     summary.HighGlucoseThreshold,
		VeryHighGlucoseThreshold: summary.VeryHighGlucoseThreshold,
	}

	if summary.Periods != nil {
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{}

		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*summaries.SummaryPeriod{}
		destPeriods := map[string]*clinics.PatientSummaryPeriod{}
		if summary.Periods.N1d != nil {
			sourcePeriods["1d"] = summary.Periods.N1d

			patientUpdate.Periods.N1d = &clinics.PatientSummaryPeriod{}
			destPeriods["1d"] = patientUpdate.Periods.N1d
		}
		if summary.Periods.N7d != nil {
			sourcePeriods["7d"] = summary.Periods.N7d

			patientUpdate.Periods.N7d = &clinics.PatientSummaryPeriod{}
			destPeriods["7d"] = patientUpdate.Periods.N7d
		}
		if summary.Periods.N14d != nil {
			sourcePeriods["14d"] = summary.Periods.N14d

			patientUpdate.Periods.N14d = &clinics.PatientSummaryPeriod{}
			destPeriods["14d"] = patientUpdate.Periods.N14d
		}
		if summary.Periods.N30d != nil {
			sourcePeriods["30d"] = summary.Periods.N30d

			patientUpdate.Periods.N30d = &clinics.PatientSummaryPeriod{}
			destPeriods["30d"] = patientUpdate.Periods.N30d
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

	return patientUpdate
}

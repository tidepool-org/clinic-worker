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

		for i := range sourcePeriods {
			destPeriods[i].AverageGlucose = &clinics.AverageGlucose{
				Value: sourcePeriods[i].AvgGlucose.Value,
				Units: clinics.AverageGlucoseUnits(sourcePeriods[i].AvgGlucose.Units)}

			destPeriods[i].GlucoseManagementIndicator = sourcePeriods[i].GlucoseManagementIndicator
			destPeriods[i].HasGlucoseManagementIndicator = sourcePeriods[i].HasGlucoseManagementIndicator

			destPeriods[i].TimeCGMUseMinutes = sourcePeriods[i].TimeCGMUseMinutes
			destPeriods[i].TimeCGMUsePercent = sourcePeriods[i].TimeCGMUsePercent
			destPeriods[i].HasTimeCGMUsePercent = sourcePeriods[i].HasTimeCGMUsePercent
			destPeriods[i].TimeCGMUseRecords = sourcePeriods[i].TimeCGMUseRecords

			destPeriods[i].TimeInHighMinutes = sourcePeriods[i].TimeInHighMinutes
			destPeriods[i].TimeInHighPercent = sourcePeriods[i].TimeInHighPercent
			destPeriods[i].TimeInHighRecords = sourcePeriods[i].TimeInHighRecords

			destPeriods[i].TimeInLowMinutes = sourcePeriods[i].TimeInLowMinutes
			destPeriods[i].TimeInLowPercent = sourcePeriods[i].TimeInLowPercent
			destPeriods[i].TimeInLowRecords = sourcePeriods[i].TimeInLowRecords

			destPeriods[i].TimeInTargetMinutes = sourcePeriods[i].TimeInTargetMinutes
			destPeriods[i].TimeInTargetPercent = sourcePeriods[i].TimeInTargetPercent
			destPeriods[i].TimeInTargetRecords = sourcePeriods[i].TimeInTargetRecords

			destPeriods[i].TimeInVeryHighMinutes = sourcePeriods[i].TimeInVeryHighMinutes
			destPeriods[i].TimeInVeryHighPercent = sourcePeriods[i].TimeInVeryHighPercent
			destPeriods[i].TimeInVeryHighRecords = sourcePeriods[i].TimeInVeryHighRecords

			destPeriods[i].TimeInVeryLowMinutes = sourcePeriods[i].TimeInVeryLowMinutes
			destPeriods[i].TimeInVeryLowPercent = sourcePeriods[i].TimeInVeryLowPercent
			destPeriods[i].TimeInVeryLowRecords = sourcePeriods[i].TimeInVeryLowRecords
		}
	}

	return patientUpdate
}

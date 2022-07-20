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
		OutdatedSince:            summary.OutdatedSince,
		TotalHours:               summary.TotalHours,
		LowGlucoseThreshold:      summary.LowGlucoseThreshold,
		VeryLowGlucoseThreshold:  summary.VeryLowGlucoseThreshold,
		HighGlucoseThreshold:     summary.HighGlucoseThreshold,
		VeryHighGlucoseThreshold: summary.VeryHighGlucoseThreshold,
	}

	if summary.Periods != nil {
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{}

		if summary.Periods.N1d != nil {
			patientUpdate.Periods.N1d = &clinics.PatientSummaryPeriod{
				AverageGlucose: &clinics.AverageGlucose{Value: summary.Periods.N1d.AvgGlucose.Value,
					Units: clinics.AverageGlucoseUnits(summary.Periods.N1d.AvgGlucose.Units)},
				GlucoseManagementIndicator: summary.Periods.N1d.GlucoseManagementIndicator,

				TimeCGMUseMinutes: summary.Periods.N1d.TimeCGMUseMinutes,
				TimeCGMUsePercent: summary.Periods.N1d.TimeCGMUsePercent,
				TimeCGMUseRecords: summary.Periods.N1d.TimeCGMUseRecords,

				TimeInHighMinutes: summary.Periods.N1d.TimeInHighMinutes,
				TimeInHighPercent: summary.Periods.N1d.TimeInHighPercent,
				TimeInHighRecords: summary.Periods.N1d.TimeInHighRecords,

				TimeInLowMinutes: summary.Periods.N1d.TimeInLowMinutes,
				TimeInLowPercent: summary.Periods.N1d.TimeInLowPercent,
				TimeInLowRecords: summary.Periods.N1d.TimeInLowRecords,

				TimeInTargetMinutes: summary.Periods.N1d.TimeInTargetMinutes,
				TimeInTargetPercent: summary.Periods.N1d.TimeInTargetPercent,
				TimeInTargetRecords: summary.Periods.N1d.TimeInTargetRecords,

				TimeInVeryHighMinutes: summary.Periods.N1d.TimeInVeryHighMinutes,
				TimeInVeryHighPercent: summary.Periods.N1d.TimeInVeryHighPercent,
				TimeInVeryHighRecords: summary.Periods.N1d.TimeInVeryHighRecords,

				TimeInVeryLowMinutes: summary.Periods.N1d.TimeInVeryLowMinutes,
				TimeInVeryLowPercent: summary.Periods.N1d.TimeInVeryLowPercent,
				TimeInVeryLowRecords: summary.Periods.N1d.TimeInVeryLowRecords,
			}
		}

		if summary.Periods.N7d != nil {
			patientUpdate.Periods.N7d = &clinics.PatientSummaryPeriod{
				AverageGlucose: &clinics.AverageGlucose{Value: summary.Periods.N7d.AvgGlucose.Value,
					Units: clinics.AverageGlucoseUnits(summary.Periods.N7d.AvgGlucose.Units)},
				GlucoseManagementIndicator: summary.Periods.N7d.GlucoseManagementIndicator,

				TimeCGMUseMinutes: summary.Periods.N7d.TimeCGMUseMinutes,
				TimeCGMUsePercent: summary.Periods.N7d.TimeCGMUsePercent,
				TimeCGMUseRecords: summary.Periods.N7d.TimeCGMUseRecords,

				TimeInHighMinutes: summary.Periods.N7d.TimeInHighMinutes,
				TimeInHighPercent: summary.Periods.N7d.TimeInHighPercent,
				TimeInHighRecords: summary.Periods.N7d.TimeInHighRecords,

				TimeInLowMinutes: summary.Periods.N7d.TimeInLowMinutes,
				TimeInLowPercent: summary.Periods.N7d.TimeInLowPercent,
				TimeInLowRecords: summary.Periods.N7d.TimeInLowRecords,

				TimeInTargetMinutes: summary.Periods.N7d.TimeInTargetMinutes,
				TimeInTargetPercent: summary.Periods.N7d.TimeInTargetPercent,
				TimeInTargetRecords: summary.Periods.N7d.TimeInTargetRecords,

				TimeInVeryHighMinutes: summary.Periods.N7d.TimeInVeryHighMinutes,
				TimeInVeryHighPercent: summary.Periods.N7d.TimeInVeryHighPercent,
				TimeInVeryHighRecords: summary.Periods.N7d.TimeInVeryHighRecords,

				TimeInVeryLowMinutes: summary.Periods.N7d.TimeInVeryLowMinutes,
				TimeInVeryLowPercent: summary.Periods.N7d.TimeInVeryLowPercent,
				TimeInVeryLowRecords: summary.Periods.N7d.TimeInVeryLowRecords,
			}
		}

		if summary.Periods.N14d != nil {
			patientUpdate.Periods.N14d = &clinics.PatientSummaryPeriod{
				AverageGlucose: &clinics.AverageGlucose{Value: summary.Periods.N14d.AvgGlucose.Value,
					Units: clinics.AverageGlucoseUnits(summary.Periods.N14d.AvgGlucose.Units)},
				GlucoseManagementIndicator: summary.Periods.N14d.GlucoseManagementIndicator,

				TimeCGMUseMinutes: summary.Periods.N14d.TimeCGMUseMinutes,
				TimeCGMUsePercent: summary.Periods.N14d.TimeCGMUsePercent,
				TimeCGMUseRecords: summary.Periods.N14d.TimeCGMUseRecords,

				TimeInHighMinutes: summary.Periods.N14d.TimeInHighMinutes,
				TimeInHighPercent: summary.Periods.N14d.TimeInHighPercent,
				TimeInHighRecords: summary.Periods.N14d.TimeInHighRecords,

				TimeInLowMinutes: summary.Periods.N14d.TimeInLowMinutes,
				TimeInLowPercent: summary.Periods.N14d.TimeInLowPercent,
				TimeInLowRecords: summary.Periods.N14d.TimeInLowRecords,

				TimeInTargetMinutes: summary.Periods.N14d.TimeInTargetMinutes,
				TimeInTargetPercent: summary.Periods.N14d.TimeInTargetPercent,
				TimeInTargetRecords: summary.Periods.N14d.TimeInTargetRecords,

				TimeInVeryHighMinutes: summary.Periods.N14d.TimeInVeryHighMinutes,
				TimeInVeryHighPercent: summary.Periods.N14d.TimeInVeryHighPercent,
				TimeInVeryHighRecords: summary.Periods.N14d.TimeInVeryHighRecords,

				TimeInVeryLowMinutes: summary.Periods.N14d.TimeInVeryLowMinutes,
				TimeInVeryLowPercent: summary.Periods.N14d.TimeInVeryLowPercent,
				TimeInVeryLowRecords: summary.Periods.N14d.TimeInVeryLowRecords,
			}
		}

		if summary.Periods.N30d != nil {
			patientUpdate.Periods.N30d = &clinics.PatientSummaryPeriod{
				AverageGlucose: &clinics.AverageGlucose{Value: summary.Periods.N30d.AvgGlucose.Value,
					Units: clinics.AverageGlucoseUnits(summary.Periods.N30d.AvgGlucose.Units)},
				GlucoseManagementIndicator: summary.Periods.N30d.GlucoseManagementIndicator,

				TimeCGMUseMinutes: summary.Periods.N30d.TimeCGMUseMinutes,
				TimeCGMUsePercent: summary.Periods.N30d.TimeCGMUsePercent,
				TimeCGMUseRecords: summary.Periods.N30d.TimeCGMUseRecords,

				TimeInHighMinutes: summary.Periods.N30d.TimeInHighMinutes,
				TimeInHighPercent: summary.Periods.N30d.TimeInHighPercent,
				TimeInHighRecords: summary.Periods.N30d.TimeInHighRecords,

				TimeInLowMinutes: summary.Periods.N30d.TimeInLowMinutes,
				TimeInLowPercent: summary.Periods.N30d.TimeInLowPercent,
				TimeInLowRecords: summary.Periods.N30d.TimeInLowRecords,

				TimeInTargetMinutes: summary.Periods.N30d.TimeInTargetMinutes,
				TimeInTargetPercent: summary.Periods.N30d.TimeInTargetPercent,
				TimeInTargetRecords: summary.Periods.N30d.TimeInTargetRecords,

				TimeInVeryHighMinutes: summary.Periods.N30d.TimeInVeryHighMinutes,
				TimeInVeryHighPercent: summary.Periods.N30d.TimeInVeryHighPercent,
				TimeInVeryHighRecords: summary.Periods.N30d.TimeInVeryHighRecords,

				TimeInVeryLowMinutes: summary.Periods.N30d.TimeInVeryLowMinutes,
				TimeInVeryLowPercent: summary.Periods.N30d.TimeInVeryLowPercent,
				TimeInVeryLowRecords: summary.Periods.N30d.TimeInVeryLowRecords,
			}
		}
	}

	return patientUpdate
}

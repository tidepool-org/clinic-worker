package patients

import (
	"fmt"
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
	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{
		FirstData:                summary.FirstData,
		LastData:                 summary.LastData,
		LastUpdatedDate:          summary.LastUpdatedDate,
		LastUploadDate:           summary.LastUploadDate,
		OutdatedSince:            summary.OutdatedSince,
		TotalDays:                summary.TotalDays,
		LowGlucoseThreshold:      summary.LowGlucoseThreshold,
		VeryLowGlucoseThreshold:  summary.VeryLowGlucoseThreshold,
		HighGlucoseThreshold:     summary.HighGlucoseThreshold,
		VeryHighGlucoseThreshold: summary.VeryHighGlucoseThreshold,
	}

	var periodExists = false
	var period14dExists = false
	if summary.Periods != nil {
		periodExists = true
		if summary.Periods.N14d != nil {
			period14dExists = true
		}
	}

	if periodExists && period14dExists {
		fmt.Println("summary", summary)
		fmt.Println("periods", summary.Periods)
		fmt.Println("periods 14", summary.Periods.N14d)
		fmt.Println("periods 14 glucose", summary.Periods.N14d.AvgGlucose)
		fmt.Println("periods 14 glucose value", summary.Periods.N14d.AvgGlucose.Value)
		patientUpdate.Periods = &clinics.PatientSummaryPeriods{N14d: &clinics.PatientSummaryPeriod{
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
		}}
	}
	return patientUpdate
}

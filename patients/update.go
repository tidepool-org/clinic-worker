package patients

import (
	"fmt"
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
	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{}

	if cgmSummary != nil {
		patientUpdate.CgmStats = &clinics.PatientCGMStats{
			Dates:         (*clinics.PatientSummaryDates)(cgmSummary.Dates),
			Config:        (*clinics.PatientSummaryConfig)(cgmSummary.Config),
			Periods:       &clinics.PatientCGMPeriods{},
			OffsetPeriods: &clinics.PatientCGMPeriods{},
		}

		if cgmSummary.Stats != nil {
			cgmStats, err := cgmSummary.Stats.AsCGMStats()
			if err != nil {
				return clinics.UpdatePatientSummaryJSONRequestBody{}, errors.Wrapf(err, "unable to unserialize CGM summary stats for userId %s", *cgmSummary.UserId)
			}
			patientUpdate.CgmStats.TotalHours = cgmStats.TotalHours

			if cgmStats.Periods != nil {
				for k, source := range *cgmStats.Periods {
					fmt.Println("converting cgm period:", k)
					(*patientUpdate.CgmStats.Periods)[k] = clinics.PatientCGMPeriod(source)
				}
			}

			if cgmStats.OffsetPeriods != nil {
				for k, source := range *cgmStats.OffsetPeriods {
					fmt.Println("converting cgm offsetperiod:", k)
					(*patientUpdate.CgmStats.OffsetPeriods)[k] = clinics.PatientCGMPeriod(source)
				}
			}
		}
	}

	if bgmSummary != nil {
		patientUpdate.BgmStats = &clinics.PatientBGMStats{
			Dates:         (*clinics.PatientSummaryDates)(bgmSummary.Dates),
			Config:        (*clinics.PatientSummaryConfig)(bgmSummary.Config),
			Periods:       &clinics.PatientBGMPeriods{},
			OffsetPeriods: &clinics.PatientBGMPeriods{},
		}

		if bgmSummary.Stats != nil {
			bgmStats, err := bgmSummary.Stats.AsBGMStats()
			if err != nil {
				return clinics.UpdatePatientSummaryJSONRequestBody{}, errors.Wrapf(err, "unable to unserialize BGM summary stats for userId %s", *bgmSummary.UserId)
			}
			patientUpdate.BgmStats.TotalHours = bgmStats.TotalHours

			if bgmStats.Periods != nil {
				for k, source := range *bgmStats.Periods {
					fmt.Println("converting bgm period:", k)
					(*patientUpdate.BgmStats.Periods)[k] = clinics.PatientBGMPeriod(source)
				}
			}

			if bgmStats.OffsetPeriods != nil {
				for k, source := range *bgmStats.OffsetPeriods {
					fmt.Println("converting bgm offsetperiod:", k)
					(*patientUpdate.BgmStats.OffsetPeriods)[k] = clinics.PatientBGMPeriod(source)
				}
			}
		}
	}

	return patientUpdate, nil
}

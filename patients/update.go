package patients

import (
	"regexp"
	"strconv"
	"time"

	"github.com/tidepool-org/clinic-worker/patientsummary"
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
	if patient.DiagnosisType != nil {
		patientProfile["diagnosisType"] = *patient.DiagnosisType
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
	if _, ok := removedFieldsMap["diagnosisType"]; ok {
		delete(patientProfile, "diagnosisType")
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

func CreateSummaryUpdateBody(cgmSummary *summaries.SummaryV5, bgmSummary *summaries.SummaryV5) (clinics.UpdatePatientSummaryJSONRequestBody, error) {
	patientUpdate := clinics.UpdatePatientSummaryJSONRequestBody{}

	if cgmSummary != nil {
		var firstData *time.Time
		if !cgmSummary.Dates.FirstData.IsZero() {
			firstData = &cgmSummary.Dates.FirstData
		}

		var lastData *time.Time
		if !cgmSummary.Dates.LastData.IsZero() {
			lastData = &cgmSummary.Dates.LastData
		}

		var lastUpdatedDate *time.Time
		if !cgmSummary.Dates.LastUpdatedDate.IsZero() {
			lastUpdatedDate = &cgmSummary.Dates.LastUpdatedDate
		}

		var lastUploadDate *time.Time
		if !cgmSummary.Dates.LastUpdatedDate.IsZero() {
			lastUploadDate = &cgmSummary.Dates.LastUpdatedDate
		}

		if cgmSummary.Dates.OutdatedReason == nil {
			cgmSummary.Dates.OutdatedReason = []string{}
		}

		if cgmSummary.Dates.LastUpdatedReason == nil {
			cgmSummary.Dates.LastUpdatedReason = []string{}
		}

		patientUpdate.CgmStats = &clinics.CgmStatsV1{
			Id: cgmSummary.Id,
			Dates: clinics.SummaryDatesV1{
				LastUpdatedDate:   lastUpdatedDate,
				LastUpdatedReason: &cgmSummary.Dates.LastUpdatedReason,
				OutdatedReason:    &cgmSummary.Dates.OutdatedReason,
				HasLastUploadDate: lastUploadDate != nil,
				LastUploadDate:    lastUploadDate,
				HasFirstData:      firstData != nil,
				FirstData:         firstData,
				HasLastData:       lastData != nil,
				LastData:          lastData,
				HasOutdatedSince:  cgmSummary.Dates.OutdatedSince != nil,
				OutdatedSince:     cgmSummary.Dates.OutdatedSince,
			},
			Config: clinics.SummaryConfigV1(cgmSummary.Config),
		}

		if cgmSummary.Periods != nil {
			cgmPeriods, err := cgmSummary.Periods.AsCgmPeriodsV5()
			if err != nil {
				return clinics.UpdatePatientSummaryJSONRequestBody{}, errors.Wrapf(err, "unable to unserialize CGM summary stats for userId %s", *cgmSummary.UserId)
			}

			daysRe := regexp.MustCompile("(\\d+)d")
			patientUpdate.CgmStats.Periods = clinics.CgmPeriodsV1{}
			for k := range cgmPeriods {
				// get integer portion of 1d/7d/14d/30d map string
				m := daysRe.FindStringSubmatch(k)
				if len(m) >= 2 {
					i, _ := strconv.Atoi(m[1])
					patientUpdate.CgmStats.Periods[k] = patientsummary.ExportCGMPeriod(cgmPeriods[k], i)
				}
			}
		}
	}

	if bgmSummary != nil {
		var firstData *time.Time
		if !bgmSummary.Dates.FirstData.IsZero() {
			firstData = &bgmSummary.Dates.FirstData
		}

		var lastData *time.Time
		if !bgmSummary.Dates.LastData.IsZero() {
			lastData = &bgmSummary.Dates.LastData
		}

		var lastUpdatedDate *time.Time
		if !bgmSummary.Dates.LastUpdatedDate.IsZero() {
			lastUpdatedDate = &bgmSummary.Dates.LastUpdatedDate
		}

		var lastUploadDate *time.Time
		if !bgmSummary.Dates.LastUpdatedDate.IsZero() {
			lastUploadDate = &bgmSummary.Dates.LastUpdatedDate
		}

		if bgmSummary.Dates.OutdatedReason == nil {
			bgmSummary.Dates.OutdatedReason = []string{}
		}

		if bgmSummary.Dates.LastUpdatedReason == nil {
			bgmSummary.Dates.LastUpdatedReason = []string{}
		}

		patientUpdate.BgmStats = &clinics.BgmStatsV1{
			Id: bgmSummary.Id,
			Dates: clinics.SummaryDatesV1{
				LastUpdatedDate:   lastUpdatedDate,
				LastUpdatedReason: &bgmSummary.Dates.LastUpdatedReason,
				OutdatedReason:    &bgmSummary.Dates.OutdatedReason,
				HasLastUploadDate: lastUploadDate != nil,
				LastUploadDate:    lastUploadDate,
				HasFirstData:      firstData != nil,
				FirstData:         firstData,
				HasLastData:       lastData != nil,
				LastData:          lastData,
				HasOutdatedSince:  bgmSummary.Dates.OutdatedSince != nil,
				OutdatedSince:     bgmSummary.Dates.OutdatedSince,
			},
			Config: clinics.SummaryConfigV1(bgmSummary.Config),
		}

		if bgmSummary.Periods != nil {
			bgmPeriods, err := bgmSummary.Periods.AsBgmPeriodsV5()
			if err != nil {
				return clinics.UpdatePatientSummaryJSONRequestBody{}, errors.Wrapf(err, "unable to unserialize BGM summary stats for userId %s", *bgmSummary.UserId)
			}

			daysRe := regexp.MustCompile("(\\d+)d")
			patientUpdate.BgmStats.Periods = clinics.BgmPeriodsV1{}
			for k := range bgmPeriods {
				m := daysRe.FindStringSubmatch(k)
				if len(m) >= 2 {
					patientUpdate.BgmStats.Periods[k] = patientsummary.ExportBGMPeriod(bgmPeriods[k])
				}
			}

		}
	}

	return patientUpdate, nil
}

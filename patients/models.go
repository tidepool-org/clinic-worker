package patients

import (
	summaries "github.com/tidepool-org/go-common/clients/summary"
	"time"

	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/patientsummary"
	api "github.com/tidepool-org/clinic/client"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
)

type PatientCDCEvent struct {
	Offset            int64             `json:"-"`
	FullDocument      Patient           `json:"fullDocument"`
	OperationType     string            `json:"operationType"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

func (p PatientCDCEvent) IsUploadReminderEvent() bool {
	if p.OperationType != cdc.OperationTypeUpdate && p.OperationType != cdc.OperationTypeReplace {
		return false
	}
	if p.FullDocument.UserId == nil {
		return false
	}

	lastUploadReminderTime := p.UpdateDescription.UpdatedFields.LastUploadReminderTime
	return lastUploadReminderTime != nil && lastUploadReminderTime.Value > 0
}

func (p PatientCDCEvent) IsRequestDexcomConnectEvent() bool {
	if p.OperationType != cdc.OperationTypeUpdate && p.OperationType != cdc.OperationTypeReplace {
		return false
	}
	if p.FullDocument.UserId == nil || p.UpdateDescription.UpdatedFields.LastRequestedDexcomConnectTime == nil {
		return false
	}
	return p.UpdateDescription.UpdatedFields.LastRequestedDexcomConnectTime.Value > 0
}

func (p PatientCDCEvent) IsProfileUpdateEvent() bool {
	if p.OperationType != cdc.OperationTypeInsert && p.OperationType != cdc.OperationTypeUpdate && p.OperationType != cdc.OperationTypeReplace {
		return false
	}
	if p.FullDocument.UserId == nil {
		return false
	}
	// We want to apply profile updates and send invites only to custodial accounts.
	return p.FullDocument.IsCustodial()
}

func (p PatientCDCEvent) IsPatientCreateFromExistingUserEvent() bool {
	return p.OperationType == cdc.OperationTypeInsert && !p.FullDocument.IsCustodial()
}

func (p PatientCDCEvent) PatientHasPendingDexcomConnection() bool {
	if p.FullDocument.DataSources != nil {
		for _, dataSource := range *p.FullDocument.DataSources {
			if *dataSource.ProviderName == DexcomDataSourceProviderName && *dataSource.State == string(clinics.DataSourceStatePending) {
				return true
			}
		}
	}
	return false
}

func (p PatientCDCEvent) PatientNeedsSummary() bool {
	if p.OperationType != cdc.OperationTypeInsert {
		return false
	}

	if p.FullDocument.UserId == nil {
		return false
	}

	// return true if summary is missing
	return p.FullDocument.Summary == nil
}

func (p PatientCDCEvent) ApplyUpdatesToExistingProfile(profile map[string]interface{}) {
	switch p.OperationType {
	case cdc.OperationTypeInsert, cdc.OperationTypeReplace:
		ApplyPatientChangesToProfile(p.FullDocument, profile)
	case cdc.OperationTypeUpdate:
		p.UpdateDescription.applyUpdatesToExistingProfile(profile)
	}
}

type BGMStats struct {
	Config     summaries.ConfigV1                    `json:"config" bson:"config"`
	Dates      patientsummary.Dates                  `json:"dates" bson:"dates"`
	Periods    map[string]*summaries.GlucoseperiodV5 `json:"periods" bson:"periods"`
	TotalHours int                                   `json:"totalHours" bson:"totalHours"`
}

type CGMStats struct {
	Config     summaries.ConfigV1                    `json:"config" bson:"config"`
	Dates      patientsummary.Dates                  `json:"dates" bson:"dates"`
	Periods    map[string]*summaries.GlucoseperiodV5 `json:"periods" bson:"periods"`
	TotalHours int                                   `json:"totalHours" bson:"totalHours"`
}

type CDCSummary struct {
	CGM *CGMStats `json:"cgmStats" bson:"cgmStats"`
	BGM *BGMStats `json:"bgmStats" bson:"bgmStats"`
}

type Patient struct {
	Id                             *cdc.ObjectId        `json:"_id" bson:"_id"`
	ClinicId                       *cdc.ObjectId        `json:"clinicId" bson:"clinicId"`
	UserId                         *string              `json:"userId" bson:"userId"`
	BirthDate                      *string              `json:"birthDate" bson:"birthDate"`
	Email                          *string              `json:"email" bson:"email"`
	FullName                       *string              `json:"fullName" bson:"fullName"`
	Mrn                            *string              `json:"mrn" bson:"mrn"`
	TargetDevices                  *[]string            `json:"targetDevices" bson:"targetDevices"`
	DataSources                    *[]PatientDataSource `json:"dataSources" bson:"dataSources"`
	Permissions                    *Permissions         `json:"permissions" bson:"permissions"`
	IsMigrated                     bool                 `json:"isMigrated" bson:"isMigrated"`
	InvitedBy                      *string              `json:"invitedBy" bson:"invitedBy"`
	LastRequestedDexcomConnectTime *cdc.Date            `json:"lastRequestedDexcomConnectTime" bson:"lastRequestedDexcomConnectTime"`
	LastUploadReminderTime         *cdc.Date            `json:"lastUploadReminderTime" bson:"lastUploadReminderTime"`
	Summary                        *CDCSummary          `json:"summary" bson:"summary"`
}

func (p PatientCDCEvent) CreateDataSourceBody(source clients.DataSource) clinics.DataSource {
	dataSource := clinics.DataSource{
		ProviderName: *source.ProviderName,
		State:        api.DataSourceState(*source.State),
	}

	if source.ModifiedTime != nil {
		modifiedTimeVal := clinics.DateTime(source.ModifiedTime.Format(time.RFC3339))
		dataSource.ModifiedTime = &modifiedTimeVal
	}

	return dataSource
}

type PatientDataSource struct {
	DataSourceId *cdc.ObjectId `json:"dataSourceId,omitempty"`
	ModifiedTime *cdc.Date     `json:"modifiedTime,omitempty"`
	ProviderName *string       `json:"providerName"`
	State        *string       `json:"state"`
}

func (p Patient) IsCustodial() bool {
	return p.Permissions != nil && p.Permissions.Custodian != nil
}

type Permissions struct {
	Custodian *Permission `json:"custodian"`
}

type Permission map[string]interface{}

type UpdateDescription struct {
	UpdatedFields UpdatedFields `json:"updatedFields"`
	RemovedFields []string      `json:"removedFields"`
}

func (u UpdateDescription) applyUpdatesToExistingProfile(profile map[string]interface{}) {
	ApplyPatientChangesToProfile(u.UpdatedFields.Patient, profile)
	RemoveFieldsFromProfile(u.RemovedFields, profile)
}

type UpdatedFields struct {
	Patient
}

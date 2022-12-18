package patients

import (
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/patientsummary"
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

func (p PatientCDCEvent) PatientHasPendingDexcomConnection() bool {
	for _, dataSource := range *p.FullDocument.DataSources {
		if *dataSource.ProviderName == "dexcom" && *dataSource.State == "pending" {
			return true
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

type PatientDataSource struct {
	DataSourceId *cdc.ObjectId `json:"dataSourceId,omitempty"`
	ModifiedTime *cdc.Date     `json:"modifiedTime,omitempty"`
	ProviderName *string       `json:"providerName"`
	State        *string       `json:"state"`
}

type Patient struct {
	Id                             *cdc.ObjectId           `json:"_id"`
	ClinicId                       *cdc.ObjectId           `json:"clinicId"`
	UserId                         *string                 `json:"userId"`
	BirthDate                      *string                 `json:"birthDate"`
	Email                          *string                 `json:"email"`
	FullName                       *string                 `json:"fullName"`
	Mrn                            *string                 `json:"mrn"`
	TargetDevices                  *[]string               `json:"targetDevices"`
	DataSources                    *[]PatientDataSource    `json:"dataSources"`
	Permissions                    *Permissions            `json:"permissions"`
	IsMigrated                     bool                    `json:"isMigrated"`
	InvitedBy                      *string                 `json:"invitedBy"`
	LastRequestedDexcomConnectTime *cdc.Date               `json:"lastRequestedDexcomConnectTime"`
	LastUploadReminderTime         *cdc.Date               `json:"lastUploadReminderTime"`
	Summary                        *patientsummary.Summary `json:"summary"`
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

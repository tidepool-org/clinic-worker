package patients

import (
	"fmt"
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

func (p PatientCDCEvent) PatientNeedsSummary() bool {
	fmt.Println("operation type", p.OperationType, cdc.OperationTypeInsert)
	fmt.Println("userid", p.FullDocument.UserId)
	fmt.Println("summary", p.FullDocument.Summary)
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

type Patient struct {
	Id                     *cdc.ObjectId           `json:"_id"`
	ClinicId               *cdc.ObjectId           `json:"clinicId"`
	UserId                 *string                 `json:"userId"`
	BirthDate              *string                 `json:"birthDate"`
	Email                  *string                 `json:"email"`
	FullName               *string                 `json:"fullName"`
	Mrn                    *string                 `json:"mrn"`
	TargetDevices          *[]string               `json:"targetDevices"`
	Permissions            *Permissions            `json:"permissions"`
	IsMigrated             bool                    `json:"isMigrated"`
	InvitedBy              *string                 `json:"invitedBy"`
	LastUploadReminderTime *cdc.Date               `json:"lastUploadReminderTime"`
	Summary                *patientsummary.Summary `json:"summary"`
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

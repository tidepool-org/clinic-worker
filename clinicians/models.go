package clinicians

import "github.com/tidepool-org/clinic-worker/cdc"

type PatientCDCEvent struct {
	Offset            int64             `json:"-"`
	OperationType     string            `json:"operationType"`
	FullDocument      *Clinician        `json:"fullDocument"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

func (p PatientCDCEvent) ShouldApplyUpdates() bool {
	return (p.OperationType == cdc.OperationTypeUpdate || p.OperationType == cdc.OperationTypeInsert || p.OperationType == cdc.OperationTypeReplace) &&
		p.FullDocument != nil &&
		p.FullDocument.ClinicId != nil && p.FullDocument.UserId != ""
}

// ShouldBackfillSecurityProfile reports whether the event marks a clinician record gaining a
// user — the moment to backfill the clinician's security profile from the user's current
// keycloak state. That is an insert carrying a userId (admin or service-account creation), or
// an update that sets userId (invite acceptance updates the invite record in place).
func (p PatientCDCEvent) ShouldBackfillSecurityProfile() bool {
	if p.FullDocument == nil || p.FullDocument.UserId == "" {
		return false
	}
	switch p.OperationType {
	case cdc.OperationTypeInsert:
		return true
	case cdc.OperationTypeUpdate:
		return p.UpdateDescription.UpdatedFields.UserId != ""
	default:
		return false
	}
}

type Clinician struct {
	Id           *cdc.ObjectId `json:"_id" bson:"id"`
	ClinicId     *cdc.ObjectId `json:"clinicId" bson:"clinicId"`
	UserId       string        `json:"userId" bson:"userId"`
	RolesUpdates []RolesUpdate `json:"rolesUpdates" bson:"rolesUpdates"`
}

type RolesUpdate struct {
	UpdatedBy string   `json:"updatedBy" bson:"updatedBy"`
	Roles     []string `json:"roles" bson:"roles"`
}

type UpdateDescription struct {
	UpdatedFields UpdatedFields `json:"updatedFields"`
	RemovedFields []string      `json:"removedFields"`
}

type UpdatedFields struct {
	Clinician
}

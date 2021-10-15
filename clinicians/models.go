package clinicians

import "github.com/tidepool-org/clinic-worker/cdc"

type PatientCDCEvent struct {
	Offset            int64             `json:"-"`
	OperationType     string            `json:"operationType"`
	FullDocument      Clinician         `json:"fullDocument"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

func (p PatientCDCEvent) ShouldApplyUpdates() bool {
	return p.OperationType == cdc.OperationTypeUpdate
}

type Clinician struct {
	Id           *cdc.ObjectId `json:"_id"`
	ClinicId     *cdc.ObjectId `json:"clinicId"`
	UserId       string        `json:"userId"`
	RolesUpdates []RolesUpdate `json:"rolesUpdates"`
}

type RolesUpdate struct {
	UpdatedBy string   `json:"updatedBy"`
	Roles     []string `json:"roles"`
}

type UpdateDescription struct {
	UpdatedFields UpdatedFields `json:"updatedFields"`
	RemovedFields []string      `json:"removedFields"`
}

type UpdatedFields struct {
	Clinician
}

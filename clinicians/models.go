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

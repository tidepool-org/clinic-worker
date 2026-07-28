package patientdeletions

import (
	"github.com/tidepool-org/clinic-worker/cdc"
)

type PatientDeletionsCDCEvent struct {
	Offset            int64             `json:"-"`
	OperationType     string            `json:"operationType"`
	FullDocument      PatientDeletion   `json:"fullDocument"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

type Permissions struct {
	Custodian *Permission `json:"custodian"`
}

type Permission map[string]interface{}

type Patient struct {
	ClinicId    *cdc.ObjectId `json:"clinicId" bson:"clinicId"`
	UserId      string        `json:"userId" bson:"userId"`
	Permissions Permissions   `json:"permissions" bson:"permissions"`
}

type PatientDeletion struct {
	Id      *cdc.ObjectId `json:"_id" bson:"_id"`
	Patient Patient       `json:"patient" bson:"patient"`
}

type UpdateDescription struct {
	UpdatedFields UpdatedFields `json:"updatedFields"`
	RemovedFields []string      `json:"removedFields"`
}

type UpdatedFields struct {
	PatientDeletion
}

func (p PatientDeletion) IsCustodial() bool {
	return p.Patient.Permissions.Custodian != nil && p.Patient.UserId != ""
}

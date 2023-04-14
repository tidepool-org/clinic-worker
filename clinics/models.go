package clinics

import (
	"github.com/tidepool-org/clinic-worker/cdc"
)

type ClinicCDCEvent struct {
	Offset            int64             `json:"-"`
	OperationType     string            `json:"operationType"`
	FullDocument      Clinic            `json:"fullDocument"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

func (p ClinicCDCEvent) ShouldApplyUpdates() bool {
	if p.OperationType == cdc.OperationTypeInsert && len(p.FullDocument.Name) > 0 && len(p.FullDocument.Admins) > 0 {
		return true
	}

	if p.OperationType == cdc.OperationTypeUpdate && p.isPatientTagDelete() {
		return true
	}

	return false
}

func (p ClinicCDCEvent) isPatientTagDelete() bool {
	lastDeletedPatientTag := p.UpdateDescription.UpdatedFields.LastDeletedPatientTag
	return lastDeletedPatientTag != nil
}

type Clinic struct {
	Id                    *cdc.ObjectId `json:"_id"`
	Name                  string        `json:"name"`
	Admins                []string      `json:"admins"`
	LastDeletedPatientTag *cdc.ObjectId `json:"lastDeletedPatientTag"`
}

type UpdateDescription struct {
	UpdatedFields UpdatedFields `json:"updatedFields"`
	RemovedFields []string      `json:"removedFields"`
}

type UpdatedFields struct {
	Clinic
}

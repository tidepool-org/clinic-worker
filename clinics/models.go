package clinics

import "github.com/tidepool-org/clinic-worker/cdc"

type ClinicCDCEvent struct {
	Offset        int64  `json:"-"`
	OperationType string `json:"operationType"`
	FullDocument  Clinic `json:"fullDocument"`
}

func (p ClinicCDCEvent) ShouldApplyUpdates() bool {
	return p.OperationType == cdc.OperationTypeInsert && len(p.FullDocument.Name) > 0 && len(p.FullDocument.Admins) > 0
}

type Clinic struct {
	Id     *cdc.ObjectId `json:"_id"`
	Name   string        `json:"name"`
	Admins []string      `json:"admins"`
}

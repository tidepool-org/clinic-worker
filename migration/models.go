package migration

import "github.com/tidepool-org/clinic-worker/cdc"

const OperationTypeInsert = "insert"

type MigrationCDCEvent struct {
	Offset        int64     `json:"-"`
	FullDocument  Migration `json:"fullDocument"`
	OperationType string    `json:"operationType"`
}

type Migration struct {
	Offset   int64        `json:"-"`
	UserId   string       `json:"userId"`
	ClinicId cdc.ObjectId `json:"clinicId"`
}

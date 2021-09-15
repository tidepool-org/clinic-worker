package migration

import (
	"github.com/tidepool-org/clinic-worker/cdc"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
)

type MigrationCDCEvent struct {
	Offset        int64             `json:"-"`
	FullDocument  MigrationDocument `json:"fullDocument"`
	OperationType string            `json:"operationType"`
}

type MigrationDocument struct {
	UserId   string       `json:"userId"`
	ClinicId cdc.ObjectId `json:"clinicId"`
}

type Migration struct {
	clinic                 *clinics.Clinic
	legacyClinicianUserId  string
	legacyClinicianProfile *LegacyClinicianProfile
	legacyPatients         clients.UsersPermissions
}

type LegacyClinicianProfile struct {
	Name string `json:"fullName"`
}

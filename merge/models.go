package merge

import (
	"github.com/tidepool-org/clinic-worker/clinicians"
	"github.com/tidepool-org/clinic-worker/patients"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	patientPlanType = "patient"
	clinicianPlanType = "clinician"

	// ClinicianActionRetain is used for target clinicians when there's no corresponding clinician in the source clinic
	ClinicianActionRetain = "RETAIN"
	// ClinicianActionMerge is used when the source clinician will be merged to a target clinician record
	ClinicianActionMerge = "MERGE"
	// ClinicianActionMergeInto is when the target record will be the recipient of a merge
	ClinicianActionMergeInto = "MERGE_INTO"
	// ClinicianActionMove is used when the source clinician will be moved to the target clinic
	ClinicianActionMove = "MOVE"
)

type PersistentPlan[T any] struct {
	Id     *primitive.ObjectID `bson:"_id,omitempty"`
	Plan   T                   `bson:"plan"`
	PlanId primitive.ObjectID  `bson:"planId"`
	Type   string              `bson:"type"`
}

type PatientPlan struct {
	SourceClinicId   *primitive.ObjectID `bson:"sourceClinicId"`
	SourceClinicName string              `bson:"sourceClinicName"`
	SourcePatient    *patients.Patient   `bson:"sourcePatient"`

	TargetClinicId   *primitive.ObjectID `bson:"targetClinicId"`
	TargetClinicName string              `bson:"targetClinicName"`
	TargetPatient    *patients.Patient   `bson:"targetPatient"`
}

type ClinicianPlan struct {
	Clinician       clinicians.Clinician `bson:"clinician"`
	ClinicianAction string               `bson:"clinicianAction"`
	Downgraded      bool                 `bson:"downgraded"`
	ResultingRoles  []string             `bson:"resultingRoles"`
	Workspaces      []string             `bson:"workspaces"`

	SourceClinicName string `bson:"sourceClinicName"`
	TargetClinicName string `bson:"targetClinicName"`
}

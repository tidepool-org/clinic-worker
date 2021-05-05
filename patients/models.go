package patients

const OperationTypeInsert = "insert"
const OperationTypeReplace = "replace"
const OperationTypeUpdate = "update"

type PatientCDCEvent struct {
	Offset            int64             `json:"-"`
	FullDocument      Patient           `json:"fullDocument"`
	OperationType     string            `json:"operationType"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

func (p PatientCDCEvent) ShouldApplyUpdates() bool {
	if p.OperationType != OperationTypeInsert && p.OperationType != OperationTypeUpdate && p.OperationType != OperationTypeReplace {
		return false
	}
	return p.FullDocument.IsCustodial()
}

func (p PatientCDCEvent) ApplyUpdatesToExistingProfile(profile map[string]interface{}) {
	switch p.OperationType {
	case OperationTypeInsert, OperationTypeReplace:
		ApplyPatientChangesToProfile(p.FullDocument, profile)
	case OperationTypeUpdate:
		p.UpdateDescription.applyUpdatesToExistingProfile(profile)
	}
}

type ObjectId struct {
	Value string `json:"$oid"`
}

type Patient struct {
	Id            *ObjectId    `json:"_id"`
	ClinicId      *ObjectId    `json:"clinicId"`
	UserId        *string      `json:"userId"`
	BirthDate     *string      `json:"birthDate"`
	Email         *string      `json:"email"`
	FullName      *string      `json:"fullName"`
	Mrn           *string      `json:"mrn"`
	TargetDevices *[]string    `json:"targetDevices"`
	Permissions   *Permissions `json:"permissions"`
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

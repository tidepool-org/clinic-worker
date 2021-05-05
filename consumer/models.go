package consumer

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
	return p.Permissions != nil && p.Permissions.Custodial != nil
}

type Permissions struct {
	Custodial *Permission `json:"custodial"`
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

func ApplyPatientChangesToProfile(patient Patient, profile map[string]interface{}) {
	var patientProfile map[string]interface{}
	switch profile["patient"].(type) {
	case map[string]interface{}:
		patientProfile = profile["patient"].(map[string]interface{})
	default:
		patientProfile = make(map[string]interface{}, 0)
	}

	if patient.FullName != nil {
		profile["fullName"] = *patient.FullName
	}
	if patient.BirthDate != nil {
		patientProfile["birthday"] = *patient.BirthDate
	}
	if patient.Mrn != nil {
		patientProfile["mrn"] = *patient.Mrn
	}
	if patient.TargetDevices != nil {
		patientProfile["targetDevices"] = *patient.TargetDevices
	}
	if patient.Email != nil {
		profile["email"] = *patient.Email
		patientProfile["emails"] = []string{*patient.Email}
	}
}

func RemoveFieldsFromProfile(removedFields []string, profile map[string]interface{}) {
	removedFieldsMap := make(map[string]bool, 0)
	for _, field := range removedFields {
		removedFieldsMap[field] = true
	}
	if _, ok := removedFieldsMap["fullName"]; ok {
		delete(profile, "fullName")
	}
	if _, ok := removedFieldsMap["birthDate"]; ok {
		delete(profile["patient"].(map[string]interface{}), "birthday")
	}
	if _, ok := removedFieldsMap["mrn"]; ok {
		delete(profile["patient"].(map[string]interface{}), "mrn")
	}
	if _, ok := removedFieldsMap["targetDevices"]; ok {
		delete(profile["patient"].(map[string]interface{}), "targetDevices")
	}
	if _, ok := removedFieldsMap["email"]; ok {
		delete(profile, "email")
		delete(profile["patient"].(map[string]interface{}), "emails")
	}
}

type UpdatedFields struct {
	Patient
}

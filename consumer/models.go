package consumer

const OperationTypeCreate = "create"
const OperationTypeUpdate = "update"

type PatientCDCEvent struct {
	FullDocument      Patient           `json:"fullDocument"`
	OperationType     string            `json:"operationType"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

func (p PatientCDCEvent) ShouldApplyUpdates() bool {
	if p.OperationType != OperationTypeCreate && p.OperationType != OperationTypeUpdate {
		return false
	}
	return p.FullDocument.IsCustodial()
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

func (u UpdateDescription) ApplyUpdatesToExistingProfile(profile map[string]interface{}) {
	var patient map[string]interface{}
	switch profile["patient"].(type) {
	case map[string]interface{}:
		patient = profile["patient"].(map[string]interface{})
	default:
		patient = make(map[string]interface{}, 0)
	}

	removedFields := make(map[string]bool, 0)
	for _, field := range u.RemovedFields {
		removedFields[field] = true
	}

	if u.UpdatedFields.FullName != nil {
		profile["fullName"] = *u.UpdatedFields.FullName
	}
	if _, ok := removedFields["fullName"]; ok {
		delete(profile, "fullName")
	}

	if u.UpdatedFields.BirthDate != nil {
		patient["birthday"] = *u.UpdatedFields.BirthDate
	}
	if _, ok := removedFields["birthDate"]; ok {
		delete(patient, "birthday")
	}

	if u.UpdatedFields.Mrn != nil {
		patient["mrn"] = *u.UpdatedFields.Mrn
	}
	if _, ok := removedFields["mrn"]; ok {
		delete(patient, "mrn")
	}

	if u.UpdatedFields.TargetDevices != nil {
		patient["targetDevices"] = *u.UpdatedFields.TargetDevices
	}
	if _, ok := removedFields["targetDevices"]; ok {
		delete(patient, "targetDevices")
	}

	if u.UpdatedFields.Email != nil {
		profile["email"] = *u.UpdatedFields.Email
		patient["emails"] = []string{*u.UpdatedFields.Email}
	}
	if _, ok := removedFields["email"]; ok {
		delete(profile, "email")
		delete(patient, "emails")
	}

	if len(patient) == 0 {
		delete(profile, "patient")
	} else {
		profile["patient"] = patient
	}
}

type UpdatedFields struct {
	Patient
}

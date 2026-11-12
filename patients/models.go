package patients

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/tidepool-org/clinic-worker/cdc"
	api "github.com/tidepool-org/clinic/client"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
)

type PatientCDCEvent struct {
	Offset            int64             `json:"-"`
	FullDocument      Patient           `json:"fullDocument"`
	OperationType     string            `json:"operationType"`
	UpdateDescription UpdateDescription `json:"updateDescription"`
}

func (p PatientCDCEvent) IsUploadReminderEvent() bool {
	if p.OperationType != cdc.OperationTypeUpdate && p.OperationType != cdc.OperationTypeReplace {
		return false
	}
	if p.FullDocument.UserId == nil {
		return false
	}

	lastUploadReminderTime := p.UpdateDescription.UpdatedFields.LastUploadReminderTime
	return lastUploadReminderTime != nil && lastUploadReminderTime.Value > 0
}

func (p PatientCDCEvent) IsProfileUpdateEvent() bool {
	if p.OperationType != cdc.OperationTypeInsert && p.OperationType != cdc.OperationTypeUpdate && p.OperationType != cdc.OperationTypeReplace {
		return false
	}
	if p.FullDocument.UserId == nil {
		return false
	}
	// We want to apply profile updates and send invites only to custodial accounts.
	return p.FullDocument.IsCustodial()
}

func (p PatientCDCEvent) IsPatientCreateFromExistingUserEvent() bool {
	return p.OperationType == cdc.OperationTypeInsert && !p.FullDocument.IsCustodial()
}

// PatientNeedsSummary reports whether the event creates a patient record without a
// summary, which the data service is then asked to send.
func (p PatientCDCEvent) PatientNeedsSummary() bool {
	if p.OperationType != cdc.OperationTypeInsert {
		return false
	}

	if p.FullDocument.UserId == nil {
		return false
	}

	// return true if summary is missing
	return p.FullDocument.Summary == nil
}

const glycemicRangesField = "glycemicRanges"

// GlycemicRangesChanged reports whether the event changes the glycemic ranges of a
// patient record, whose summary is then out of date: it was computed for the ranges the
// record had before. A change to the ranges is described either as the whole field, or
// as the paths of the parts of it that changed or were removed. A replacement doesn't
// describe what changed, so it is reported as a change.
func (p PatientCDCEvent) GlycemicRangesChanged() bool {
	if p.FullDocument.UserId == nil {
		return false
	}
	switch p.OperationType {
	case cdc.OperationTypeReplace:
		return true
	case cdc.OperationTypeUpdate:
		description := p.UpdateDescription
		return slices.ContainsFunc(description.UpdatedFieldNames, isGlycemicRangesField) ||
			slices.ContainsFunc(description.RemovedFields, isGlycemicRangesField)
	}
	return false
}

// isGlycemicRangesField reports whether name is the glycemic ranges field of a patient
// record, or the path of a part of it.
func isGlycemicRangesField(name string) bool {
	return name == glycemicRangesField || strings.HasPrefix(name, glycemicRangesField+".")
}

func (p PatientCDCEvent) ApplyUpdatesToExistingProfile(profile map[string]interface{}) {
	switch p.OperationType {
	case cdc.OperationTypeInsert, cdc.OperationTypeReplace:
		ApplyPatientChangesToProfile(p.FullDocument, profile)
	case cdc.OperationTypeUpdate:
		p.UpdateDescription.applyUpdatesToExistingProfile(profile)
	}
}

// PatientSummary only records that a patient record has a summary: the data service
// sends summaries to the clinic service, so nothing here reads their content.
type PatientSummary struct{}

type Patient struct {
	Id                             *cdc.ObjectId              `json:"_id" bson:"_id"`
	ClinicId                       *cdc.ObjectId              `json:"clinicId" bson:"clinicId"`
	UserId                         *string                    `json:"userId" bson:"userId"`
	BirthDate                      *string                    `json:"birthDate" bson:"birthDate"`
	Email                          *string                    `json:"email" bson:"email"`
	FullName                       *string                    `json:"fullName" bson:"fullName"`
	Mrn                            *string                    `json:"mrn" bson:"mrn"`
	TargetDevices                  *[]string                  `json:"targetDevices" bson:"targetDevices"`
	DataSources                    *[]PatientDataSource       `json:"dataSources" bson:"dataSources"`
	Permissions                    *Permissions               `json:"permissions" bson:"permissions"`
	IsMigrated                     bool                       `json:"isMigrated" bson:"isMigrated"`
	InvitedBy                      *string                    `json:"invitedBy" bson:"invitedBy"`
	LastRequestedDexcomConnectTime *cdc.Date                  `json:"lastRequestedDexcomConnectTime" bson:"lastRequestedDexcomConnectTime"`
	LastUploadReminderTime         *cdc.Date                  `json:"lastUploadReminderTime" bson:"lastUploadReminderTime"`
	Summary                        *PatientSummary            `json:"summary" bson:"summary"`
	ProviderConnectionRequests     ProviderConnectionRequests `json:"providerConnectionRequests" bson:"providerConnectionRequests"`
	DiagnosisType                  *string                    `json:"diagnosisType" bson:"diagnosisType"`
}

type ProviderConnectionRequests map[string]ConnectionRequests

type ConnectionRequests []ConnectionRequest

type ConnectionRequest struct {
	ProviderName string   `json:"providerName" bson:"providerName"`
	CreatedTime  cdc.Date `json:"createdTime" bson:"createdTime"`
}

func (p PatientCDCEvent) CreateDataSourceBody(source clients.DataSource) clinics.DataSourceV1 {
	dataSource := clinics.DataSourceV1{
		ProviderName: *source.ProviderName,
		State:        api.DataSourceV1State(*source.State),
	}

	if source.ModifiedTime != nil {
		modifiedTimeVal := clinics.DatetimeV1(source.ModifiedTime.Format(time.RFC3339))
		dataSource.ModifiedTime = &modifiedTimeVal
	}

	return dataSource
}

type PatientDataSource struct {
	DataSourceId *cdc.ObjectId `json:"dataSourceId,omitempty"`
	ModifiedTime *cdc.Date     `json:"modifiedTime,omitempty"`
	ProviderName *string       `json:"providerName"`
	State        *string       `json:"state"`
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

	// UpdatedFieldNames holds the name of every updated field, including the paths of
	// nested fields updated in part, which UpdatedFields can't all represent.
	UpdatedFieldNames []string `json:"-"`
}

// UnmarshalJSON decodes the update description, and records the names of the updated
// fields.
func (u *UpdateDescription) UnmarshalJSON(data []byte) error {
	type plain UpdateDescription
	if err := json.Unmarshal(data, (*plain)(u)); err != nil {
		return err
	}

	var names struct {
		UpdatedFields map[string]json.RawMessage `json:"updatedFields"`
	}
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}
	u.UpdatedFieldNames = make([]string, 0, len(names.UpdatedFields))
	for name := range names.UpdatedFields {
		u.UpdatedFieldNames = append(u.UpdatedFieldNames, name)
	}
	slices.Sort(u.UpdatedFieldNames)
	return nil
}

func (u UpdateDescription) applyUpdatesToExistingProfile(profile map[string]interface{}) {
	ApplyPatientChangesToProfile(u.UpdatedFields.Patient, profile)
	RemoveFieldsFromProfile(u.RemovedFields, profile)
}

type UpdatedFields struct {
	Patient

	// Partial updates to nested fields are encoded using dot notation in CDC events
	ProviderConnectionRequestsDexcom ConnectionRequests `json:"providerConnectionRequests.dexcom"`
	ProviderConnectionRequestsTwiist ConnectionRequests `json:"providerConnectionRequests.twiist"`
	ProviderConnectionRequestsAbbott ConnectionRequests `json:"providerConnectionRequests.abbott"`
}

func (u UpdatedFields) GetUpdatedConnectionRequests() ConnectionRequests {
	var requests ConnectionRequests
	for _, r := range u.ProviderConnectionRequests {
		requests = AppendMostRecentConnectionRequest(requests, r)
	}
	requests = AppendMostRecentConnectionRequest(requests, u.ProviderConnectionRequestsDexcom)
	requests = AppendMostRecentConnectionRequest(requests, u.ProviderConnectionRequestsTwiist)
	requests = AppendMostRecentConnectionRequest(requests, u.ProviderConnectionRequestsAbbott)

	return requests
}

func AppendMostRecentConnectionRequest(requests ConnectionRequests, updated ConnectionRequests) ConnectionRequests {
	if len(updated) == 0 {
		return requests
	}

	return append(requests, updated[0])
}

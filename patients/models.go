package patients

import (
	"time"

	summaries "github.com/tidepool-org/go-common/clients/summary"

	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/patientsummary"
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

func (p PatientCDCEvent) ApplyUpdatesToExistingProfile(profile map[string]interface{}) {
	switch p.OperationType {
	case cdc.OperationTypeInsert, cdc.OperationTypeReplace:
		ApplyPatientChangesToProfile(p.FullDocument, profile)
	case cdc.OperationTypeUpdate:
		p.UpdateDescription.applyUpdatesToExistingProfile(profile)
	}
}

type BGMStats struct {
	Config  summaries.SummaryConfigV1             `json:"config" bson:"config"`
	Dates   patientsummary.Dates                  `json:"dates" bson:"dates"`
	Periods map[string]*summaries.GlucosePeriodV5 `json:"periods" bson:"periods"`
}

type CGMStats struct {
	Config  summaries.SummaryConfigV1             `json:"config" bson:"config"`
	Dates   patientsummary.Dates                  `json:"dates" bson:"dates"`
	Periods map[string]*summaries.GlucosePeriodV5 `json:"periods" bson:"periods"`
}

type CDCSummary struct {
	CGM *CGMStats `json:"cgmStats" bson:"cgmStats"`
	BGM *BGMStats `json:"bgmStats" bson:"bgmStats"`
}

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
	Summary                        *CDCSummary                `json:"summary" bson:"summary"`
	ProviderConnectionRequests     ProviderConnectionRequests `json:"providerConnectionRequests" bson:"providerConnectionRequests"`
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

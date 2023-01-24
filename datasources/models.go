package datasources

import (
	"time"

	"github.com/tidepool-org/clinic-worker/cdc"
	api "github.com/tidepool-org/clinic/client"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
)

type CDCEvent struct {
	Offset        int64      `json:"-"`
	FullDocument  DataSource `json:"fullDocument"`
	OperationType string     `json:"operationType"`
}

func (p CDCEvent) ShouldApplyUpdates() bool {
	if p.OperationType != cdc.OperationTypeInsert &&
		p.OperationType != cdc.OperationTypeUpdate &&
		p.OperationType != cdc.OperationTypeReplace {
		return false
	}

	if p.FullDocument.UserID == nil {
		return false
	}

	return true
}

type DataSource struct {
	ID           *cdc.ObjectId `json:"_id"`
	UserID       *string       `json:"userId,omitempty"`
	ProviderName *string       `json:"providerName"`
	ModifiedTime *cdc.Date     `json:"modifiedTime,omitempty"`
	State        *string       `json:"state"`
}

func (p CDCEvent) CreateUpdateBody(source clients.DataSource) clinics.DataSource {
	patientUpdate := clinics.DataSource{
		DataSourceId: &p.FullDocument.ID.Value,
		ProviderName: *source.ProviderName,
		State:        api.DataSourceState(*source.State),
	}

	if source.ModifiedTime != nil {
		modifiedTimeVal := clinics.DateTime(source.ModifiedTime.Format(time.RFC3339))
		patientUpdate.ModifiedTime = &modifiedTimeVal
	}

	return patientUpdate
}

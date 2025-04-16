package redox

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/tidepool-org/clinic-worker/types"
	models "github.com/tidepool-org/clinic/redox_models"
	"io"
	"time"
)

type NewNotes models.NewNotes

var _ Notes = &NewNotes{}

func CreateNewNotes() *NewNotes {
	res := NewNotes{}
	now := time.Now().Format(time.RFC3339)

	res.Meta.EventType = EventTypeNewNotes
	res.Meta.DataModel = DataModelNotes
	res.Meta.EventDateTime = &now

	return &res
}

func (n *NewNotes) SetSourceFromClient(client Client) {
	source := client.GetSource()
	n.Meta.Source = &source
}

func (n *NewNotes) SetDestination(destinationId string) {
	n.Meta.Destinations = types.NewSlicePtr(n.Meta.Destinations, 1)
	(*n.Meta.Destinations)[0].ID = &destinationId
}

func (n *NewNotes) SetPatientFromOrder(order models.NewOrder) {
	n.Patient.Identifiers = order.Patient.Identifiers
	n.Patient.Demographics = order.Patient.Demographics
}

func (n *NewNotes) SetProviderFromOrder(order models.NewOrder) {
	if order.Order.Provider != nil {
		if order.Order.Provider.ID != nil {
			n.Note.Provider.ID = *order.Order.Provider.ID
		}
		n.Note.Provider.FirstName = order.Order.Provider.FirstName
		n.Note.Provider.LastName = order.Order.Provider.LastName
		n.Note.Provider.IDType = order.Order.Provider.IDType
	}
}

func (n *NewNotes) SetProcedureFromOrder(order models.NewOrder) {
	if order.Order.Procedure != nil {
		if n.Visit == nil {
			n.Visit = types.NewStructPtr(n.Visit)
		}
		n.Visit.AdditionalStaff = types.NewSlicePtr(n.Visit.AdditionalStaff, 1)
		(*n.Visit.AdditionalStaff)[0].Role = types.NewStructPtr((*n.Visit.AdditionalStaff)[0].Role)
		(*n.Visit.AdditionalStaff)[0].Role.Code = order.Order.Procedure.Code
		(*n.Visit.AdditionalStaff)[0].Role.Codeset = order.Order.Procedure.Codeset
		(*n.Visit.AdditionalStaff)[0].Role.Description = order.Order.Procedure.Description
	}
}

func (n *NewNotes) SetVisitNumberFromOrder(order models.NewOrder) {
	if order.Visit != nil && order.Visit.VisitNumber != nil {
		if n.Visit == nil {
			n.Visit = types.NewStructPtr(n.Visit)
		}
		n.Visit.VisitNumber = order.Visit.VisitNumber
	}
}

func (n *NewNotes) SetVisitLocationFromOrder(order models.NewOrder) {
	if order.Visit == nil {
		return
	}

	if n.Visit == nil {
		n.Visit = types.NewStructPtr(n.Visit)
	}
	n.Visit.Location = order.Visit.Location
}

func (n *NewNotes) SetAccountNumberFromOrder(order models.NewOrder) {
	if order.Visit != nil && order.Visit.AccountNumber != nil {
		if n.Visit == nil {
			n.Visit = types.NewStructPtr(n.Visit)
		}
		n.Visit.AccountNumber = order.Visit.AccountNumber
	}
}

func (n *NewNotes) SetOrderId(order models.NewOrder) {
	n.Orders = types.NewSlicePtr(n.Orders, 1)
	(*n.Orders)[0].ID = &order.Order.ID
}

func (n *NewNotes) SetReportMetadata(documentId string) {
	now := time.Now().Format(time.RFC3339)
	availability := NoteAvailabilityAvailable

	n.Note.Availability = &availability
	n.Note.DocumentationDateTime = &now
	n.Note.DocumentID = documentId
	n.Note.DocumentType = NoteReportDocumentType
	n.Note.Provider.ID = NoteProviderId
}

func (n *NewNotes) SetEmbeddedFile(fileName string, fileType string, reader io.Reader) error {
	if fileName == "" {
		return fmt.Errorf("file name is required")
	}
	if fileType == "" {
		return fmt.Errorf("file type is required")
	}

	n.Note.ContentType = NoteContentTypeBase64
	n.Note.FileName = &fileName
	n.Note.FileType = &fileType

	buffer := new(bytes.Buffer)
	if _, err := buffer.ReadFrom(reader); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	fileContents := base64.StdEncoding.EncodeToString(buffer.Bytes())
	n.Note.FileContents = &fileContents
	return nil
}

func (n *NewNotes) SetUploadReference(fileName string, fileType string, result UploadResult) error {
	if fileName == "" {
		return fmt.Errorf("file name is required")
	}
	if fileType == "" {
		return fmt.Errorf("file type is required")
	}
	if result.URI == "" {
		return fmt.Errorf("upload result URI is required")
	}

	n.Note.ContentType = NoteContentTypePlainText
	n.Note.FileName = &fileName
	n.Note.FileType = &fileType
	n.Note.FileContents = &result.URI

	return nil
}

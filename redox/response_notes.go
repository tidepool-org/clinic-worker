package redox

import (
	_ "embed"
	"fmt"
	"io"

	models "github.com/tidepool-org/clinic/redox_models"
)

const (
	EventTypeNewNotes     = "New"
	EventTypeReplaceNotes = "Replace"
	DataModelNotes        = "Notes"

	NoteProviderId = "Tidepool"

	NoteAvailabilityAvailable  = "Available"
	NoteContentTypeBase64      = "Base64 Encoded"
	NoteContentTypePlainText   = "Plain Text"
	NoteReportDocumentIdPrefix = "Report"
	NoteReportDocumentType     = "Tidepool Report"
	NoteReportFileType         = "PDF"
	NoteReportFileName         = "report.pdf"
)

type Notes interface {
	SetAccountNumberFromOrder(order models.NewOrder)
	SetSourceFromClient(client Client)
	SetDestination(destinationId string)
	SetPatientFromOrder(order models.NewOrder)
	SetProviderFromOrder(order models.NewOrder)
	SetProcedureFromOrder(order models.NewOrder)
	SetVisitNumberFromOrder(order models.NewOrder)
	SetVisitLocationFromOrder(order models.NewOrder)
	SetOrderId(order models.NewOrder)
	SetReportMetadata(documentId string)
	SetEmbeddedFile(fileName string, fileType string, reader io.Reader) error
	SetUploadReference(fileName string, fileType string, result UploadResult) error
}

func GenerateReportDocumentId(clinicId string, patientId string) string {
	return fmt.Sprintf("%s-%s-%s", NoteReportDocumentIdPrefix, clinicId, patientId)
}

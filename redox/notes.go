package redox

import (
	"bytes"
	"encoding/base64"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/redox/models"
	"io"
	"time"

	_ "embed"
)

const (
	EventTypeNewNotes = "New"
	DataModelNotes    = "Notes"

	NoteProviderId   = "3012128418" // FEI Number
	NoteProviderType = "FEI"

	NoteAvailabilityAvailable  = "Available"
	NoteContentTypeBase64      = "Base64 Encoded"
	NoteReportDocumentIdPrefix = "Report"
	NoteReportDocumentType     = "Tidepool Report"
	NoteReportFileType         = "PDF"
)

//go:embed test/sample-report.pdf
var sampleReport []byte

func NewNotes() models.NewNotes {
	res := models.NewNotes{}
	now := time.Now().Format(time.RFC3339)

	res.Meta.EventType = EventTypeNewNotes
	res.Meta.DataModel = DataModelNotes
	res.Meta.EventDateTime = &now

	return res
}

func SetNotesPatientFromOrder(order models.NewOrder, notes *models.NewNotes) {
	notes.Patient.Identifiers = order.Patient.Identifiers
	notes.Patient.Demographics = order.Patient.Demographics
}

func SetReportMetadata(clinic clinics.Clinic, patient clinics.Patient, notes *models.NewNotes) {
	now := time.Now().Format(time.RFC3339)
	availability := NoteAvailabilityAvailable
	providerIdType := NoteProviderType

	notes.Note.Availability = &availability
	notes.Note.DocumentationDateTime = &now
	notes.Note.DocumentID = GenerateReportDocumentId(*clinic.Id, *patient.Id)
	notes.Note.DocumentType = NoteReportDocumentType
	notes.Note.Provider.ID = NoteProviderId
	notes.Note.Provider.IDType = &providerIdType
}

func GenerateReportDocumentId(clinicId string, patientId string) string {
	return fmt.Sprintf("%s-%s-%s", NoteReportDocumentIdPrefix, clinicId, patientId)
}

func EmbedFileInNotes(fileName string, fileType string, reader io.Reader, notes *models.NewNotes) error {
	if fileName == "" {
		return fmt.Errorf("file name is required")
	}
	if fileType == "" {
		return fmt.Errorf("file type is required")
	}

	notes.Note.ContentType = NoteContentTypeBase64
	notes.Note.FileName = &fileName
	notes.Note.FileType = &fileType

	buffer := new(bytes.Buffer)
	if _, err := buffer.ReadFrom(reader); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	fileContents := base64.StdEncoding.EncodeToString(buffer.Bytes())
	notes.Note.FileContents = &fileContents
	return nil
}

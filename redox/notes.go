package redox

import (
	"bytes"
	"encoding/base64"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"io"
	"time"

	_ "embed"
)

const (
	EventTypeNewNotes = "New"
	DataModelNotes    = "Notes"

	NoteProviderId = "Tidepool"

	NoteAvailabilityAvailable  = "Available"
	NoteContentTypeBase64      = "Base64 Encoded"
	NoteContentTypePlainText   = "Plain Text"
	NoteReportDocumentIdPrefix = "Report"
	NoteReportDocumentType     = "Tidepool Report"
	NoteReportFileType         = "PDF"
	NoteReportFileName         = "report.pdf"
)

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

	notes.Note.Availability = &availability
	notes.Note.DocumentationDateTime = &now
	notes.Note.DocumentID = GenerateReportDocumentId(*clinic.Id, *patient.Id)
	notes.Note.DocumentType = NoteReportDocumentType
	notes.Note.Provider.ID = NoteProviderId
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

func SetUploadReferenceInNote(fileName string, fileType string, result UploadResult, notes *models.NewNotes) error {
	if fileName == "" {
		return fmt.Errorf("file name is required")
	}
	if fileType == "" {
		return fmt.Errorf("file type is required")
	}
	if result.URI == "" {
		return fmt.Errorf("upload result URI is required")
	}

	notes.Note.ContentType = NoteContentTypePlainText
	notes.Note.FileName = &fileName
	notes.Note.FileType = &fileType
	notes.Note.FileContents = &result.URI

	return nil
}

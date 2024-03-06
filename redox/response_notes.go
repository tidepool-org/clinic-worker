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

func SetNotesProviderFromOrder(order models.NewOrder, notes *models.NewNotes) {
	if order.Order.Provider != nil {
		if order.Order.Provider.ID != nil {
			notes.Note.Provider.ID = *order.Order.Provider.ID
		}
		notes.Note.Provider.FirstName = order.Order.Provider.FirstName
		notes.Note.Provider.LastName = order.Order.Provider.LastName
		notes.Note.Provider.IDType = order.Order.Provider.IDType
	}
}

func SetNotesProcedureFromOrder(order models.NewOrder, notes *models.NewNotes) {
	if order.Order.Procedure != nil {
		additionalStaff := []struct {
			Address *struct {
				City          *string `json:"City"`
				Country       *string `json:"Country"`
				County        *string `json:"County"`
				State         *string `json:"State"`
				StreetAddress *string `json:"StreetAddress"`
				ZIP           *string `json:"ZIP"`
			} `json:"Address,omitempty"`
			Credentials    *[]interface{} `json:"Credentials,omitempty"`
			EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
			FirstName      *string        `json:"FirstName"`
			ID             *string        `json:"ID"`
			IDType         *string        `json:"IDType"`
			LastName       *string        `json:"LastName"`
			Location       *struct {
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			PhoneNumber *struct {
				Office *string `json:"Office"`
			} `json:"PhoneNumber,omitempty"`
			Role *struct {
				Code        *string `json:"Code"`
				Codeset     *string `json:"Codeset"`
				Description *string `json:"Description"`
			} `json:"Role,omitempty"`
		}{{
			Role: &struct {
				Code        *string `json:"Code"`
				Codeset     *string `json:"Codeset"`
				Description *string `json:"Description"`
			}{},
		}}

		additionalStaff[0].Role.Code = order.Order.Procedure.Code
		additionalStaff[0].Role.Codeset = order.Order.Procedure.Codeset
		additionalStaff[0].Role.Description = order.Order.Procedure.Description

		notes.Visit = &struct {
			AccountNumber   *string `json:"AccountNumber"`
			AdditionalStaff *[]struct {
				Address *struct {
					City          *string `json:"City"`
					Country       *string `json:"Country"`
					County        *string `json:"County"`
					State         *string `json:"State"`
					StreetAddress *string `json:"StreetAddress"`
					ZIP           *string `json:"ZIP"`
				} `json:"Address,omitempty"`
				Credentials    *[]interface{} `json:"Credentials,omitempty"`
				EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
				FirstName      *string        `json:"FirstName"`
				ID             *string        `json:"ID"`
				IDType         *string        `json:"IDType"`
				LastName       *string        `json:"LastName"`
				Location       *struct {
					Department            *string `json:"Department"`
					DepartmentIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"DepartmentIdentifiers,omitempty"`
					Facility            *string `json:"Facility"`
					FacilityIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"FacilityIdentifiers,omitempty"`
					Room *string `json:"Room"`
					Type *string `json:"Type"`
				} `json:"Location,omitempty"`
				PhoneNumber *struct {
					Office *string `json:"Office"`
				} `json:"PhoneNumber,omitempty"`
				Role *struct {
					Code        *string `json:"Code"`
					Codeset     *string `json:"Codeset"`
					Description *string `json:"Description"`
				} `json:"Role,omitempty"`
			} `json:"AdditionalStaff,omitempty"`
			Location *struct {
				Bed                   *string `json:"Bed"`
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			PatientClass  *string `json:"PatientClass"`
			VisitDateTime *string `json:"VisitDateTime"`
			VisitNumber   *string `json:"VisitNumber"`
		}{
			AdditionalStaff: &additionalStaff,
		}
	}
}

func SetVisitNumberInNotes(order models.NewOrder, notes *models.NewNotes) {
	if order.Visit != nil && order.Visit.VisitNumber != nil && *order.Visit.VisitNumber != "" {
		visit := struct {
			AccountNumber   *string `json:"AccountNumber"`
			AdditionalStaff *[]struct {
				Address *struct {
					City          *string `json:"City"`
					Country       *string `json:"Country"`
					County        *string `json:"County"`
					State         *string `json:"State"`
					StreetAddress *string `json:"StreetAddress"`
					ZIP           *string `json:"ZIP"`
				} `json:"Address,omitempty"`
				Credentials    *[]interface{} `json:"Credentials,omitempty"`
				EmailAddresses *[]interface{} `json:"EmailAddresses,omitempty"`
				FirstName      *string        `json:"FirstName"`
				ID             *string        `json:"ID"`
				IDType         *string        `json:"IDType"`
				LastName       *string        `json:"LastName"`
				Location       *struct {
					Department            *string `json:"Department"`
					DepartmentIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"DepartmentIdentifiers,omitempty"`
					Facility            *string `json:"Facility"`
					FacilityIdentifiers *[]struct {
						ID     *string `json:"ID"`
						IDType *string `json:"IDType"`
					} `json:"FacilityIdentifiers,omitempty"`
					Room *string `json:"Room"`
					Type *string `json:"Type"`
				} `json:"Location,omitempty"`
				PhoneNumber *struct {
					Office *string `json:"Office"`
				} `json:"PhoneNumber,omitempty"`
				Role *struct {
					Code        *string `json:"Code"`
					Codeset     *string `json:"Codeset"`
					Description *string `json:"Description"`
				} `json:"Role,omitempty"`
			} `json:"AdditionalStaff,omitempty"`
			Location *struct {
				Bed                   *string `json:"Bed"`
				Department            *string `json:"Department"`
				DepartmentIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"DepartmentIdentifiers,omitempty"`
				Facility            *string `json:"Facility"`
				FacilityIdentifiers *[]struct {
					ID     *string `json:"ID"`
					IDType *string `json:"IDType"`
				} `json:"FacilityIdentifiers,omitempty"`
				Room *string `json:"Room"`
				Type *string `json:"Type"`
			} `json:"Location,omitempty"`
			PatientClass  *string `json:"PatientClass"`
			VisitDateTime *string `json:"VisitDateTime"`
			VisitNumber   *string `json:"VisitNumber"`
		}{
			VisitNumber: order.Visit.VisitNumber,
		}

		notes.Visit = &visit
	}
}

func SetOrderIdInNotes(order models.NewOrder, notes *models.NewNotes) {
	orders := []struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	}{{
		ID: &order.Order.ID,
	}}
	notes.Orders = &orders
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

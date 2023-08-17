package redox_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic-worker/redox"
	"github.com/tidepool-org/clinic-worker/test"
	api "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"time"
)

var _ = Describe("Notes", func() {
	Describe("NewNotes", func() {
		It("returns a correctly instantiated note", func() {
			notes := redox.NewNotes()
			Expect(notes.Meta.DataModel).To(Equal("Notes"))
			Expect(notes.Meta.EventType).To(Equal("New"))
			Expect(notes.Meta.EventDateTime).ToNot(BeNil())

			eventDateTime, err := time.Parse(time.RFC3339, *notes.Meta.EventDateTime)
			Expect(err).ToNot(HaveOccurred())
			Expect(eventDateTime).To(BeTemporally("~", time.Now(), 3*time.Second))
		})
	})

	Context("With order", func() {
		var notes models.NewNotes
		var order models.NewOrder

		BeforeEach(func() {
			notes = redox.NewNotes()
			fixture, err := test.LoadFixture("test/fixtures/neworder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(fixture, &order)).To(Succeed())
		})

		Describe("SetNotesPatientFromOrder", func() {
			It("sets the patient identifier and demographics from the order", func() {
				redox.SetNotesPatientFromOrder(order, &notes)
				Expect(notes.Patient.Identifiers).To(Equal(order.Patient.Identifiers))
				Expect(notes.Patient.Demographics).To(Equal(order.Patient.Demographics))
			})
		})

		Describe("SetVisitNumberInNotes", func() {
			It("sets the visit number from the order", func() {
				redox.SetVisitNumberInNotes(order, &notes)
				Expect(notes.Visit).ToNot(BeNil())
				Expect(notes.Visit.VisitNumber).To(PointTo(Equal(*order.Visit.VisitNumber)))
			})
		})

		Describe("SetOrderIdInNotes", func() {
			It("sets the order id from the order", func() {
				redox.SetOrderIdInNotes(order, &notes)
				Expect(notes.Orders).ToNot(BeNil())
				Expect(*notes.Orders).To(HaveLen(1))
				Expect((*notes.Orders)[0].ID).To(PointTo(Equal(order.Order.ID)))
			})
		})

		Describe("SetReportMetadata", func() {
			It("sets the correct metadata", func() {
				clinciId := "clinic12345"
				patientId := "patient12345"
				clinic := api.Clinic{Id: &clinciId}
				patient := api.Patient{Id: &patientId}

				redox.SetReportMetadata(clinic, patient, &notes)
				Expect(notes.Note.Availability).To(PointTo(Equal("Available")))
				Expect(notes.Note.DocumentID).To(Equal("Report-clinic12345-patient12345"))
				Expect(notes.Note.DocumentType).To(Equal("Tidepool Report"))
				Expect(notes.Note.Provider.ID).To(Equal("Tidepool"))
				Expect(notes.Note.DocumentationDateTime).ToNot(BeNil())

				documentationDateTime, err := time.Parse(time.RFC3339, *notes.Note.DocumentationDateTime)
				Expect(err).ToNot(HaveOccurred())
				Expect(documentationDateTime).To(BeTemporally("~", time.Now(), 3*time.Second))
			})
		})

		Describe("EmbedFileInNotes", func() {
			It("embeds the file in the notes", func() {
				content := []byte("test")
				expected := base64.StdEncoding.EncodeToString(content)
				fileName := "test.pdf"
				fileType := "PDF"
				reader := bytes.NewReader(content)

				Expect(redox.EmbedFileInNotes(fileName, fileType, reader, &notes)).To(Succeed())
				Expect(notes.Note.FileContents).To(PointTo(Equal(expected)))
				Expect(notes.Note.FileName).To(PointTo(Equal(fileName)))
				Expect(notes.Note.FileType).To(PointTo(Equal(fileType)))
				Expect(notes.Note.ContentType).To(Equal("Base64 Encoded"))
			})
		})

		Describe("SetUploadReferenceInNotes", func() {
			fileName := "test.pdf"
			fileType := "PDF"
			uri := "https://test.com/test.pdf"
			result := redox.UploadResult{URI: uri}

			Expect(redox.SetUploadReferenceInNote(fileName, fileType, result, &notes)).To(Succeed())
			Expect(notes.Note.FileContents).To(PointTo(Equal(uri)))
			Expect(notes.Note.FileName).To(PointTo(Equal(fileName)))
			Expect(notes.Note.FileType).To(PointTo(Equal(fileType)))
			Expect(notes.Note.ContentType).To(Equal("Plain Text"))
		})

	})

})

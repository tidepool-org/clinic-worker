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
	models "github.com/tidepool-org/clinic/redox_models"
	"time"
)

var _ = Describe("Notes", func() {
	Describe("ReplaceNotes", func() {
		It("returns a correctly instantiated note", func() {
			documentId := "01234567890"
			notes, err := redox.CreateReplaceNotes(documentId)
			Expect(err).ToNot(HaveOccurred())

			Expect(notes.Meta.DataModel).To(Equal("Notes"))
			Expect(notes.Meta.EventType).To(Equal("Replace"))
			Expect(notes.Meta.EventDateTime).ToNot(BeNil())

			Expect(notes.Note.OriginalDocumentID).To(Equal(documentId))
			eventDateTime, err := time.Parse(time.RFC3339, *notes.Meta.EventDateTime)
			Expect(err).ToNot(HaveOccurred())
			Expect(eventDateTime).To(BeTemporally("~", time.Now(), 3*time.Second))
		})
	})

	Context("With order", func() {
		var notes redox.ReplaceNotes
		var order models.NewOrder

		BeforeEach(func() {
			notes = redox.ReplaceNotes{}
			fixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(fixture, &order)).To(Succeed())
		})

		Describe("SetNotesPatientFromOrder", func() {
			It("sets the patient identifier and demographics from the order", func() {
				notes.SetPatientFromOrder(order)
				Expect(notes.Patient.Identifiers).To(Equal(order.Patient.Identifiers))
				Expect(notes.Patient.Demographics).To(Equal(order.Patient.Demographics))
			})
		})

		Describe("SetVisitNumberInNotes", func() {
			It("sets the visit number from the order", func() {
				notes.SetVisitNumberFromOrder(order)
				Expect(notes.Visit).ToNot(BeNil())
				Expect(notes.Visit.VisitNumber).To(PointTo(Equal(*order.Visit.VisitNumber)))
			})
		})

		Describe("SetVisitLocationFromOrder", func() {
			It("sets the visit location from the order", func() {
				notes.SetVisitLocationFromOrder(order)
				Expect(notes.Visit).ToNot(BeNil())
				Expect(notes.Visit.Location).To(PointTo(Equal(*order.Visit.Location)))
			})
		})

		Describe("SetAccountNumberInNotes", func() {
			It("sets the account number from the order", func() {
				notes.SetAccountNumberFromOrder(order)
				Expect(notes.Visit).ToNot(BeNil())
				Expect(notes.Visit.AccountNumber).To(PointTo(Equal(*order.Visit.AccountNumber)))
			})
		})

		Describe("SetOrderIdInNotes", func() {
			It("sets the order id from the order", func() {
				notes.SetOrderId(order)
				Expect(notes.Orders).ToNot(BeNil())
				Expect(*notes.Orders).To(HaveLen(1))
				Expect((*notes.Orders)[0].ID).To(PointTo(Equal(order.Order.ID)))
			})
		})

		Describe("SetReportMetadata", func() {
			It("sets the correct metadata", func() {
				clinciId := "clinic12345"
				patientId := "patient12345"
				documentId := redox.GenerateReportDocumentId(clinciId, patientId)

				notes.SetReportMetadata(documentId)
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

				Expect(notes.SetEmbeddedFile(fileName, fileType, reader)).To(Succeed())
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

			Expect(notes.SetUploadReference(fileName, fileType, result)).To(Succeed())
			Expect(notes.Note.FileContents).To(PointTo(Equal(uri)))
			Expect(notes.Note.FileName).To(PointTo(Equal(fileName)))
			Expect(notes.Note.FileType).To(PointTo(Equal(fileType)))
			Expect(notes.Note.ContentType).To(Equal("Plain Text"))
		})
	})
})

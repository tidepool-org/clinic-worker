package redox_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic-worker/redox"
	"github.com/tidepool-org/clinic-worker/test"
	models "github.com/tidepool-org/clinic/redox_models"
)

var _ = Describe("Notes", func() {
	Describe("NewNotes", func() {
		It("returns a correctly instantiated note", func() {
			notes := redox.CreateNewNotes()
			Expect(notes.Meta.DataModel).To(Equal("Notes"))
			Expect(notes.Meta.EventType).To(Equal("New"))
			Expect(notes.Meta.EventDateTime).ToNot(BeNil())

			eventDateTime, err := time.Parse(time.RFC3339, *notes.Meta.EventDateTime)
			Expect(err).ToNot(HaveOccurred())
			Expect(eventDateTime).To(BeTemporally("~", time.Now(), 3*time.Second))
		})
	})

	Context("With order", func() {
		var notes redox.NewNotes
		var order models.NewOrder

		BeforeEach(func() {
			notes = redox.NewNotes{}
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

			Context("with an existing visit", func() {
				var secondOrder models.NewOrder

				BeforeEach(func() {
					notes.SetVisitNumberFromOrder(order)
					Expect(notes.Visit).ToNot(BeNil())
					Expect(notes.Visit.VisitNumber).To(PointTo(Equal(*order.Visit.VisitNumber)))

					fixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
					Expect(err).ToNot(HaveOccurred())
					Expect(json.Unmarshal(fixture, &secondOrder)).To(Succeed())
					*secondOrder.Visit.VisitNumber = "foo"
				})

				It("sets the visit number", func() {
					notes.SetVisitNumberFromOrder(secondOrder)
					Expect(notes.Visit).ToNot(BeNil())
					Expect(notes.Visit.VisitNumber).To(PointTo(Equal(*secondOrder.Visit.VisitNumber)))
				})
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
			It("sets the visit number from the order", func() {
				notes.SetAccountNumberFromOrder(order)
				Expect(notes.Visit).ToNot(BeNil())
				Expect(notes.Visit.AccountNumber).To(PointTo(Equal(*order.Visit.AccountNumber)))
			})

			Context("with an existing visit", func() {
				var secondOrder models.NewOrder

				BeforeEach(func() {
					notes.SetAccountNumberFromOrder(order)
					Expect(notes.Visit).ToNot(BeNil())
					Expect(notes.Visit.AccountNumber).To(PointTo(Equal(*order.Visit.AccountNumber)))

					fixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
					Expect(err).ToNot(HaveOccurred())
					Expect(json.Unmarshal(fixture, &secondOrder)).To(Succeed())
					*secondOrder.Visit.AccountNumber = "foo"
				})

				It("sets the account number", func() {
					notes.SetAccountNumberFromOrder(secondOrder)
					Expect(notes.Visit).ToNot(BeNil())
					Expect(notes.Visit.AccountNumber).To(PointTo(Equal(*secondOrder.Visit.AccountNumber)))
				})
			})
		})

		Describe("SetProcedureFromOrder", func() {
			Context("with an existing visit", func() {
				BeforeEach(func() {
					notes.SetVisitNumberFromOrder(order)
					Expect(notes.Visit).ToNot(BeNil())
					Expect(notes.Visit.VisitNumber).To(PointTo(Equal(*order.Visit.VisitNumber)))
				})

				It("doesn't change an existing Visit's VisitNumber", func() {
					Expect(notes.Visit).ToNot(BeNil())
					notes.SetProcedureFromOrder(order)
					Expect(notes.Visit).ToNot(BeNil())
					Expect(notes.Visit.VisitNumber).To(PointTo(Equal(*order.Visit.VisitNumber)))
				})
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

		Describe("SetComponents", func() {
			Describe("ObservationsToGMINoteComponents", func() {
				It("sets the components to only the gmi observations", func() {
					percentageUnits := "%"
					bgUnits := "mmol/L"
					dayUnits := "day"
					hourUnits := "hour"
					reportingTime := "2026-02-25T11:06:39"

					observations := []*redox.Observation{
						{"REPORTING_PERIOD_START_CGM", "2023-04-09T17:44:09Z", "DateTime", nil, reportingTime, "CGM Reporting Period Start"},
						{"REPORTING_PERIOD_END_CGM", "2023-04-23T17:44:09Z", "DateTime", nil, reportingTime, "CGM Reporting Period End"},
						{"REPORTING_PERIOD_START_CGM_DATA", "2023-04-14T00:00:00Z", "DateTime", nil, reportingTime, "CGM Reporting Period Start Date of actual Data"},
						{"TIME_ABOVE_RANGE_VERY_HIGH_CGM", "4.4059", "Numeric", &percentageUnits, reportingTime, "CGM Level 2 Hyperglycemia: Time above range (TAR-VH): % of readings and time >250 mg/dL (>13.9 mmol/L)"},
						{"TIME_ABOVE_RANGE_HIGH_CGM", "25.6436", "Numeric", &percentageUnits, reportingTime, "CGM Time in Level 1 Hyperglycemia: Time above range (TAR-H): % of readings and time 181–250 mg/dL (10.1–13.9 mmol/L)"},
						{"TIME_IN_RANGE_CGM", "56.2871", "Numeric", &percentageUnits, reportingTime, "CGM Time in Range: Time in range (TIR): % of readings and time 70–180 mg/dL (3.9–10.0 mmol/L)"},
						{"TIME_BELOW_RANGE_LOW_CGM", "8.6139", "Numeric", &percentageUnits, reportingTime, "CGM Time in Level 1 Hypoglycemia: Time below range (TBR-L): % of readings and time 54–69 mg/dL (3.0–3.8 mmol/L)"},
						{"TIME_BELOW_RANGE_VERY_LOW_CGM", "5.0495", "Numeric", &percentageUnits, reportingTime, "CGM Time in Level 2 Hypoglycemia: <Time below range (TBR-VL): % of readings and time <54 mg/dL (<3.0 mmol/L)"},
						{"GLUCOSE_MANAGEMENT_INDICATOR", "6.7206", "Numeric", nil, reportingTime, "CGM Glucose Management Indicator during reporting period"},
						{"AVERAGE_CGM", "7.9212", "Numeric", &bgUnits, reportingTime, "CGM Average Glucose during reporting period"},
						{"STANDARD_DEVIATION_CGM", "1.4697", "Numeric", &bgUnits, reportingTime, "The standard deviation of CGM measurements during the reporting period"},
						{"COEFFICIENT_OF_VARIATION_CGM", "0.2004", "Numeric", nil, reportingTime, "The coefficient of variation (standard deviation * 100 / mean) of CGM measurements during the reporting period"},
						{"ACTIVE_WEAR_TIME_CGM", "50.1262", "Numeric", &percentageUnits, reportingTime, "Percentage of time CGM worn during reporting period"},
						{"DAYS_WITH_DATA_CGM", "2", "Numeric", &dayUnits, reportingTime, "Number of days with at least one CGM datum during the reporting period"},
						{"HOURS_WITH_DATA_CGM", "28", "Numeric", &hourUnits, reportingTime, "Number of hours with at least one CGM datum during the reporting period"},
						{"REPORTING_PERIOD_START_SMBG", "2023-04-11T00:57:11Z", "DateTime", nil, reportingTime, "SMBG Reporting Period Start"},
						{"REPORTING_PERIOD_END_SMBG", "2023-04-25T00:57:11Z", "DateTime", nil, reportingTime, "SMBG Reporting Period End"},
						{"REPORTING_PERIOD_START_SMBG_DATA", "2023-04-11T00:57:11Z", "DateTime", nil, reportingTime, "SMBG Reporting Period Start Date of actual Data"},
						{"TIME_ABOVE_RANGE_VERY_HIGH_SMBG", "18.8406", "Numeric", &percentageUnits, reportingTime, "% of readings > 250 mg/dL (>13.9 mmol/L)"},
						{"TIME_ABOVE_RANGE_HIGH_SMBG", "23.1884", "Numeric", &percentageUnits, reportingTime, "% of readings between 181–250 mg/dL (10.1–13.9 mmol/L)"},
						{"TIME_IN_RANGE_SMBG", "44.9275", "Numeric", &percentageUnits, reportingTime, "% of readings between 70–180 mg/dL (3.9–10.0 mmol/L)"},
						{"TIME_BELOW_RANGE_LOW_SMBG", "7.2464", "Numeric", &percentageUnits, reportingTime, "% of readings between 54–69 mg/dL (3.0–3.8 mmol/L)"},
						{"TIME_BELOW_RANGE_VERY_LOW_SMBG", "5.7971", "Numeric", &percentageUnits, reportingTime, "% of readings < 54 mg/dL (<3.0 mmol/L)"},
						{"READINGS_ABOVE_RANGE_VERY_HIGH_SMBG", "13", "Numeric", nil, reportingTime, "SMBG Level 2 Hyperglycemia: Number of readings above range (TAR-VH) time >250 mg/dL (>13.9 mmol/L) during reporting period"},
						{"READINGS_BELOW_RANGE_VERY_LOW_SMBG", "4", "Numeric", nil, reportingTime, "SMBG Level 2 Hypoglycemia Events: Number of readings <54 mg/dL (<3.0 mmol/L) during reporting period"},
						{"MAX_SMBG", "15.5556", "Numeric", &bgUnits, reportingTime, "Maximum blood glucose reading over the time period"},
						{"MIN_SMBG", "2.9889", "Numeric", &bgUnits, reportingTime, "Minimum blood glucose reading over the time period"},
						{"AVERAGE_SMBG", "9.5634", "Numeric", &bgUnits, reportingTime, "SMBG Average Glucose during reporting period"},
						{"STANDARD_DEVIATION_SMBG", "1.4698", "Numeric", &bgUnits, reportingTime, "The standard deviation of SMBG measurements during the reporting period"},
						{"COEFFICIENT_OF_VARIATION_SMBG", "0.2005", "Numeric", nil, reportingTime, "The coefficient of variation (standard deviation * 100 / mean) of SMBG measurements during the reporting period"},
						{"TOTAL_READING_COUNT_SMBG", "69", "Numeric", nil, reportingTime, "The total number of SMBG readings taken during the SMBG Reporting Period"},
						{"CHECK_RATE_READINGS_DAY_SMBG", "4.9286", "Numeric", nil, reportingTime, "Average Numeric of SMBG readings per day during reporting period"},
						{"DAYS_WITH_DATA_SMBG", "3", "Numeric", &dayUnits, reportingTime, "The total number of days with at least 1 SMBG reading over the reporting period"},
					}
					expectedNoteComponents := []redox.NoteComponent{
						{ID: "GLUCOSE_MANAGEMENT_INDICATOR", Name: "CGM Glucose Management Indicator during reporting period", Value: "6.7206", Comments: fmt.Sprintf("DateTime Observed: %s", reportingTime)},
					}
					noteComponents := redox.ObservationsToGMINoteComponents(observations)
					notes.SetComponents(noteComponents)
					Expect(notes.Note.Components).ToNot(BeNil())
					Expect(*notes.Note.Components).To(HaveLen(1))
					Expect(*notes.Note.Components).To(matchNoteComponents(expectedNoteComponents))
				})
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

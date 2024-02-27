package redox_test

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic-worker/redox"
	"github.com/tidepool-org/clinic-worker/test"
	models "github.com/tidepool-org/clinic/redox_models"
	"time"
)

var _ = Describe("Results", func() {
	Describe("NewResults", func() {
		It("returns a correctly instantiated result", func() {
			results := redox.NewResults()
			Expect(results.Meta.DataModel).To(Equal("Results"))
			Expect(results.Meta.EventType).To(Equal("New"))
			Expect(results.Meta.EventDateTime).ToNot(BeNil())

			eventDateTime, err := time.Parse(time.RFC3339, *results.Meta.EventDateTime)
			Expect(err).ToNot(HaveOccurred())
			Expect(eventDateTime).To(BeTemporally("~", time.Now(), 3*time.Second))
		})
	})

	Context("With order", func() {
		var results models.NewResults
		var order models.NewOrder

		BeforeEach(func() {
			results = redox.NewResults()
			fixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(fixture, &order)).To(Succeed())
		})

		Describe("SetResultsPatientFromOrder", func() {
			It("sets the patient identifier and demographics from the order", func() {
				redox.SetResultsPatientFromOrder(order, &results)
				Expect(results.Patient.Identifiers).To(Equal(order.Patient.Identifiers))
				Expect(results.Patient.Demographics).To(Equal(order.Patient.Demographics))
			})
		})

		Describe("SetMatchingResult", func() {
			var matchingResult redox.ResultsNotification

			It("sets the order id and status", func() {
				redox.SetMatchingResult(redox.ResultsNotification{
					IsSuccess: true,
					Message:   "success",
				}, order, &results)

				Expect(results.Orders).To(HaveLen(1))
				Expect(results.Orders[0].ID).To(Equal(order.Order.ID))
				Expect(results.Orders[0].Status).To(Equal("Resulted"))
			})

			When("matching result is success", func() {
				BeforeEach(func() {
					matchingResult = redox.ResultsNotification{
						IsSuccess: true,
						Message:   "Matched!",
					}
					redox.SetMatchingResult(matchingResult, order, &results)
				})

				It("sets the expected results", func() {
					Expect(results.Orders).To(HaveLen(1))

					Expect(results.Orders[0].Procedure).ToNot(BeNil())
					Expect(results.Orders[0].Procedure.Code).To(Equal(order.Order.Procedure.Code))
					Expect(results.Orders[0].Procedure.Codeset).To(Equal(order.Order.Procedure.Codeset))
					Expect(results.Orders[0].Procedure.Description).To(Equal(order.Order.Procedure.Description))

					Expect(results.Orders[0].CompletionDateTime).ToNot(PointTo(BeEmpty()))
					Expect(results.Orders[0].ResultsStatus).To(PointTo(Equal("Final")))

					Expect(results.Orders[0].Results).To(HaveLen(2))

					Expect(results.Orders[0].Results[0].Code).To(Equal("MATCHING_RESULT"))
					Expect(results.Orders[0].Results[0].CompletionDateTime).ToNot(PointTo(BeEmpty()))
					Expect(results.Orders[0].Results[0].Value).To(Equal("SUCCESS"))
					Expect(results.Orders[0].Results[0].ValueType).To(Equal("String"))
					Expect(results.Orders[0].Results[0].Description).To(PointTo(Equal("Indicates whether the order was successfully matched")))
					Expect(results.Orders[0].Results[0].Status).To(PointTo(Equal("Final")))

					Expect(results.Orders[0].Results[1].Code).To(Equal("MATCHING_RESULT_MESSAGE"))
					Expect(results.Orders[0].Results[1].CompletionDateTime).ToNot(PointTo(BeEmpty()))
					Expect(results.Orders[0].Results[1].Value).To(Equal("Matched!"))
					Expect(results.Orders[0].Results[1].ValueType).To(Equal("String"))
					Expect(results.Orders[0].Results[1].Description).To(PointTo(Equal("Message indicating the result of the matching process")))
					Expect(results.Orders[0].Results[1].Status).To(PointTo(Equal("Final")))

					Expect(results.Orders[0].Provider).To(PointTo(Equal(*order.Order.Provider)))
				})
			})

			When("matching result is failure", func() {
				BeforeEach(func() {
					matchingResult = redox.ResultsNotification{
						IsSuccess: false,
						Message:   "No patients matched!",
					}
					redox.SetMatchingResult(matchingResult, order, &results)
				})

				It("sets the expected results", func() {
					Expect(results.Orders).To(HaveLen(1))

					Expect(results.Orders[0].Procedure).ToNot(BeNil())
					Expect(results.Orders[0].Procedure.Code).To(Equal(order.Order.Procedure.Code))
					Expect(results.Orders[0].Procedure.Codeset).To(Equal(order.Order.Procedure.Codeset))
					Expect(results.Orders[0].Procedure.Description).To(Equal(order.Order.Procedure.Description))

					Expect(results.Orders[0].CompletionDateTime).ToNot(PointTo(BeEmpty()))
					Expect(results.Orders[0].ResultsStatus).To(PointTo(Equal("Final")))

					Expect(results.Orders[0].Results).To(HaveLen(2))
					Expect(results.Orders[0].Results[0].Code).To(Equal("MATCHING_RESULT"))
					Expect(results.Orders[0].Results[0].CompletionDateTime).ToNot(PointTo(BeEmpty()))
					Expect(results.Orders[0].Results[0].Value).To(Equal("FAILURE"))
					Expect(results.Orders[0].Results[0].ValueType).To(Equal("String"))
					Expect(results.Orders[0].Results[0].Description).To(PointTo(Equal("Indicates whether the order was successfully matched")))
					Expect(results.Orders[0].Results[0].Status).To(PointTo(Equal("Final")))

					Expect(results.Orders[0].Results[1].Code).To(Equal("MATCHING_RESULT_MESSAGE"))
					Expect(results.Orders[0].Results[1].CompletionDateTime).ToNot(PointTo(BeEmpty()))
					Expect(results.Orders[0].Results[1].Value).To(Equal("No patients matched!"))
					Expect(results.Orders[0].Results[1].ValueType).To(Equal("String"))
					Expect(results.Orders[0].Results[1].Description).To(PointTo(Equal("Message indicating the result of the matching process")))
					Expect(results.Orders[0].Results[1].Status).To(PointTo(Equal("Final")))

					Expect(results.Orders[0].Provider).To(PointTo(Equal(*order.Order.Provider)))
				})
			})
		})
	})

})

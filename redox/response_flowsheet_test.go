package redox_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic-worker/test"
	models "github.com/tidepool-org/clinic/redox_models"

	"github.com/tidepool-org/clinic-worker/redox"
)

var _ = Describe("Flowsheet", func() {
	Describe("NewFlowsheet", func() {
		It("returns a correctly instantiated flowsheet", func() {
			flowsheet := redox.NewFlowsheet()
			Expect(flowsheet.Meta.DataModel).To(Equal("Flowsheet"))
			Expect(flowsheet.Meta.EventType).To(Equal("New"))
			Expect(flowsheet.Meta.EventDateTime).ToNot(BeNil())

			eventDateTime, err := time.Parse(time.RFC3339, *flowsheet.Meta.EventDateTime)
			Expect(err).ToNot(HaveOccurred())
			Expect(eventDateTime).To(BeTemporally("~", time.Now(), 3*time.Second))
		})
	})

	Describe("AppendObservation", func() {
		It("maps a flowsheet entry onto an observation without modification", func() {
			flowsheet := redox.NewFlowsheet()
			units := "%"
			redox.AppendObservation(&flowsheet, &redox.Observation{
				Code:        "TIME_IN_RANGE_CGM",
				Value:       "56.2871",
				ValueType:   "Numeric",
				Units:       &units,
				DateTime:    "2023-06-22T23:44:16Z",
				Description: "CGM Time in Range",
			})

			Expect(flowsheet.Observations).To(HaveExactElements(MatchFields(IgnoreExtras, Fields{
				"Code":        Equal("TIME_IN_RANGE_CGM"),
				"Value":       Equal("56.2871"),
				"ValueType":   Equal("Numeric"),
				"Units":       PointTo(Equal("%")),
				"Description": PointTo(Equal("CGM Time in Range")),
				"DateTime":    Equal("2023-06-22T23:44:16Z"),
			})))
		})
	})

	Describe("ObservationsToGMINoteComponents", func() {
		It("extracts only the GMI observation as a note component", func() {
			units := "%"
			observations := []*redox.Observation{
				{Code: "TIME_IN_RANGE_CGM", Value: "56.2871", ValueType: "Numeric", Units: &units, DateTime: "2023-06-22T23:44:16Z", Description: "CGM Time in Range"},
				{Code: "GLUCOSE_MANAGEMENT_INDICATOR", Value: "6.7206", ValueType: "Numeric", DateTime: "2023-06-22T23:44:16Z", Description: "CGM Glucose Management Indicator during reporting period"},
			}

			components := redox.ObservationsToGMINoteComponents(observations)
			Expect(components).To(HaveLen(1))
			Expect(components[0].ID).To(Equal("GLUCOSE_MANAGEMENT_INDICATOR"))
			Expect(components[0].Value).To(Equal("6.7206"))
			Expect(components[0].Name).To(Equal("CGM Glucose Management Indicator during reporting period"))
			Expect(components[0].Comments).To(Equal("DateTime Observed: 2023-06-22T23:44:16Z"))
		})
	})

	Describe("SetVisitNumberInFlowsheet", func() {
		var flowsheet models.NewFlowsheet
		var order models.NewOrder

		BeforeEach(func() {
			flowsheet = redox.NewFlowsheet()
			fixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(fixture, &order)).To(Succeed())
		})

		It("sets the visit number from the order", func() {
			redox.SetVisitNumberInFlowsheet(order, &flowsheet)
			Expect(flowsheet.Visit).ToNot(BeNil())
			Expect(flowsheet.Visit.VisitNumber).To(PointTo(Equal(*order.Visit.VisitNumber)))
		})

		Describe("SetVisitLocationFromOrder", func() {
			It("sets the visit location from the order", func() {
				redox.SetVisitLocationInFlowsheet(order, &flowsheet)
				Expect(flowsheet.Visit).ToNot(BeNil())
				Expect(flowsheet.Visit.Location).To(PointTo(Equal(*order.Visit.Location)))
			})
		})

		Describe("SetProviderInFlowsheet", func() {
			It("sets the provider extension from the order", func() {
				expectedProviderExtension := Fields{
					"URL": Equal("https://api.redoxengine.com/extensions/additional-provider-info"),
					"Participants": ContainElements(MatchFields(IgnoreExtras, Fields{
						"Id":     Equal("4356789876"),
						"IdType": Equal("NPI"),
						"Person": MatchFields(IgnoreExtras, Fields{
							"Name": MatchFields(IgnoreExtras, Fields{
								"Given":  ConsistOf(Equal("Pat")),
								"Family": Equal("Granite"),
							}),
						}),
					})),
				}
				redox.SetProviderInFlowsheet(order, &flowsheet)
				Expect(flowsheet.Visit).ToNot(BeNil())
				Expect(flowsheet.Visit.Extensions).To(PointTo(HaveKeyWithValue("additional-provider-info", MatchFields(IgnoreExtras, expectedProviderExtension))))
			})
		})
	})
})

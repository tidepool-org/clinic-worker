package patients_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/patients"
	"github.com/tidepool-org/clinic-worker/test"
	"strings"
)

var _ = Describe("PatientCDCConsumer", func() {
	Describe("Unmarshal", func() {
		It("unmarshals events successfully", func() {
			fixture, err := test.LoadFixture("test/fixtures/patient_event.txt")
			Expect(err).ToNot(HaveOccurred())

			// Some editors add a new line at the end of the file by default, remove it
			fixture = []byte(strings.TrimSuffix(string(fixture), "\n"))

			event := patients.PatientCDCEvent{}
			err = patients.UnmarshalEvent(fixture, &event)
			Expect(err).ToNot(HaveOccurred())

			// Make sure cdc.Date is parsed correctly
			Expect(event.FullDocument.LastRequestedDexcomConnectTime).ToNot(BeNil())
			Expect(event.FullDocument.LastRequestedDexcomConnectTime.Value).To(Equal(int64(1725664480753)))
		})
	})

	Describe("", func() {
		It("returns only the added provider connection request", func() {
			fixture, err := test.LoadFixture("test/fixtures/provider_connection_request.txt")
			Expect(err).ToNot(HaveOccurred())

			// Some editors add a new line at the end of the file by default, remove it
			fixture = []byte(strings.TrimSuffix(string(fixture), "\n"))

			event := patients.PatientCDCEvent{}
			err = patients.UnmarshalEvent(fixture, &event)
			Expect(err).ToNot(HaveOccurred())

			requests := event.UpdateDescription.UpdatedFields.GetUpdatedConnectionRequests()
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].ProviderName).To(Equal("dexcom"))
		})

		It("returns only the added tandem provider connection request", func() {
			fixture, err := test.LoadFixture("test/fixtures/provider_connection_request_tandem.txt")
			Expect(err).ToNot(HaveOccurred())
			fixture = []byte(strings.TrimSuffix(string(fixture), "\n"))

			event := patients.PatientCDCEvent{}
			err = patients.UnmarshalEvent(fixture, &event)
			Expect(err).ToNot(HaveOccurred())

			requests := event.UpdateDescription.UpdatedFields.GetUpdatedConnectionRequests()
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].ProviderName).To(Equal("tandem"))
			Expect(event.FullDocument.ProviderConnectEmailTemplate(requests[0].ProviderName)).To(Equal("request_tandem_connect_custodial"))
		})
	})

	Describe("ProviderConnectEmailTemplate", func() {
		custodian := patients.Permission{}

		DescribeTable("selects the template from the custodial status and data source state",
			func(patient patients.Patient, expected string) {
				Expect(patient.ProviderConnectEmailTemplate("tandem")).To(Equal(expected))
			},
			Entry("connect", patients.Patient{
				DataSources: &[]patients.PatientDataSource{{ProviderName: pointer("tandem"), State: pointer("pending")}},
			}, "request_tandem_connect"),
			Entry("connect custodial", patients.Patient{
				Permissions: &patients.Permissions{Custodian: &custodian},
				DataSources: &[]patients.PatientDataSource{{ProviderName: pointer("tandem"), State: pointer("pending")}},
			}, "request_tandem_connect_custodial"),
			Entry("reconnect", patients.Patient{
				Permissions: &patients.Permissions{Custodian: &custodian},
				DataSources: &[]patients.PatientDataSource{{ProviderName: pointer("tandem"), State: pointer("pendingReconnect")}},
			}, "request_tandem_reconnect"),
			Entry("reconnect of another provider", patients.Patient{
				DataSources: &[]patients.PatientDataSource{
					{ProviderName: pointer("dexcom"), State: pointer("pendingReconnect")},
					{ProviderName: pointer("tandem"), State: pointer("pending")},
				},
			}, "request_tandem_connect"),
		)
	})
})

func pointer(value string) *string {
	return &value
}

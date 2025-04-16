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
	})
})

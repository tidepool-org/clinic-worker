package patients_test

import (
	"bytes"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/patients"
	"github.com/tidepool-org/clinic-worker/test"
)

var _ = Describe("PatientCDCConsumer", func() {
	Describe("Unmarshal", func() {
		It("unmarshals events successfully", func() {
			fixture, err := test.LoadFixture("test/fixtures/patient_event.txt")
			Expect(err).ToNot(HaveOccurred())

			// Some editors add a new line at the end of the file by default, remove it
			fixture = bytes.TrimSpace(fixture)

			event := patients.PatientCDCEvent{}
			err = patients.UnmarshalEvent(fixture, &event)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("connection request is modified", func() {
			It("can read migratedTime", func() {
				fixture, err := test.LoadFixture("test/fixtures/patient_event_conn_modified.txt")
				Expect(err).ToNot(HaveOccurred())

				// Some editors add a new line at the end of the file by default, remove it
				fixture = []byte(bytes.TrimSpace(fixture))

				event := patients.PatientCDCEvent{}
				err = patients.UnmarshalEvent(fixture, &event)
				Expect(err).ToNot(HaveOccurred())

				exp := patients.ConnectionRequest{
					MigratedTime: &cdc.Date{
						Value: 1728059814765,
					},
					ProviderName: "dexcom",
					CreatedTime: cdc.Date{
						Value: 1728059814765,
					},
				}

				pcrs := event.UpdateDescription.UpdatedFields.ProviderConnectionRequestsDexcom
				Expect(pcrs[0]).To(Equal(exp))
			})
		})

		Context("connection request is newly created", func() {
			It("can read migratedTime", func() {
				fixture, err := test.LoadFixture("test/fixtures/patient_event_new_conn.txt")
				Expect(err).ToNot(HaveOccurred())

				// Some editors add a new line at the end of the file by default, remove it
				fixture = []byte(bytes.TrimSpace(fixture))

				event := patients.PatientCDCEvent{}
				err = patients.UnmarshalEvent(fixture, &event)
				Expect(err).ToNot(HaveOccurred())

				exp := patients.ConnectionRequest{
					MigratedTime: &cdc.Date{
						Value: 1728059814765,
					},
					ProviderName: "dexcom",
					CreatedTime: cdc.Date{
						Value: 1728059814765,
					},
				}

				pcrs := event.UpdateDescription.UpdatedFields.ProviderConnectionRequests["dexcom"]
				Expect(pcrs[0]).To(Equal(exp))
			})
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

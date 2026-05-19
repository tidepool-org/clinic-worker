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

	Describe("IsConnectionRequestMigration", func() {
		loadEvent := func(path string) patients.PatientCDCEvent {
			GinkgoHelper()
			fixture, err := test.LoadFixture(path)
			Expect(err).ToNot(HaveOccurred())

			event := patients.PatientCDCEvent{}
			err = patients.UnmarshalEvent(bytes.TrimSpace(fixture), &event)
			Expect(err).ToNot(HaveOccurred())
			return event
		}

		DescribeTable("detects migrations",
			func(path string, expected bool) {
				event := loadEvent(path)
				updated := event.UpdateDescription.UpdatedFields
				Expect(updated.IsConnectionRequestMigration()).To(Equal(expected))
			},
			Entry("when a connection request is newly created by a migration",
				"test/fixtures/patient_event_new_conn.txt", true),
			Entry("when a connection request is modified by a migration",
				"test/fixtures/patient_event_conn_modified.txt", true),
			Entry("when a migration also rewrites other providers' connection requests",
				"test/fixtures/patient_event_migration_multi_provider.txt", true),
			Entry("unless the connection request was added after a migration",
				"test/fixtures/patient_event_conn_added_after_migration.txt", false),
			Entry("unless the connection request was added without a migration",
				"test/fixtures/provider_connection_request.txt", false),
		)

		It("reports other providers' connection requests during a migration", func() {
			// Without the migration check, these would each result in an email.
			event := loadEvent("test/fixtures/patient_event_migration_multi_provider.txt")
			requests := event.UpdateDescription.UpdatedFields.GetUpdatedConnectionRequests()
			providers := []string{}
			for _, r := range requests {
				providers = append(providers, r.ProviderName)
			}
			Expect(providers).To(ConsistOf("dexcom", "abbott"))
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

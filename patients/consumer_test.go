package patients_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/cdc"
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

	Describe("GlycemicRangesChanged", func() {
		newEvent := func(operationType string, updated []string,
			removed []string) patients.PatientCDCEvent {

			userId := "b650bb9b-34ee-4c5d-aff0-bd149084c658"
			return patients.PatientCDCEvent{
				OperationType: operationType,
				FullDocument:  patients.Patient{UserId: &userId},
				UpdateDescription: patients.UpdateDescription{
					UpdatedFieldNames: updated,
					RemovedFields:     removed,
				},
			}
		}

		DescribeTable("reports whether the event changes the glycemic ranges",
			func(event patients.PatientCDCEvent, expected bool) {
				Expect(event.GlycemicRangesChanged()).To(Equal(expected))
			},
			Entry("an update of the whole field",
				newEvent(cdc.OperationTypeUpdate, []string{"glycemicRanges"}, nil), true),
			Entry("an update of a part of the field",
				newEvent(cdc.OperationTypeUpdate,
					[]string{"glycemicRanges.custom.thresholds.low"}, nil), true),
			Entry("an update removing a part of the field",
				newEvent(cdc.OperationTypeUpdate, nil, []string{"glycemicRanges.custom"}),
				true),
			Entry("an update removing the field",
				newEvent(cdc.OperationTypeUpdate, nil, []string{"glycemicRanges"}), true),
			Entry("an update of other fields",
				newEvent(cdc.OperationTypeUpdate, []string{"mrn", "summary.cgmStats"},
					[]string{"email"}), false),
			Entry("an update of a field whose name only starts the same",
				newEvent(cdc.OperationTypeUpdate, []string{"glycemicRangesKey"}, nil),
				false),
			// A replacement doesn't describe what changed
			Entry("a replacement", newEvent(cdc.OperationTypeReplace, nil, nil), true),
			// A sync is requested for a record created without a summary, whatever its
			// ranges
			Entry("an insert",
				newEvent(cdc.OperationTypeInsert, []string{"glycemicRanges"}, nil), false),
			Entry("a delete", newEvent(cdc.OperationTypeDelete, nil, nil), false),
		)

		It("reports no change for a record without a user", func() {
			event := newEvent(cdc.OperationTypeUpdate, []string{"glycemicRanges"}, nil)
			event.FullDocument.UserId = nil
			Expect(event.GlycemicRangesChanged()).To(BeFalse())
		})

		It("recognizes a change in an unmarshaled event", func() {
			fixture, err := test.LoadFixture("test/fixtures/glycemic_ranges_update.txt")
			Expect(err).ToNot(HaveOccurred())
			fixture = []byte(strings.TrimSuffix(string(fixture), "\n"))

			event := patients.PatientCDCEvent{}
			Expect(patients.UnmarshalEvent(fixture, &event)).To(Succeed())
			Expect(event.UpdateDescription.UpdatedFieldNames).To(Equal(
				[]string{"glycemicRanges.preset", "glycemicRanges.type", "updatedTime"}))
			Expect(event.GlycemicRangesChanged()).To(BeTrue())
		})

		It("recognizes no change in an unmarshaled event updating other fields", func() {
			fixture, err := test.LoadFixture("test/fixtures/provider_connection_request.txt")
			Expect(err).ToNot(HaveOccurred())
			fixture = []byte(strings.TrimSuffix(string(fixture), "\n"))

			event := patients.PatientCDCEvent{}
			Expect(patients.UnmarshalEvent(fixture, &event)).To(Succeed())
			Expect(event.UpdateDescription.UpdatedFieldNames).ToNot(BeEmpty())
			Expect(event.GlycemicRangesChanged()).To(BeFalse())
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

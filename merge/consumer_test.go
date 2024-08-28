package merge_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/merge"
	"github.com/tidepool-org/clinic-worker/test"
	"go.mongodb.org/mongo-driver/bson"
)

var _ = Describe("NewMergePlansConsumerCDCConsumer", func() {
	Describe("Unmarshal", func() {
		It("unmarshals events successfully", func() {
			fixture, err := test.LoadFixture("test/fixtures/patient_plan_event.json")
			Expect(err).ToNot(HaveOccurred())

			event := cdc.Event[merge.PersistentPlan[bson.Raw]]{}
			Expect(merge.UnmarshalEvent(fixture, &event)).To(Succeed())
		})

		It("unmarshals plans successfully", func() {
			fixture, err := test.LoadFixture("test/fixtures/patient_plan_event.json")
			Expect(err).ToNot(HaveOccurred())

			event := cdc.Event[merge.PersistentPlan[bson.Raw]]{}
			Expect(merge.UnmarshalEvent(fixture, &event)).To(Succeed())

			plan := merge.PatientPlan{}
			Expect(merge.UnmarshalPlan(event, &plan)).To(Succeed())

			Expect(plan.SourcePatient).ToNot(BeNil())
			Expect(plan.SourcePatient.UserId).ToNot(BeNil())
			Expect(plan.SourcePatient.Id).ToNot(BeNil())
			Expect(plan.SourcePatient.Id.Value).To(Equal("66ceef8d03b01ff45f5e7d81"))
		})
	})
})

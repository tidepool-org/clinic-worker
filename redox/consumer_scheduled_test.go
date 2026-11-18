package redox_test

import (
	"bytes"
	"time"

	"github.com/IBM/sarama"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/redox"
	redoxTest "github.com/tidepool-org/clinic-worker/redox/test"
	"github.com/tidepool-org/clinic-worker/test"
	"github.com/tidepool-org/go-common/events"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

var _ = Describe("Consumer Scheduled", func() {
	Describe("UnmarshalEvent", func() {
		It("works correctly", func() {
			eventFxiture, err := test.LoadFixture("test/fixtures/scheduledorderevent.json")
			Expect(err).ToNot(HaveOccurred())

			event := cdc.Event[redox.ScheduledSummaryAndReport]{}
			Expect(redox.UnmarshalEvent(eventFxiture, &event)).To(Succeed())

			Expect(event.FullDocument).ToNot(BeNil())
			Expect(event.FullDocument.LastMatchedOrder.Meta.IsValid()).To(BeTrue())
			Expect(event.FullDocument.LastMatchedOrder.Meta.DataModel).To(Equal("Order"))
			Expect(event.FullDocument.LastMatchedOrder.Meta.EventType).To(Equal("New"))
		})
	})

	Describe("UnmarshalEvent of an order rescheduled after a change of glycemic ranges", func() {
		It("decodes the time the summary changed ranges", func() {
			fixture, err := test.LoadFixture("test/fixtures/scheduledorderevent.json")
			Expect(err).ToNot(HaveOccurred())

			event := cdc.Event[redox.ScheduledSummaryAndReport]{}
			Expect(redox.UnmarshalEvent(fixture, &event)).To(Succeed())
			Expect(event.FullDocument).ToNot(BeNil())
			Expect(event.FullDocument.GlycemicRangesChangedTime).To(BeNil())

			withTime := bytes.Replace(fixture, []byte(`"createdTime": {`),
				[]byte(`"glycemicRangesChangedTime": {"$date": {"$numberLong": `+
					`"1697180000000"}}, "createdTime": {`), 1)
			Expect(withTime).ToNot(Equal(fixture))

			event = cdc.Event[redox.ScheduledSummaryAndReport]{}
			Expect(redox.UnmarshalEvent(withTime, &event)).To(Succeed())
			Expect(event.FullDocument).ToNot(BeNil())
			Expect(event.FullDocument.GlycemicRangesChangedTime).To(HaveValue(
				BeTemporally("==", time.UnixMilli(1697180000000))))
			Expect(event.FullDocument.CreatedTime).To(
				BeTemporally("==", time.UnixMilli(1697180977107)))
		})
	})

	Describe("Handle Kafka Message", func() {
		var consumer events.MessageConsumer
		var processor *redoxTest.ScheduledOrderProcessor

		BeforeEach(func() {
			processor = &redoxTest.ScheduledOrderProcessor{}

			var err error
			consumer, err = redox.NewScheduledSummaryAndReportsCDCConsumer(redox.ScheduledSummaryAndReportsCDCConsumerParams{
				Logger: zap.NewNop().Sugar(),
				Config: redox.ModuleConfig{
					Enabled: true,
				},
				Processor: processor,
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("uses the processor for handling valid messages", func() {
			message, err := test.LoadFixture("test/fixtures/scheduledorderevent.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(consumer.HandleKafkaMessage(&sarama.ConsumerMessage{
				Value: message,
			})).To(Succeed())
			Expect(processor.Scheduled).To(HaveLen(1))

			expectedId, _ := primitive.ObjectIDFromHex("6528ed3121d14252a7855a60")
			Expect(processor.Scheduled[0].Id).To(Equal(expectedId))
		})

		It("skips messages with invalid metadata", func() {
			message, err := test.LoadFixture("test/fixtures/invalidscheduledevent.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(consumer.HandleKafkaMessage(&sarama.ConsumerMessage{
				Value: message,
			})).To(Succeed())
			Expect(processor.Scheduled).To(BeEmpty())
		})
	})
})

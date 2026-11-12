package patients

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	summaries "github.com/tidepool-org/go-common/clients/summary"

	"github.com/tidepool-org/clinic-worker/cdc"
)

var _ = Describe("PatientCDCConsumer summary sync", func() {
	const userId = "b650bb9b-34ee-4c5d-aff0-bd149084c658"

	var server *httptest.Server
	var requests []string
	var responseStatusCode int
	var consumer *PatientCDCConsumer

	BeforeEach(func() {
		requests = nil
		responseStatusCode = http.StatusAccepted
		handler := func(res http.ResponseWriter, req *http.Request) {
			requests = append(requests, req.Method+" "+req.URL.Path)
			res.WriteHeader(responseStatusCode)
		}
		server = httptest.NewServer(http.HandlerFunc(handler))

		client, err := summaries.NewClientWithResponses(server.URL)
		Expect(err).ToNot(HaveOccurred())
		consumer = &PatientCDCConsumer{logger: zap.NewNop().Sugar(), summaries: client}
	})

	AfterEach(func() {
		server.Close()
	})

	newEvent := func(operationType string) PatientCDCEvent {
		id := userId
		return PatientCDCEvent{
			OperationType: operationType,
			FullDocument:  Patient{UserId: &id},
		}
	}

	sync := "POST /v1/summaries/" + userId + "/sync"

	It("requests a sync when a patient record is created without a summary", func() {
		event := newEvent(cdc.OperationTypeInsert)

		Expect(consumer.syncSummariesIfNeeded(context.Background(), event)).To(Succeed())
		Expect(requests).To(Equal([]string{sync}))
	})

	It("requests no sync when a patient record is created with a summary", func() {
		event := newEvent(cdc.OperationTypeInsert)
		event.FullDocument.Summary = &PatientSummary{}

		Expect(consumer.syncSummariesIfNeeded(context.Background(), event)).To(Succeed())
		Expect(requests).To(BeEmpty())
	})

	// The summary of the record was computed for the ranges it had before
	It("requests a sync when the glycemic ranges of a patient record change", func() {
		event := newEvent(cdc.OperationTypeUpdate)
		event.UpdateDescription.UpdatedFieldNames = []string{"glycemicRanges.preset"}

		Expect(consumer.handleCDCEvent(event)).To(Succeed())
		Expect(requests).To(Equal([]string{sync}))
	})

	It("requests no sync when an update leaves the glycemic ranges alone", func() {
		event := newEvent(cdc.OperationTypeUpdate)
		event.UpdateDescription.UpdatedFieldNames = []string{"mrn"}

		Expect(consumer.handleCDCEvent(event)).To(Succeed())
		Expect(requests).To(BeEmpty())
	})

	// The error has the event retried
	It("returns an error when the data service doesn't accept the request", func() {
		responseStatusCode = http.StatusInternalServerError
		event := newEvent(cdc.OperationTypeUpdate)
		event.UpdateDescription.UpdatedFieldNames = []string{"glycemicRanges"}

		err := consumer.handleCDCEvent(event)
		Expect(err).To(MatchError(ContainSubstring("unexpected status code")))
	})
})

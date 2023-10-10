package redox_test

import (
	"context"
	"encoding/json"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic-worker/redox"
	testRedox "github.com/tidepool-org/clinic-worker/redox/test"
	"github.com/tidepool-org/clinic-worker/report"
	"github.com/tidepool-org/clinic-worker/test"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"net/http"
)

var _ = Describe("ScheduledSummaryAndReportProcessor", func() {
	var redoxClient *testRedox.RedoxClient
	var clinicCtrl *gomock.Controller
	var clinicClient *clinics.MockClientWithResponsesInterface
	var scheduledProcessor redox.ScheduledSummaryAndReportProcessor

	BeforeEach(func() {
		redoxClient = testRedox.NewTestRedoxClient("testSourceId", "testSourceName")
		clinicCtrl = gomock.NewController(GinkgoT())
		clinicClient = clinics.NewMockClientWithResponsesInterface(clinicCtrl)
		processor := redox.NewNewOrderProcessor(clinicClient, redoxClient, report.NewSampleReportGenerator(), zap.NewNop().Sugar())
		scheduledProcessor = redox.NewScheduledSummaryAndReportProcessor(processor, clinicClient, zap.NewNop().Sugar())
	})

	Describe("ProcessOrder", func() {
		var order models.NewOrder
		var scheduled redox.ScheduledSummaryAndReport

		BeforeEach(func() {
			response := &clinics.EHRMatchResponse{}
			matchFixture, err := test.LoadFixture("test/fixtures/subscriptionmatchresponse.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(matchFixture, response)).To(Succeed())

			clinicClient.EXPECT().
				GetClinicWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&clinics.GetClinicResponse{
					Body:         nil,
					HTTPResponse: &http.Response{StatusCode: http.StatusOK},
					JSON200:      &response.Clinic,
				}, nil)

			clinicClient.EXPECT().
				GetPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&clinics.GetPatientResponse{
					Body:         nil,
					HTTPResponse: &http.Response{StatusCode: http.StatusOK},
					JSON200:      &((*response.Patients)[0]),
				}, nil)

			clinicClient.EXPECT().
				GetEHRSettingsWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&clinics.GetEHRSettingsResponse{
					Body:         nil,
					HTTPResponse: &http.Response{StatusCode: http.StatusOK},
					JSON200:      &response.Settings,
				}, nil)

			newOrderFixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(newOrderFixture, &order)).To(Succeed())

			message := bson.Raw{}
			Expect(bson.UnmarshalExtJSON(newOrderFixture, true, &message)).To(Succeed())

			envelope := models.MessageEnvelope{
				Id:      primitive.NewObjectID(),
				Meta:    order.Meta,
				Message: message,
			}

			clinicId := *response.Clinic.Id
			clinicObjectId, err := primitive.ObjectIDFromHex(clinicId)
			Expect(err).ToNot(HaveOccurred())

			scheduled = redox.ScheduledSummaryAndReport{
				UserId:           *(*response.Patients)[0].Id,
				ClinicId:         clinicObjectId,
				LastMatchedOrder: envelope,
				DecodedOrder:     order,
			}
		})

		It("send flowsheet and notes when patient and clinic successfully matched", func() {
			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))

			var notes models.NewNotes
			var flowsheet models.NewFlowsheet

			for _, payload := range redoxClient.Sent {
				switch payload.(type) {
				case models.NewNotes:
					notes = payload.(models.NewNotes)
				case models.NewFlowsheet:
					flowsheet = payload.(models.NewFlowsheet)
				}
			}

			Expect(flowsheet).To(MatchFields(IgnoreExtras, Fields{
				"Meta": MatchFields(IgnoreExtras, Fields{
					"DataModel": Equal("Flowsheet"),
					"EventType": Equal("New"),
				}),
			}))

			Expect(notes).To(MatchFields(IgnoreExtras, Fields{
				"Meta": MatchFields(IgnoreExtras, Fields{
					"DataModel": Equal("Notes"),
					"EventType": Equal("New"),
				}),
				"Note": MatchFields(IgnoreExtras, Fields{
					"FileContents": PointTo(Not(BeEmpty())),
				}),
			}))

			Expect(redoxClient.Uploaded).To(BeEmpty())
		})

		It("uploads a file and references it in the note when upload api is enabled", func() {
			redoxClient.SetUploadFileEnabled(true)
			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))

			var notes models.NewNotes

			for _, payload := range redoxClient.Sent {
				switch payload.(type) {
				case models.NewNotes:
					notes = payload.(models.NewNotes)
				}
			}

			Expect(redoxClient.Uploaded).To(HaveKey("report.pdf"))
			Expect(notes).To(MatchFields(IgnoreExtras, Fields{
				"Meta": MatchFields(IgnoreExtras, Fields{
					"DataModel": Equal("Notes"),
					"EventType": Equal("New"),
				}),
				"Note": MatchFields(IgnoreExtras, Fields{
					"FileContents": PointTo(Equal("https://blob.redoxengine.com/upload/report.pdf")),
				}),
			}))

		})
	})
})

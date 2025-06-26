package redox_test

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic-worker/redox"
	testRedox "github.com/tidepool-org/clinic-worker/redox/test"
	"github.com/tidepool-org/clinic-worker/report"
	"github.com/tidepool-org/clinic-worker/test"
	clinics "github.com/tidepool-org/clinic/client"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
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
		shorelineClient := shoreline.NewMock("test")
		processor := redox.NewNewOrderProcessor(clinicClient, redoxClient, report.NewSampleReportGenerator(), shorelineClient, zap.NewNop().Sugar())
		scheduledProcessor = redox.NewScheduledSummaryAndReportProcessor(processor, clinicClient, zap.NewNop().Sugar())
	})

	Describe("ProcessOrder", func() {
		var order models.NewOrder
		var scheduled redox.ScheduledSummaryAndReport
		var patient *clinics.PatientV1

		BeforeEach(func() {
			response := &clinics.EhrMatchResponseV1{}
			matchFixture, err := test.LoadFixture("test/fixtures/subscriptionmatchresponse.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(matchFixture, response)).To(Succeed())

			patient = &(*response.Patients)[0]
			// Make sure we're not ignoring the scheduled order when
			// the user's last upload data is too far back
			now := time.Now()
			patient.Summary.CgmStats.Dates.LastUploadDate = &now
			patient.Summary.BgmStats.Dates.LastUploadDate = &now

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
					JSON200:      patient,
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
				UserId:           *patient.Id,
				ClinicId:         clinicObjectId,
				LastMatchedOrder: envelope,
				DecodedOrder:     order,
			}
		})

		It("send flowsheet and notes when patient and clinic successfully matched", func() {
			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))

			var notes redox.Notes
			var flowsheet models.NewFlowsheet

			for _, payload := range redoxClient.Sent {
				switch payload.(type) {
				case redox.Notes:
					notes = payload.(redox.Notes)
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

			Expect(notes).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Meta": MatchFields(IgnoreExtras, Fields{
					"DataModel": Equal("Notes"),
					"EventType": Equal("New"),
				}),
				"Note": MatchFields(IgnoreExtras, Fields{
					"FileContents": PointTo(Not(BeEmpty())),
				}),
			})))

			Expect(redoxClient.Uploaded).To(BeEmpty())
		})

		It("it sends a new flowsheet and replaces notes when there is a preceding document and clinic settings are configured for note replacement", func() {
			scheduled.Id = primitive.NewObjectID()
			scheduled.PrecedingDocument = &redox.PrecedingDocument{
				Id:          primitive.NewObjectID(),
				CreatedTime: time.Now().Add(-4 * time.Minute),
			}

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))

			var notes redox.Notes
			var flowsheet models.NewFlowsheet

			for _, payload := range redoxClient.Sent {
				switch payload.(type) {
				case redox.Notes:
					notes = payload.(redox.Notes)
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

			Expect(notes).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Meta": MatchFields(IgnoreExtras, Fields{
					"DataModel": Equal("Notes"),
					"EventType": Equal("Replace"),
				}),
				"Note": MatchFields(IgnoreExtras, Fields{
					"DocumentID":         Equal(scheduled.Id.Hex()),
					"OriginalDocumentID": Equal(scheduled.PrecedingDocument.Id.Hex()),
					"FileContents":       PointTo(Not(BeEmpty())),
				}),
			})))

			Expect(redoxClient.Uploaded).To(BeEmpty())
		})

		It("uploads a file and references it in the note when upload api is enabled", func() {
			redoxClient.SetUploadFileEnabled(true)
			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))

			var notes redox.Notes

			for _, payload := range redoxClient.Sent {
				switch payload.(type) {
				case redox.Notes:
					notes = payload.(redox.Notes)
				}
			}

			Expect(redoxClient.Uploaded).To(HaveKey("report.pdf"))
			Expect(notes).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Meta": MatchFields(IgnoreExtras, Fields{
					"DataModel": Equal("Notes"),
					"EventType": Equal("New"),
				}),
				"Note": MatchFields(IgnoreExtras, Fields{
					"FileContents": PointTo(Equal("https://blob.redoxengine.com/upload/report.pdf")),
				}),
			})))

		})

		It("sends documents if the original order doesn't have event date time", func() {
			scheduled.DecodedOrder.Meta.EventDateTime = nil

			afterOrderTime := time.Now().Add(-10 * 24 * time.Hour)
			patient.Summary.CgmStats.Dates.LastUploadDate = &afterOrderTime
			patient.Summary.BgmStats.Dates.LastUploadDate = &afterOrderTime

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))
		})

		It("sends documents if the original order's event date time can't be parsed", func() {
			orderTime := "2099-AA"
			scheduled.DecodedOrder.Meta.EventDateTime = &orderTime

			afterOrderTime := time.Now().Add(-10 * 24 * time.Hour)
			patient.Summary.CgmStats.Dates.LastUploadDate = &afterOrderTime
			patient.Summary.BgmStats.Dates.LastUploadDate = &afterOrderTime

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))
		})

		It("sends documents if last upload date is after the original order time", func() {
			orderTime := time.Now().Add(-15 * 24 * time.Hour).UTC().Format(time.RFC3339)
			scheduled.DecodedOrder.Meta.EventDateTime = &orderTime

			afterOrderTime := time.Now().Add(-10 * 24 * time.Hour)
			patient.Summary.CgmStats.Dates.LastUploadDate = &afterOrderTime
			patient.Summary.BgmStats.Dates.LastUploadDate = &afterOrderTime

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))
		})

		It("doesn't send documents if last upload date is before last scheduled document", func() {
			scheduled.PrecedingDocument = &redox.PrecedingDocument{
				Id:          primitive.NewObjectID(),
				CreatedTime: time.Now().Add(-10 * 24 * time.Hour),
			}
			beforeLastScheduledDocument := scheduled.PrecedingDocument.CreatedTime.Add(-1 * time.Hour)
			patient.Summary.CgmStats.Dates.LastUploadDate = &beforeLastScheduledDocument
			patient.Summary.BgmStats.Dates.LastUploadDate = &beforeLastScheduledDocument

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(0))
		})

		It("sends documents if bgm upload date is before the cutoff but cgm after", func() {
			now := time.Now()
			scheduled.PrecedingDocument = &redox.PrecedingDocument{
				Id:          primitive.NewObjectID(),
				CreatedTime: time.Now().Add(-10 * 24 * time.Hour),
			}
			beforeCutoff := scheduled.PrecedingDocument.CreatedTime.Add(-1 * time.Hour)
			patient.Summary.CgmStats.Dates.LastUploadDate = &now
			patient.Summary.BgmStats.Dates.LastUploadDate = &beforeCutoff

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))
		})

		It("sends documents if cgm upload date is before the cutoff but bgm after", func() {
			now := time.Now()
			scheduled.PrecedingDocument = &redox.PrecedingDocument{
				Id:          primitive.NewObjectID(),
				CreatedTime: time.Now().Add(-10 * 24 * time.Hour),
			}
			beforeCutoff := scheduled.PrecedingDocument.CreatedTime.Add(-1 * time.Hour)
			patient.Summary.CgmStats.Dates.LastUploadDate = &beforeCutoff
			patient.Summary.BgmStats.Dates.LastUploadDate = &now

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
			Expect(redoxClient.Sent).To(HaveLen(2))
		})

		It("succeeds if cgm stats is nil", func() {
			now := time.Now()
			patient.Summary.CgmStats = nil
			patient.Summary.BgmStats.Dates.LastUploadDate = &now

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
		})

		It("succeeds if bgm stats is nil", func() {
			now := time.Now()
			patient.Summary.CgmStats.Dates.LastUploadDate = &now
			patient.Summary.BgmStats = nil

			Expect(scheduledProcessor.ProcessOrder(context.Background(), scheduled)).To(Succeed())
		})
	})

	Describe("ShouldReplacePrecedingReport", func() {
		var response *clinics.EhrMatchResponseV1
		var order *models.NewOrder

		BeforeEach(func() {
			response = &clinics.EhrMatchResponseV1{}
			matchFixture, err := test.LoadFixture("test/fixtures/subscriptionmatchresponse.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(matchFixture, response)).To(Succeed())

			newOrderFixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(newOrderFixture, &order)).To(Succeed())
		})

		It("Should be false when there's no preceding document and clinic is not configured for replacement", func() {
			params := redox.SummaryAndReportParameters{
				Match:      *response,
				Order:      *order,
				DocumentId: "1234567",
			}
			Expect(params.ShouldReplacePrecedingReport()).To(BeFalse())
		})

		It("Should be false when there's is preceding document and clinic is not configured for replacement", func() {
			eventType := clinics.ScheduledReportsV1OnUploadNoteEventTypeNew
			response.Settings.ScheduledReports.OnUploadNoteEventType = &eventType
			params := redox.SummaryAndReportParameters{
				Match:      *response,
				Order:      *order,
				DocumentId: "1234567",
				PrecedingDocument: &redox.PrecedingDocument{
					Id:          primitive.NewObjectID(),
					CreatedTime: time.Now().Add(-time.Minute),
				},
			}
			Expect(params.ShouldReplacePrecedingReport()).To(BeFalse())
		})

		It("Should be false if there's is preceding document created more than 5 minutes ago and the clinic is configured for replacement", func() {
			eventType := clinics.ScheduledReportsV1OnUploadNoteEventTypeReplace
			response.Settings.ScheduledReports.OnUploadNoteEventType = &eventType

			params := redox.SummaryAndReportParameters{
				Match:      *response,
				Order:      *order,
				DocumentId: "1234567",
				PrecedingDocument: &redox.PrecedingDocument{
					Id:          primitive.NewObjectID(),
					CreatedTime: time.Now().Add(-time.Minute*5 - time.Second),
				},
			}
			Expect(params.ShouldReplacePrecedingReport()).To(BeFalse())
		})

		It("Should be true if there's is preceding document created within the last 5 minutes and the clinic is configured for replacement", func() {
			eventType := clinics.ScheduledReportsV1OnUploadNoteEventTypeReplace
			response.Settings.ScheduledReports.OnUploadNoteEventType = &eventType

			offset := rand.IntN(int((time.Minute*5 - time.Second*10).Seconds())) + 1
			params := redox.SummaryAndReportParameters{
				Match:      *response,
				Order:      *order,
				DocumentId: "1234567",
				PrecedingDocument: &redox.PrecedingDocument{
					Id:          primitive.NewObjectID(),
					CreatedTime: time.Now().Add(-time.Second * time.Duration(offset)),
				},
			}
			Expect(params.ShouldReplacePrecedingReport()).To(BeTrue())
		})
	})
})

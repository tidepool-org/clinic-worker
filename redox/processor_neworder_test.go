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
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"net/http"
)

var _ = Describe("NewOrderProcessor", func() {
	var redoxClient *testRedox.RedoxClient
	var clinicCtrl *gomock.Controller
	var clinicClient *clinics.MockClientWithResponsesInterface
	var processor redox.NewOrderProcessor

	BeforeEach(func() {
		redoxClient = testRedox.NewTestRedoxClient("testSourceId", "testSourceName")
		clinicCtrl = gomock.NewController(GinkgoT())
		clinicClient = clinics.NewMockClientWithResponsesInterface(clinicCtrl)
		shorelineClient := shoreline.NewMock("test")
		processor = redox.NewNewOrderProcessor(clinicClient, redoxClient, report.NewSampleReportGenerator(), shorelineClient, zap.NewNop().Sugar())
	})

	Describe("ProcessOrder", func() {
		Context("with subscription order", func() {
			var order models.NewOrder
			var envelope models.MessageEnvelope

			BeforeEach(func() {
				newOrderFixture, err := test.LoadFixture("test/fixtures/subscriptionorder.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(json.Unmarshal(newOrderFixture, &order)).To(Succeed())

				message := bson.Raw{}
				Expect(bson.UnmarshalExtJSON(newOrderFixture, true, &message)).To(Succeed())

				envelope = models.MessageEnvelope{
					Id:      primitive.NewObjectID(),
					Meta:    order.Meta,
					Message: message,
				}
				response := &clinics.EHRMatchResponse{}
				matchFixture, err := test.LoadFixture("test/fixtures/subscriptionmatchresponse.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(json.Unmarshal(matchFixture, response)).To(Succeed())

				matchResponse := &clinics.MatchClinicAndPatientResponse{
					Body: nil,
					HTTPResponse: &http.Response{
						StatusCode: http.StatusOK,
					},
					JSON200: response,
				}

				clinicClient.EXPECT().
					MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(matchResponse, nil)
			})

			It("send results, flowsheet and notes when patient and clinic successfully matched", func() {
				Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
				Expect(redoxClient.Sent).To(HaveLen(3))

				var results models.NewResults
				var notes models.NewNotes
				var flowsheet models.NewFlowsheet

				for _, payload := range redoxClient.Sent {
					switch payload.(type) {
					case models.NewResults:
						results = payload.(models.NewResults)
					case models.NewNotes:
						notes = payload.(models.NewNotes)
					case models.NewFlowsheet:
						flowsheet = payload.(models.NewFlowsheet)
					}
				}

				Expect(results).To(MatchFields(IgnoreExtras, Fields{
					"Meta": MatchFields(IgnoreExtras, Fields{
						"DataModel": Equal("Results"),
						"EventType": Equal("New"),
					}),
				}))

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
				Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
				Expect(redoxClient.Sent).To(HaveLen(3))

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

		Context("with account creation order", func() {
			var order models.NewOrder
			var envelope models.MessageEnvelope

			BeforeEach(func() {
				orderFixture, err := test.LoadFixture("test/fixtures/accountcreationorder.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(json.Unmarshal(orderFixture, &order)).To(Succeed())

				message := bson.Raw{}
				Expect(bson.UnmarshalExtJSON(orderFixture, true, &message)).To(Succeed())

				envelope = models.MessageEnvelope{
					Id:      primitive.NewObjectID(),
					Meta:    order.Meta,
					Message: message,
				}
				response := &clinics.EHRMatchResponse{}
				matchFixture, err := test.LoadFixture("test/fixtures/accountcreationmatchresponse.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(json.Unmarshal(matchFixture, response)).To(Succeed())

				matchResponse := &clinics.MatchClinicAndPatientResponse{
					Body: nil,
					HTTPResponse: &http.Response{
						StatusCode: http.StatusOK,
					},
					JSON200: response,
				}

				clinicClient.EXPECT().
					MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(matchResponse, nil)
			})

			It("creates the patient in the clinic service", func() {
				clinicClient.EXPECT().
					CreatePatientAccountWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&clinics.CreatePatientAccountResponse{
						Body: nil,
						HTTPResponse: &http.Response{
							StatusCode: http.StatusOK,
						},
						JSON200: &clinics.Patient{},
					}, nil)
				Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())

			})
		})
	})
})

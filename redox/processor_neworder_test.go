package redox_test

import (
	"context"
	"encoding/json"
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

var _ = Describe("NewOrderProcessor", func() {
	var redoxClient *testRedox.RedoxClient
	var clinicCtrl *gomock.Controller
	var clinicClient *clinics.MockClientWithResponsesInterface
	var processor redox.NewOrderProcessor

	BeforeEach(func() {
		redoxClient = testRedox.NewTestRedoxClient("testSourceId", "testSourceName")
		clinicCtrl = gomock.NewController(GinkgoT())
		clinicClient = clinics.NewMockClientWithResponsesInterface(clinicCtrl)
		shorelineClient := &testRedox.ShorelineNoUser{Client: shoreline.NewMock("test")}
		processor = redox.NewNewOrderProcessor(clinicClient, redoxClient, report.NewSampleReportGenerator(), shorelineClient, zap.NewNop().Sugar())
	})

	Describe("ProcessOrder", func() {
		BeforeEach(func() {
			response := &clinics.EHRMatchResponse{}
			matchFixture, err := test.LoadFixture("test/fixtures/clinic_match_response.json")
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

		Context("with subscription order", func() {
			var order models.NewOrder
			var envelope models.MessageEnvelope
			var matchResponse *clinics.MatchClinicAndPatientResponse

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

				matchResponse = &clinics.MatchClinicAndPatientResponse{
					Body: nil,
					HTTPResponse: &http.Response{
						StatusCode: http.StatusOK,
					},
					JSON200: response,
				}
			})

			When("clinic settings have ehr tag settings", func() {
				BeforeEach(func() {
					codes := []string{"TIDEPOOL_TAGS"}
					separator := ","
					matchResponse.JSON200.Settings.Tags = clinics.EHRTagsSettings{
						Codes:     &codes,
						Separator: &separator,
					}
					clinicClient.EXPECT().
						MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(matchResponse, nil)
				})

				It("replaces existing patient tags", func() {
					clinicClient.EXPECT().
						GetClinicWithResponse(gomock.Any(), *matchResponse.JSON200.Clinic.Id).
						Return(&clinics.GetClinicResponse{
							Body: nil,
							HTTPResponse: &http.Response{
								StatusCode: http.StatusOK,
							},
							JSON200: &matchResponse.JSON200.Clinic,
						}, nil)

					clinicClient.EXPECT().UpdatePatientWithResponse(gomock.Any(),
						gomock.Eq(*matchResponse.JSON200.Clinic.Id),
						gomock.Eq(*((*matchResponse.JSON200.Patients)[0]).Id),
						testRedox.MatchArg(func(body clinics.UpdatePatientJSONRequestBody) bool {
							if body.Tags == nil {
								return false
							}
							if len(*body.Tags) != 2 {
								return false
							}
							return testRedox.PatientHasTags(body.Tags, []string{"1", "2"})
						}),
					).Return(&clinics.UpdatePatientResponse{
						Body: nil,
						HTTPResponse: &http.Response{
							StatusCode: http.StatusOK,
						},
						JSON200: &(*matchResponse.JSON200.Patients)[0],
					}, nil)

					Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
				})

				It("creates missing tags", func() {
					t1dId := "1"
					t1dName := "T1D"
					adultId := "3"
					adultName := "ADULT"

					matchResponse.JSON200.Clinic.PatientTags = &[]clinics.PatientTag{
						{&t1dId, t1dName},
					}

					clinicClient.EXPECT().
						CreatePatientTagWithResponse(gomock.Any(), *matchResponse.JSON200.Clinic.Id, testRedox.MatchArg(func(body clinics.CreatePatientTagJSONRequestBody) bool {
							return body.Name == "ADULT"
						})).
						Return(&clinics.CreatePatientTagResponse{
							Body: nil,
							HTTPResponse: &http.Response{
								StatusCode: http.StatusOK,
							},
						}, nil)

					updatedClinic := matchResponse.JSON200.Clinic
					updatedClinic.PatientTags = &[]clinics.PatientTag{
						{&t1dId, t1dName},
						{&adultId, adultName},
					}

					clinicClient.EXPECT().
						GetClinicWithResponse(gomock.Any(), *matchResponse.JSON200.Clinic.Id).
						Return(&clinics.GetClinicResponse{
							Body: nil,
							HTTPResponse: &http.Response{
								StatusCode: http.StatusOK,
							},
							JSON200: &updatedClinic,
						}, nil)

					clinicClient.EXPECT().UpdatePatientWithResponse(gomock.Any(),
						gomock.Eq(*matchResponse.JSON200.Clinic.Id),
						gomock.Eq(*((*matchResponse.JSON200.Patients)[0]).Id),
						testRedox.MatchArg(func(body clinics.UpdatePatientJSONRequestBody) bool {
							return testRedox.PatientHasTags(body.Tags, []string{"1", "3"})
						}),
					).Return(&clinics.UpdatePatientResponse{
						Body: nil,
						HTTPResponse: &http.Response{
							StatusCode: http.StatusOK,
						},
						JSON200: &(*matchResponse.JSON200.Patients)[0],
					}, nil)

					Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
				})

				It("ignores tags if creation fails with bad request", func() {
					t1dId := "1"
					t1dName := "T1D"
					adultName := "ADULT"

					matchResponse.JSON200.Clinic.PatientTags = &[]clinics.PatientTag{
						{&t1dId, t1dName},
					}

					clinicClient.EXPECT().
						CreatePatientTagWithResponse(gomock.Any(), *matchResponse.JSON200.Clinic.Id, testRedox.MatchArg(func(body clinics.CreatePatientTagJSONRequestBody) bool {
							return body.Name == adultName
						})).
						Return(&clinics.CreatePatientTagResponse{
							Body: nil,
							HTTPResponse: &http.Response{
								StatusCode: http.StatusBadRequest,
							},
						}, nil)

					updatedClinic := matchResponse.JSON200.Clinic
					updatedClinic.PatientTags = &[]clinics.PatientTag{
						{&t1dId, t1dName},
					}

					clinicClient.EXPECT().
						GetClinicWithResponse(gomock.Any(), *matchResponse.JSON200.Clinic.Id).
						Return(&clinics.GetClinicResponse{
							Body: nil,
							HTTPResponse: &http.Response{
								StatusCode: http.StatusOK,
							},
							JSON200: &updatedClinic,
						}, nil)

					clinicClient.EXPECT().UpdatePatientWithResponse(gomock.Any(),
						gomock.Eq(*matchResponse.JSON200.Clinic.Id),
						gomock.Eq(*((*matchResponse.JSON200.Patients)[0]).Id),
						testRedox.MatchArg(func(body clinics.UpdatePatientJSONRequestBody) bool {
							return testRedox.PatientHasTags(body.Tags, []string{"1"})
						}),
					).Return(&clinics.UpdatePatientResponse{
						Body: nil,
						HTTPResponse: &http.Response{
							StatusCode: http.StatusOK,
						},
						JSON200: &(*matchResponse.JSON200.Patients)[0],
					}, nil)

					Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
				})
			})

			When("clinic settings don't have ehr tag settings", func() {
				BeforeEach(func() {
					clinicClient.EXPECT().
						MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(matchResponse, nil)
				})

				It("send results, flowsheet and notes when patient and clinic successfully matched", func() {
					Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
					Expect(redoxClient.Sent).To(HaveLen(3))

					var results models.NewResults
					var notes redox.Notes
					var flowsheet models.NewFlowsheet

					for _, payload := range redoxClient.Sent {
						switch payload.(type) {
						case models.NewResults:
							results = payload.(models.NewResults)
						case redox.Notes:
							notes = payload.(redox.Notes)
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

				It("uploads a file and references it in the note when upload api is enabled", func() {
					redoxClient.SetUploadFileEnabled(true)
					Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
					Expect(redoxClient.Sent).To(HaveLen(3))

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
			})
		})

		Context("with create account and enable reports order", func() {
			var order models.NewOrder
			var envelope models.MessageEnvelope
			var matchResponse *clinics.MatchClinicAndPatientResponse

			BeforeEach(func() {
				newOrderFixture, err := test.LoadFixture("test/fixtures/createandsubscribeorder.json")
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
				matchFixture, err := test.LoadFixture("test/fixtures/createansubscribematchresponse.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(json.Unmarshal(matchFixture, response)).To(Succeed())

				matchResponse = &clinics.MatchClinicAndPatientResponse{
					Body: nil,
					HTTPResponse: &http.Response{
						StatusCode: http.StatusOK,
					},
					JSON200: response,
				}

			})

			When("patient exists", func() {
				BeforeEach(func() {
					clinicClient.EXPECT().
						MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(matchResponse, nil).Times(2)
				})

				It("send results, flowsheet and notes when patient and clinic successfully matched", func() {
					Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
					Expect(redoxClient.Sent).To(HaveLen(3))

					var results models.NewResults
					var notes redox.Notes
					var flowsheet models.NewFlowsheet

					for _, payload := range redoxClient.Sent {
						switch payload.(type) {
						case models.NewResults:
							results = payload.(models.NewResults)
						case redox.Notes:
							notes = payload.(redox.Notes)
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
			})

			When("patient doesn't exist", func() {
				var noMatchResponse *clinics.MatchClinicAndPatientResponse

				BeforeEach(func() {
					response := &clinics.EHRMatchResponse{}
					matchFixture, err := test.LoadFixture("test/fixtures/createansubscribenomatchresponse.json")
					Expect(err).ToNot(HaveOccurred())
					Expect(json.Unmarshal(matchFixture, response)).To(Succeed())

					noMatchResponse = &clinics.MatchClinicAndPatientResponse{
						Body: nil,
						HTTPResponse: &http.Response{
							StatusCode: http.StatusOK,
						},
						JSON200: response,
					}

					// Check if match exists
					clinicClient.EXPECT().
						MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(noMatchResponse, nil).Times(2)

					// Check create account
					clinicClient.EXPECT().
						MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(matchResponse, nil).Times(1)
				})

				It("creates the account, send results, flowsheet and notes", func() {
					patient := (*matchResponse.JSON200.Patients)[0]
					clinicClient.EXPECT().CreatePatientAccountWithResponse(
						gomock.Any(),
						*matchResponse.JSON200.Clinic.Id,
						testRedox.MatchArg(func(body clinics.CreatePatientAccountJSONRequestBody) bool {
							if body.Mrn == nil || *body.Mrn != "0000000001" {
								return false
							}
							if body.FullName != "Timothy Bixby" {
								return false
							}
							if body.Email == nil || *body.Email != "timothy@bixby.com" {
								return false
							}
							if body.BirthDate.String() != "2008-01-06" {
								return false
							}
							return true
						}),
					).Return(&clinics.CreatePatientAccountResponse{
						HTTPResponse: &http.Response{
							StatusCode: http.StatusOK,
						},
						JSON200: &patient,
					}, nil)

					Expect(processor.ProcessOrder(context.Background(), envelope, order)).To(Succeed())
					Expect(redoxClient.Sent).To(HaveLen(3))

					var results models.NewResults
					var notes redox.Notes
					var flowsheet models.NewFlowsheet

					for _, payload := range redoxClient.Sent {
						switch payload.(type) {
						case models.NewResults:
							results = payload.(models.NewResults)
						case redox.Notes:
							notes = payload.(redox.Notes)
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
			})
		})

		Context("with account creation order", func() {
			var order models.NewOrder
			var envelope models.MessageEnvelope
			var matchResponse *clinics.MatchClinicAndPatientResponse

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

				matchResponse = &clinics.MatchClinicAndPatientResponse{
					Body: nil,
					HTTPResponse: &http.Response{
						StatusCode: http.StatusOK,
					},
					JSON200: response,
				}

				clinicClient.EXPECT().
					MatchClinicAndPatientWithResponse(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(matchResponse, nil)

				clinicClient.EXPECT().
					GetClinicWithResponse(gomock.Any(), *matchResponse.JSON200.Clinic.Id).
					Return(&clinics.GetClinicResponse{
						Body: nil,
						HTTPResponse: &http.Response{
							StatusCode: http.StatusOK,
						},
						JSON200: &matchResponse.JSON200.Clinic,
					}, nil).AnyTimes()
			})

			It("creates the patient in the clinic service", func() {
				patientBody := testRedox.MatchArg(func(body clinics.CreatePatientAccountJSONRequestBody) bool {
					return testRedox.PatientHasTags(body.Tags, []string{"1", "2"}) // The tag ids defined in the match fixture
				})

				clinicClient.EXPECT().
					CreatePatientAccountWithResponse(gomock.Any(), gomock.Any(), patientBody).
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

	Describe("GetEmailAddressFromOrder", func() {
		var order models.NewOrder

		BeforeEach(func() {
			orderFixture, err := test.LoadFixture("test/fixtures/accountcreationorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(orderFixture, &order)).To(Succeed())
		})

		It("returns the patient email address if the patient is over 13", func() {
			email, err := redox.GetEmailAddressFromOrder(order)
			Expect(err).ToNot(HaveOccurred())
			Expect(email).ToNot(BeNil())
			Expect(email).To(PointTo(Equal("tim@test.com")))
		})

		It("returns the guarantor email address if the patient is under 13", func() {
			almostThirteenYearsAgo := time.Now().AddDate(-12, 11, 13).Format("2006-01-02")
			order.Patient.Demographics.DOB = &almostThirteenYearsAgo

			email, err := redox.GetEmailAddressFromOrder(order)
			Expect(err).ToNot(HaveOccurred())
			Expect(email).ToNot(BeNil())
			Expect(email).To(PointTo(Equal("kent@test.com")))
		})
	})

	Describe("GetFullNameFromOrder", func() {
		var order models.NewOrder

		BeforeEach(func() {
			orderFixture, err := test.LoadFixture("test/fixtures/accountcreationorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(orderFixture, &order)).To(Succeed())
		})

		It("returns the concatenated first and last names", func() {
			name, err := redox.GetFullNameFromOrder(order)
			Expect(err).ToNot(HaveOccurred())
			Expect(name).ToNot(BeNil())
			Expect(name).To(Equal("Timothy Bixby"))
		})
	})

	Describe("GetBirthDateFromOrder", func() {
		var order models.NewOrder

		BeforeEach(func() {
			orderFixture, err := test.LoadFixture("test/fixtures/accountcreationorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(orderFixture, &order)).To(Succeed())
		})

		It("returns the date of birth of the patient", func() {
			dob, err := redox.GetBirthDateFromOrder(order)
			Expect(err).ToNot(HaveOccurred())
			Expect(dob.Format("2006-01-02")).To(Equal("2008-01-06"))
		})
	})

	Describe("GetMrnFromOrder", func() {
		var order models.NewOrder

		BeforeEach(func() {
			orderFixture, err := test.LoadFixture("test/fixtures/accountcreationorder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(orderFixture, &order)).To(Succeed())
		})

		It("returns the mrn of the patient", func() {
			mrn, err := redox.GetMrnFromOrder(order)
			Expect(err).ToNot(HaveOccurred())
			Expect(mrn).ToNot(BeNil())
			Expect(mrn).To(PointTo(Equal("0000000001")))
		})
	})
})

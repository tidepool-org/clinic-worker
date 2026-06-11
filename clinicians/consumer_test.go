package clinicians_test

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/clinicians"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/go-common/events"
)

const userID = "1234567890"

// fakeShoreline serves a configurable user from GetUser; everything else comes from the
// go-common mock client.
type fakeShoreline struct {
	*shoreline.ShorelineMockClient
	user *shoreline.UserData
	err  error
}

func (f *fakeShoreline) GetUser(userID, token string) (*shoreline.UserData, error) {
	return f.user, f.err
}

type fakeMarketo struct{}

func (f fakeMarketo) RefreshUserDetails(userId string) error { return nil }

type fakeMailer struct{}

func (f fakeMailer) SendEmailTemplate(context.Context, events.SendEmailTemplateEvent) error {
	return nil
}

func okSecurityProfileResponse() *clinics.UpdateClinicianSecurityProfileResponse {
	return &clinics.UpdateClinicianSecurityProfileResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
	}
}

func message(payload string) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{Value: []byte(strconv.Quote(payload))}
}

var _ = Describe("ClinicianCDCConsumer", func() {
	var consumer events.MessageConsumer
	var ctrl *gomock.Controller
	var clinicsService *clinics.MockClientWithResponsesInterface
	var shorelineClient *fakeShoreline

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clinicsService = clinics.NewMockClientWithResponsesInterface(ctrl)
		shorelineClient = &fakeShoreline{ShorelineMockClient: shoreline.NewMock("token")}

		var err error
		consumer, err = clinicians.NewClinicianCDCConsumer(clinicians.Params{
			Clinics:       clinicsService,
			Logger:        zap.NewNop().Sugar(),
			Mailer:        fakeMailer{},
			MarketoClient: fakeMarketo{},
			Shoreline:     shorelineClient,
		})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("security profile backfill", func() {
		var insertEvent = `{"operationType":"insert","fullDocument":{"clinicId":{"$oid":"5f7b3a000000000000000000"},"userId":"1234567890"}}`
		var associateEvent = `{"operationType":"update","fullDocument":{"clinicId":{"$oid":"5f7b3a000000000000000000"},"userId":"1234567890"},"updateDescription":{"updatedFields":{"userId":"1234567890"}}}`
		var unrelatedUpdate = `{"operationType":"update","fullDocument":{"clinicId":{"$oid":"5f7b3a000000000000000000"},"userId":"1234567890"},"updateDescription":{"updatedFields":{}}}`

		It("backfills the profile on insert with a user id", func() {
			lastLogin := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
			shorelineClient.user = &shoreline.UserData{
				UserID: userID,
				SecurityProfile: &shoreline.UserSecurityProfile{
					MfaEnabled: true,
					IdentityProviders: []shoreline.UserIdentityProvider{
						{Alias: "google", Name: "Google"},
					},
					LastLoginTime: &lastLogin,
				},
			}

			var captured clinics.ClinicianSecurityProfileUpdateV1
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				DoAndReturn(func(_ any, _ clinics.UserId, body clinics.ClinicianSecurityProfileUpdateV1, _ ...any) (*clinics.UpdateClinicianSecurityProfileResponse, error) {
					captured = body
					return okSecurityProfileResponse(), nil
				})

			Expect(consumer.HandleKafkaMessage(message(insertEvent))).To(Succeed())

			Expect(captured.MfaEnabled).ToNot(BeNil())
			Expect(*captured.MfaEnabled).To(BeTrue())
			Expect(captured.IdentityProviders).ToNot(BeNil())
			Expect(*captured.IdentityProviders).To(Equal([]clinics.ClinicianIdentityProviderV1{{Alias: "google", Name: "Google"}}))
			Expect(captured.LastLoginTime).ToNot(BeNil())
			Expect(string(*captured.LastLoginTime)).To(Equal("2026-06-11T10:00:00Z"))
			Expect(captured.MfaEnabledTime).To(BeNil())
		})

		It("backfills the profile when an update sets the user id (invite acceptance)", func() {
			shorelineClient.user = &shoreline.UserData{
				UserID:          userID,
				SecurityProfile: &shoreline.UserSecurityProfile{},
			}

			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				Return(okSecurityProfileResponse(), nil)

			Expect(consumer.HandleKafkaMessage(message(associateEvent))).To(Succeed())
		})

		It("omits lastLoginTime when shoreline does not report one", func() {
			shorelineClient.user = &shoreline.UserData{
				UserID:          userID,
				SecurityProfile: &shoreline.UserSecurityProfile{MfaEnabled: false},
			}

			var captured clinics.ClinicianSecurityProfileUpdateV1
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				DoAndReturn(func(_ any, _ clinics.UserId, body clinics.ClinicianSecurityProfileUpdateV1, _ ...any) (*clinics.UpdateClinicianSecurityProfileResponse, error) {
					captured = body
					return okSecurityProfileResponse(), nil
				})

			Expect(consumer.HandleKafkaMessage(message(insertEvent))).To(Succeed())

			Expect(captured.LastLoginTime).To(BeNil())
			Expect(captured.MfaEnabled).ToNot(BeNil())
			Expect(*captured.MfaEnabled).To(BeFalse())
			// Always set so an empty list wholesale-clears stale links.
			Expect(captured.IdentityProviders).ToNot(BeNil())
			Expect(*captured.IdentityProviders).To(BeEmpty())
		})

		It("does not backfill on updates that do not set the user id", func() {
			Expect(consumer.HandleKafkaMessage(message(unrelatedUpdate))).To(Succeed())
			// No UpdateClinicianSecurityProfileWithResponse expectation: a call would fail the test.
		})

		It("skips backfill when shoreline reports no security profile", func() {
			shorelineClient.user = &shoreline.UserData{UserID: userID}

			Expect(consumer.HandleKafkaMessage(message(insertEvent))).To(Succeed())
		})

		It("skips backfill when the user was deleted", func() {
			shorelineClient.user = nil
			shorelineClient.err = &status.StatusError{Status: status.NewStatus(http.StatusNotFound, "not found")}

			Expect(consumer.HandleKafkaMessage(message(insertEvent))).To(Succeed())
		})

		It("returns an error on other shoreline failures", func() {
			shorelineClient.err = &status.StatusError{Status: status.NewStatus(http.StatusInternalServerError, "boom")}

			Expect(consumer.HandleKafkaMessage(message(insertEvent))).ToNot(Succeed())
		})

		It("tolerates a 404 from the clinic service", func() {
			shorelineClient.user = &shoreline.UserData{
				UserID:          userID,
				SecurityProfile: &shoreline.UserSecurityProfile{},
			}

			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				Return(&clinics.UpdateClinicianSecurityProfileResponse{
					HTTPResponse: &http.Response{StatusCode: http.StatusNotFound},
				}, nil)

			Expect(consumer.HandleKafkaMessage(message(insertEvent))).To(Succeed())
		})

		It("returns an error on an unexpected clinic status code", func() {
			shorelineClient.user = &shoreline.UserData{
				UserID:          userID,
				SecurityProfile: &shoreline.UserSecurityProfile{},
			}

			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				Return(&clinics.UpdateClinicianSecurityProfileResponse{
					HTTPResponse: &http.Response{StatusCode: http.StatusInternalServerError},
				}, nil)

			Expect(consumer.HandleKafkaMessage(message(insertEvent))).ToNot(Succeed())
		})
	})
})

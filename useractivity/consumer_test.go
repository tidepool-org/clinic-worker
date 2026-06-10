package useractivity_test

import (
	"errors"
	"net/http"

	"github.com/IBM/sarama"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/useractivity"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/events"
)

// eventTimeMillis is 2023-11-14T22:13:20Z.
const (
	userID          = "1234567890"
	eventTimeMillis = 1700000000000
	eventTimeRFC    = "2023-11-14T22:13:20Z"
)

func okResponse() *clinics.UpdateClinicianSecurityProfileResponse {
	return &clinics.UpdateClinicianSecurityProfileResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
	}
}

func message(value string) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{Value: []byte(value)}
}

var _ = Describe("CDCConsumer", func() {
	var consumer events.MessageConsumer
	var ctrl *gomock.Controller
	var clinicsService *clinics.MockClientWithResponsesInterface

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clinicsService = clinics.NewMockClientWithResponsesInterface(ctrl)

		var err error
		consumer, err = useractivity.NewCDCConsumer(useractivity.Params{
			Logger:  zap.NewNop().Sugar(),
			Clinics: clinicsService,
		})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("HandleKafkaMessage", func() {
		It("maps a LOGIN event to lastLoginTime", func() {
			var captured clinics.ClinicianSecurityProfileUpdateV1
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				DoAndReturn(func(_ any, _ clinics.UserId, body clinics.ClinicianSecurityProfileUpdateV1, _ ...any) (*clinics.UpdateClinicianSecurityProfileResponse, error) {
					captured = body
					return okResponse(), nil
				})

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"LOGIN","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())

			Expect(captured.LastLoginTime).ToNot(BeNil())
			Expect(string(*captured.LastLoginTime)).To(Equal(eventTimeRFC))
			Expect(captured.MfaEnabled).To(BeNil())
			Expect(captured.IdentityProviders).To(BeNil())
		})

		It("maps an MFA_ENABLED event to mfaEnabled=true and mfaEnabledTime", func() {
			var captured clinics.ClinicianSecurityProfileUpdateV1
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				DoAndReturn(func(_ any, _ clinics.UserId, body clinics.ClinicianSecurityProfileUpdateV1, _ ...any) (*clinics.UpdateClinicianSecurityProfileResponse, error) {
					captured = body
					return okResponse(), nil
				})

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"MFA_ENABLED","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())

			Expect(captured.MfaEnabled).ToNot(BeNil())
			Expect(*captured.MfaEnabled).To(BeTrue())
			Expect(captured.MfaEnabledTime).ToNot(BeNil())
			Expect(string(*captured.MfaEnabledTime)).To(Equal(eventTimeRFC))
		})

		It("maps an MFA_DISABLED event to mfaEnabled=false without a time", func() {
			var captured clinics.ClinicianSecurityProfileUpdateV1
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				DoAndReturn(func(_ any, _ clinics.UserId, body clinics.ClinicianSecurityProfileUpdateV1, _ ...any) (*clinics.UpdateClinicianSecurityProfileResponse, error) {
					captured = body
					return okResponse(), nil
				})

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"MFA_DISABLED","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())

			Expect(captured.MfaEnabled).ToNot(BeNil())
			Expect(*captured.MfaEnabled).To(BeFalse())
			Expect(captured.MfaEnabledTime).To(BeNil())
		})

		It("maps an IDP_LINKS_CHANGED event to the parsed identity providers", func() {
			var captured clinics.ClinicianSecurityProfileUpdateV1
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				DoAndReturn(func(_ any, _ clinics.UserId, body clinics.ClinicianSecurityProfileUpdateV1, _ ...any) (*clinics.UpdateClinicianSecurityProfileResponse, error) {
					captured = body
					return okResponse(), nil
				})

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"IDP_LINKS_CHANGED","identity_providers":"[{\"alias\":\"google\",\"name\":\"Google\"}]","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())

			Expect(captured.IdentityProviders).ToNot(BeNil())
			Expect(*captured.IdentityProviders).To(Equal([]clinics.ClinicianIdentityProviderV1{{Alias: "google", Name: "Google"}}))
		})

		It("clears identity providers when the array is empty", func() {
			var captured clinics.ClinicianSecurityProfileUpdateV1
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				DoAndReturn(func(_ any, _ clinics.UserId, body clinics.ClinicianSecurityProfileUpdateV1, _ ...any) (*clinics.UpdateClinicianSecurityProfileResponse, error) {
					captured = body
					return okResponse(), nil
				})

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"IDP_LINKS_CHANGED","identity_providers":"[]","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())

			Expect(captured.IdentityProviders).ToNot(BeNil())
			Expect(*captured.IdentityProviders).To(BeEmpty())
		})

		It("treats a 404 (non-clinician user) as success", func() {
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				Return(&clinics.UpdateClinicianSecurityProfileResponse{
					HTTPResponse: &http.Response{StatusCode: http.StatusNotFound},
				}, nil)

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"LOGIN","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error on an unexpected status code", func() {
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				Return(&clinics.UpdateClinicianSecurityProfileResponse{
					HTTPResponse: &http.Response{StatusCode: http.StatusInternalServerError},
				}, nil)

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"LOGIN","event_time":1700000000000}}`))
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the client errors", func() {
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				Return(nil, errors.New("boom"))

			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"LOGIN","event_time":1700000000000}}`))
			Expect(err).To(HaveOccurred())
		})

		It("processes snapshot reads (op=r)", func() {
			clinicsService.EXPECT().
				UpdateClinicianSecurityProfileWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userID)), gomock.Any()).
				Return(okResponse(), nil)

			err := consumer.HandleKafkaMessage(message(`{"op":"r","after":{"user_id":"1234567890","event_type":"LOGIN","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())
		})

		It("ignores delete events", func() {
			err := consumer.HandleKafkaMessage(message(`{"op":"d","before":{"user_id":"1234567890","event_type":"LOGIN","event_time":1700000000000},"after":null}`))
			Expect(err).ToNot(HaveOccurred())
		})

		It("ignores unknown event types", func() {
			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"1234567890","event_type":"SOMETHING_ELSE","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())
		})

		It("ignores events without a user id", func() {
			err := consumer.HandleKafkaMessage(message(`{"op":"c","after":{"user_id":"","event_type":"LOGIN","event_time":1700000000000}}`))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

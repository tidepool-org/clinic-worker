package patients_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/IBM/sarama"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic-worker/patients"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
)

const (
	followUpClinicId = "660597ee52570600b1374d70"
	followUpUserId   = "30e9244c-3963-4867-b2fa-30eab4a96f24"
)

// fakeShoreline serves a configurable user from GetUser; everything else comes from the
// go-common mock client.
type fakeShoreline struct {
	*shoreline.ShorelineMockClient
	user *shoreline.UserData
}

func (f *fakeShoreline) GetUser(userID, token string) (*shoreline.UserData, error) {
	return f.user, nil
}

// fakeMailer records the emails it is asked to send.
type fakeMailer struct {
	sent []events.SendEmailTemplateEvent
	err  error
}

func (f *fakeMailer) SendEmailTemplate(_ context.Context,
	event events.SendEmailTemplateEvent) error {

	f.sent = append(f.sent, event)
	return f.err
}

// followUpMessage returns a patient CDC update message. The connection issue source is
// omitted when source is empty.
func followUpMessage(source string, updatedFields map[string]any) *sarama.ConsumerMessage {
	GinkgoHelper()
	fullDocument := map[string]any{
		"clinicId":    map[string]any{"$oid": followUpClinicId},
		"userId":      followUpUserId,
		"fullName":    "Patient Name",
		"permissions": map[string]any{"view": map[string]any{}},
	}
	if source != "" {
		fullDocument["connectionIssueSource"] = source
	}
	event := map[string]any{
		"operationType": "update",
		"fullDocument":  fullDocument,
		"updateDescription": map[string]any{
			"updatedFields": updatedFields,
			"removedFields": []string{},
		},
	}
	payload, err := json.Marshal(event)
	Expect(err).ToNot(HaveOccurred())
	return &sarama.ConsumerMessage{Value: []byte(strconv.Quote(string(payload)))}
}

var followUpUpdatedFields = map[string]any{
	"lastStaleDataFollowUpTime": map[string]any{"$date": 1792487413000},
	"updatedTime":               map[string]any{"$date": 1792487413000},
}

var _ = Describe("Stale data follow-up", func() {
	var consumer events.MessageConsumer
	var ctrl *gomock.Controller
	var clinicsService *clinics.MockClientWithResponsesInterface
	var shorelineClient *fakeShoreline
	var mailer *fakeMailer

	expectClinic := func() {
		clinicsService.EXPECT().
			GetClinicWithResponse(gomock.Any(),
				gomock.Eq(clinics.ClinicId(followUpClinicId))).
			Return(&clinics.GetClinicResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusOK},
				JSON200:      &clinics.ClinicV1{Name: "Clinic Name"},
			}, nil)
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clinicsService = clinics.NewMockClientWithResponsesInterface(ctrl)
		shorelineClient = &fakeShoreline{
			ShorelineMockClient: shoreline.NewMock("token"),
			user:                &shoreline.UserData{Username: "patient@example.com"},
		}
		mailer = &fakeMailer{}

		var err error
		consumer, err = patients.NewPatientCDCConsumer(patients.Params{
			Clinics:   clinicsService,
			Logger:    zap.NewNop().Sugar(),
			Mailer:    mailer,
			Shoreline: shorelineClient,
		})
		Expect(err).ToNot(HaveOccurred())
	})

	DescribeTable("sends the email naming the patient's device",
		func(source, deviceName string) {
			expectClinic()

			msg := followUpMessage(source, followUpUpdatedFields)
			Expect(consumer.HandleKafkaMessage(msg)).To(Succeed())

			Expect(mailer.sent).To(ConsistOf(events.SendEmailTemplateEvent{
				Recipient: "patient@example.com",
				Template:  "patient_stale_data_follow_up",
				Variables: map[string]string{
					"ClinicName": "Clinic Name",
					"DeviceName": deviceName,
				},
			}))
		},
		Entry("for dexcom", "dexcom", "Dexcom"),
		Entry("for abbott", "abbott", "FreeStyle Libre"),
		Entry("for twiist", "twiist", "twiist"),
	)

	It("returns the error when the email can't be sent", func() {
		expectClinic()
		mailer.err = errors.New("test error")

		msg := followUpMessage("dexcom", followUpUpdatedFields)
		Expect(consumer.HandleKafkaMessage(msg)).ToNot(Succeed())
	})

	DescribeTable("doesn't send the email for a non-device source",
		func(source string) {
			msg := followUpMessage(source, followUpUpdatedFields)
			Expect(consumer.HandleKafkaMessage(msg)).To(Succeed())
			Expect(mailer.sent).To(BeEmpty())
		},
		Entry("when it's an invitation", "deviceNonSpecificInvitation"),
		Entry("when it's missing", ""),
	)

	It("doesn't send the email when the patient has no email", func() {
		shorelineClient.user = &shoreline.UserData{}

		msg := followUpMessage("dexcom", followUpUpdatedFields)
		Expect(consumer.HandleKafkaMessage(msg)).To(Succeed())
		Expect(mailer.sent).To(BeEmpty())
	})

	It("doesn't send the email for unrelated updates", func() {
		msg := followUpMessage("dexcom", map[string]any{
			"updatedTime": map[string]any{"$date": 1792487413000},
		})
		Expect(consumer.HandleKafkaMessage(msg)).To(Succeed())
		Expect(mailer.sent).To(BeEmpty())
	})
})

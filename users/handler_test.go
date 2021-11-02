package users_test

import (
	"errors"
	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/users"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/zap"
	"net/http"
)

var _ = Describe("UserDataDeletionHandler", func() {
	Describe("HandleDeleteUserEvent", func() {
		var handler events.EventHandler
		var ctrl *gomock.Controller
		var clinicsService *clinics.MockClientWithResponsesInterface

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			clinicsService = clinics.NewMockClientWithResponsesInterface(ctrl)

			var err error
			handler, err = users.NewUserDataDeletionHandler(clinicsService, zap.NewNop().Sugar())
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("deletes the correct user with the provided clinics client", func() {
			userId := "1234567890"
			response := &clinics.DeleteUserFromClinicsResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			}
			clinicsService.EXPECT().
				DeleteUserFromClinicsWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userId))).
				Return(response, nil)

			event, err := NewDeleteUserEvent(userId)
			Expect(err).ToNot(HaveOccurred())
			err = handler.Handle(event)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error if the status code is unexpected", func() {
			userId := "1234567890"
			response := &clinics.DeleteUserFromClinicsResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusTeapot},
			}
			clinicsService.EXPECT().
				DeleteUserFromClinicsWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userId))).
				Return(response, nil)

			event, err := NewDeleteUserEvent(userId)
			Expect(err).ToNot(HaveOccurred())
			err = handler.Handle(event)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the client returns an error", func() {
			userId := "1234567890"
			clinicsService.EXPECT().
				DeleteUserFromClinicsWithResponse(gomock.Any(), gomock.Eq(clinics.UserId(userId))).
				Return(nil, errors.New("error"))

			event, err := NewDeleteUserEvent(userId)
			Expect(err).ToNot(HaveOccurred())
			err = handler.Handle(event)
			Expect(err).To(HaveOccurred())
		})
	})
})

func NewDeleteUserEvent(userId string) (ce.Event, error) {
	deleteUserEvent := events.DeleteUserEvent{
		UserData: shoreline.UserData{UserID: userId},
	}
	e := ce.NewEvent()
	e.SetType(deleteUserEvent.GetEventType())
	e.SetSource("clinic-worker-test")

	err := e.SetData(ce.ApplicationJSON, deleteUserEvent)
	return e, err
}

package users

import (
	"context"
	"fmt"
	clinics "github.com/tidepool-org/clinic/client"
	ev "github.com/tidepool-org/go-common/events"

	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
)

type userDeletionEventsHandler struct {
	ev.NoopUserEventsHandler

	clinics clinics.ClientWithResponsesInterface
	logger  *zap.SugaredLogger
}

func NewUserDataDeletionHandler(clinicService clinics.ClientWithResponsesInterface, logger *zap.SugaredLogger) (ev.EventHandler, error) {
	return ev.NewUserEventsHandler(&userDeletionEventsHandler{
		clinics: clinicService,
		logger:  logger,
	}), nil
}

func (u *userDeletionEventsHandler) HandleDeleteUserEvent(payload ev.DeleteUserEvent) error {
	userId := payload.UserID
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	u.logger.Infow("deleting user from clinics", "userId", userId)
	response, err := u.clinics.DeleteUserFromClinicsWithResponse(ctx, clinics.UserId(userId))
	if err != nil {
		return err
	}
	if response.StatusCode() != http.StatusOK {
		err := fmt.Errorf("unexpected response code %v", response.StatusCode())
		u.logger.Errorw("could not delete user from clinics", "userId", userId, zap.Error(err))
		return err
	}
	u.logger.Infow("successfully deleted user from all clinics", "userId", userId)
	return nil
}

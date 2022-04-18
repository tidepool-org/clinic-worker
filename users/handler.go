package users

import (
	"context"
	"fmt"
	"github.com/deepmap/oapi-codegen/pkg/types"
	clinics "github.com/tidepool-org/clinic/client"
	ev "github.com/tidepool-org/go-common/events"

	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
)

type userEventsHandler struct {
	ev.NoopUserEventsHandler

	clinics clinics.ClientWithResponsesInterface
	logger  *zap.SugaredLogger
}

func NewUserDataDeletionHandler(clinicService clinics.ClientWithResponsesInterface, logger *zap.SugaredLogger) (ev.EventHandler, error) {
	return ev.NewUserEventsHandler(&userEventsHandler{
		clinics: clinicService,
		logger:  logger,
	}), nil
}

func (u *userEventsHandler) HandleUpdateUserEvent(payload ev.UpdateUserEvent) error {
	userId := payload.Original.UserID
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if payload.Original.Username != payload.Updated.Username {
		u.logger.Infow("updating user email", "userId", userId)
		email := types.Email(payload.Updated.Username)
		update := clinics.UpdateClinicUserDetailsJSONRequestBody{
			Email: &email,
		}
		resp, err := u.clinics.UpdateClinicUserDetailsWithResponse(ctx, clinics.UserId(userId), update)
		if err != nil {
			u.logger.Errorw("could not update clinician user details", "userId", userId, zap.Error(err))
			return err
		}
		if resp.StatusCode() != http.StatusOK {
			err := fmt.Errorf("unexpected response code %v", resp.StatusCode())
			u.logger.Errorw("could not update clinic user details", "userId", userId, zap.Error(err))
			return err
		}
	}

	u.logger.Infow("successfully updated clinic user details", "userId", userId)
	return nil
}

func (u *userEventsHandler) HandleDeleteUserEvent(payload ev.DeleteUserEvent) error {
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

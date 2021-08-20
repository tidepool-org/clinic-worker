package confirmation

import (
	"fmt"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/errors"
	"go.uber.org/fx"
	"net/http"
)

var Module = fx.Provide(
	ConfigProvider,
	NewService,
)

type Service interface {
	UpsertSignUpInvite(userId string) error
}

func NewService(config Config, shorelineClient shoreline.Client, httpClient *http.Client) (Service, error) {
	return &service{
		host:            config.HydrophoneHost,
		httpClient:      httpClient,
		shorelineClient: shorelineClient,
	}, nil
}

type service struct {
	host            string
	httpClient      *http.Client
	shorelineClient shoreline.Client
}

func (s *service) UpsertSignUpInvite(userId string) error {
	url := fmt.Sprintf("%s/confirm/send/signup/%s", s.host, userId)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Add("x-tidepool-session-token", s.shorelineClient.TokenProvide())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to upsert confirmation: %w", err)
	}

	// Hydrophone returns 403 when there's an existing invite
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
		return errors.Newf("unexpected status code %v when upserting confirmation", resp.StatusCode)
	}
	return nil
}

package confirmation

import (
	"bytes"
	"encoding/json"
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

type SignUpInvite struct {
	UserId    string  `json:"-"`
	ClinicId  string  `json:"clinicId"`
	InvitedBy *string `json:"invitedBy"`
}

type Service interface {
	UpsertSignUpInvite(invite SignUpInvite) error
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

func (s *service) UpsertSignUpInvite(invite SignUpInvite) error {
	url := fmt.Sprintf("%s/confirm/send/signup/%s", s.host, invite.UserId)
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(invite); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Add("x-tidepool-session-token", s.shorelineClient.TokenProvide())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to upsert confirmation: %w", err)
	}

	// Hydrophone returns 403 when there's an existing invite so that's an expected response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
		return errors.Newf("unexpected status code %v when upserting confirmation", resp.StatusCode)
	}
	return nil
}

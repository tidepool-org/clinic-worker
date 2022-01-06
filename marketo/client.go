package marketo

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"time"
)

const tidepoolSessionTokenKey = "x-tidepool-session-token"

var Module = fx.Provide(NewConfig, NewClient)

type Client interface {
	RefreshUserDetails(userId string) error
}

type Config struct {
	MarketoServiceHost    string        `envconfig:"TIDEPOOL_MARKETO_SERVICE_HOST" default:"http://marketo-service:8080"`
	MarketoServiceEnabled bool          `envconfig:"TIDEPOOL_MARKETO_SERVICE_ENABLED" default:"true"`
	Timeout               time.Duration `envconfig:"TIDEPOOL_MARKETO_SERVICE_TIMEOUT" default:"30s"`
}

func NewConfig() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}

func NewClient(config Config, logger *zap.SugaredLogger, shorelineClient shoreline.Client) (Client, error) {
	if !config.MarketoServiceEnabled {
		logger.Info("marketo service integration is disabled")
		return &disabled{
			logger: logger,
		}, nil
	}

	httpClient := http.Client{
		Timeout: config.Timeout,
	}
	return &client{
		httpClient:         httpClient,
		logger:             logger,
		marketoServiceHost: config.MarketoServiceHost,
		shorelineClient:    shorelineClient,
	}, nil
}

type client struct {
	httpClient         http.Client
	logger             *zap.SugaredLogger
	marketoServiceHost string
	shorelineClient    shoreline.Client
}

func (c *client) RefreshUserDetails(userId string) error {
	url := fmt.Sprintf("%s/v1/users/%s/marketo", c.marketoServiceHost, userId)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set(tidepoolSessionTokenKey, c.shorelineClient.TokenProvide())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, rErr := ioutil.ReadAll(resp.Body)
		if rErr != nil {
			c.logger.Errorf("unable to decode request body %v", rErr)
		}
		err := fmt.Errorf("unepxected response from marketo service: %v %v", resp.StatusCode, string(body))
		c.logger.Errorw(err.Error(), "userId", userId)

		return err
	}

	return nil
}

type disabled struct {
	logger *zap.SugaredLogger
}

func (d *disabled) RefreshUserDetails(userId string) error {
	d.logger.Debugw("skipping marketo user details refresh, because client is disabled", "userId", userId)
	return nil
}

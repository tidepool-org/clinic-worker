package worker

import (
	"context"
	"crypto/tls"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/disc"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"net/http"
	"time"
)

type DependenciesConfig struct {
	ShorelineHost  string `envconfig:"TIDEPOOL_SHORELINE_CLIENT_ADDRESS" default:"http://shoreline:9107"`
	SeagullHost    string `envconfig:"TIDEPOOL_SEAGULL_CLIENT_ADDRESS" default:"http://seagull:9120"`
	ServerSecret   string `envconfig:"TIDEPOOL_SERVER_SECRET"`
}

func configProvider() (DependenciesConfig, error) {
	cfg := DependenciesConfig{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}

func eventsConfigProvider() (*events.CloudEventsConfig, error) {
	config := events.NewConfig()
	err := config.LoadFromEnv()
	return config, err
}

func httpClientProvider() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{Transport: tr}
}

func shorelineProvider(config DependenciesConfig, httpClient *http.Client, lifecycle fx.Lifecycle) shoreline.Client {
	client := shoreline.NewShorelineClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.ShorelineHost)).
		WithHttpClient(httpClient).
		WithName("clinics").
		WithSecret(config.ServerSecret).
		WithTokenRefreshInterval(time.Hour).
		Build()

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return client.Start()
		},
		OnStop: func(ctx context.Context) error {
			client.Close()
			return nil
		},
	})

	return client
}

func seagullProvider(config DependenciesConfig, httpClient *http.Client) clients.Seagull {
	return clients.NewSeagullClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.SeagullHost)).
		WithHttpClient(httpClient).
		Build()
}

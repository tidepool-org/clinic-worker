package worker

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/disc"
	"github.com/tidepool-org/go-common/clients/shoreline"
	summaries "github.com/tidepool-org/go-common/clients/summary"
	"github.com/tidepool-org/go-common/events"
	confirmations "github.com/tidepool-org/hydrophone/client"
	"go.uber.org/fx"
)

type DependenciesConfig struct {
	ShorelineHost  string `envconfig:"TIDEPOOL_SHORELINE_CLIENT_ADDRESS" default:"http://shoreline:9107"`
	SeagullHost    string `envconfig:"TIDEPOOL_SEAGULL_CLIENT_ADDRESS" default:"http://seagull:9120"`
	GatekeeperHost string `envconfig:"TIDEPOOL_GATEKEEPER_CLIENT_ADDRESS" default:"http://gatekeeper:9123"`
	ClinicsHost    string `envconfig:"TIDEPOOL_CLINIC_CLIENT_ADDRESS" default:"http://clinic:8080"`
	DataHost       string `envconfig:"TIDEPOOL_DATA_CLIENT_ADDRESS" default:"http://data:9220"`
	HydrophoneHost string `envconfig:"TIDEPOOL_CONFIRMATION_CLIENT_ADDRESS" default:"http://hydrophone:9157"`
	AuthHost       string `envconfig:"TIDEPOOL_DOCKER_PLATFORM_AUTH_HOST" default:"http://auth:9222"`
	ServerSecret   string `envconfig:"TIDEPOOL_SERVER_SECRET"`
}

func configProvider() (DependenciesConfig, error) {
	cfg := DependenciesConfig{}
	err := envconfig.Process("", &cfg)
	return cfg, err
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
		WithName("clinic-worker").
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

func gatekeeperProvider(config DependenciesConfig, httpClient *http.Client, shoreline shoreline.Client) clients.Gatekeeper {
	return clients.NewGatekeeperClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.GatekeeperHost)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()
}

func summariesProvider(config DependenciesConfig, shoreline shoreline.Client) (summaries.ClientWithResponsesInterface, error) {
	opts := summaries.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("x-tidepool-session-token", shoreline.TokenProvide())
		return nil
	})
	return summaries.NewClientWithResponses(config.DataHost, opts)
}

func confirmationsProvider(config DependenciesConfig, shoreline shoreline.Client) (confirmations.ClientWithResponsesInterface, error) {
	opts := confirmations.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("x-tidepool-session-token", shoreline.TokenProvide())
		return nil
	})
	return confirmations.NewClientWithResponses(config.HydrophoneHost, opts)
}

func datasourcesProvider(config DependenciesConfig, httpClient *http.Client, shoreline shoreline.Client) clients.DataClient {
	return *clients.NewDataClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.DataHost)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()
}

func authProvider(config DependenciesConfig, httpClient *http.Client, shoreline shoreline.Client) clients.AuthClient {
	return *clients.NewAuthClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.AuthHost)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()
}

func clinicProvider(config DependenciesConfig, shoreline shoreline.Client) (clinics.ClientWithResponsesInterface, error) {
	opts := clinics.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("x-tidepool-session-token", shoreline.TokenProvide())
		return nil
	})
	return clinics.NewClientWithResponses(config.ClinicsHost, opts)
}

func mailerProvider() (clients.MailerClient, error) {
	config := events.NewConfig()
	if err := config.LoadFromEnv(); err != nil {
		return nil, err
	}

	// Hack - Replaces '.' suffix with '-', because mongo CDC uses '.' as separator,
	// and the topics managed by us (like the mailer topic) use '-'
	if strings.HasSuffix(config.KafkaTopicPrefix, ".") {
		config.KafkaTopicPrefix = strings.TrimSuffix(config.KafkaTopicPrefix, ".") + "-"
	}
	config.KafkaTopic = clients.MailerTopic
	config.EventSource = config.KafkaConsumerGroup
	// We are using a sync producer which requires setting the variables below
	config.SaramaConfig.Producer.Return.Errors = true
	config.SaramaConfig.Producer.Return.Successes = true

	return clients.NewMailerClient(config)
}

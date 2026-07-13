package redox

import (
	"time"

	"github.com/avast/retry-go"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/report"
	"go.uber.org/fx"
)

var Module = fx.Provide(
	NewConfig,
	NewClient,
	NewNewOrderProcessor,
	NewScheduledSummaryAndReportProcessor,
	report.NewReportGenerator,
	fx.Annotated{
		Group:  "consumers",
		Target: CreateRedoxMessageConsumerGroup,
	},
	fx.Annotated{
		Group:  "consumers",
		Target: CreateScheduledSummaryAndReportsConsumerGroup,
	},
)

const (
	defaultTimeout = 180 * time.Second
)

var retryOptions = cdc.RetryOptions{
	Attempts:  3,
	Delay:     15 * time.Second,
	DelayType: retry.FixedDelay,
}

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_REDOX_ENABLED" default:"false"`
}

func NewConfig() (ModuleConfig, error) {
	config := ModuleConfig{}
	err := envconfig.Process("", &config)
	return config, err
}

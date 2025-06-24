package redox

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"

	"github.com/tidepool-org/clinic-worker/report"
)

var Module = fx.Provide(
	NewConfig,
	NewClient,
	NewNewOrderProcessor,
	NewScheduledSummaryAndReportProcessor,
	report.NewReportGenerator,
	fx.Annotated{
		Group:  "runners",
		Target: NewMessageCDCConsumer,
	},
	fx.Annotated{
		Group:  "runners",
		Target: NewScheduledSummaryAndReportsRunner,
	},
)

const (
	defaultTimeout = 60 * time.Second
)

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_REDOX_ENABLED" default:"false"`
}

func NewConfig() (ModuleConfig, error) {
	config := ModuleConfig{}
	err := envconfig.Process("", &config)
	return config, err
}

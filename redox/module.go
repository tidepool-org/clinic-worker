package redox

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
	"time"
)

const (
	redoxTopic                      = "clinic.redox"
	scheduledSummaryAndReportsTopic = "clinic.scheduledSummaryAndReportsOrders"
	defaultTimeout                  = 30 * time.Second
)

var Module = fx.Provide(
	NewModuleConfig,
	NewClient,
	NewNewOrderProcessor,
	NewScheduledSummaryAndReportProcessor,
	NewReportGenerator,
	fx.Annotated{
		Group:  "consumers",
		Target: CreateRedoxMessageConsumerGroup,
	},
	fx.Annotated{
		Group:  "consumers",
		Target: CreateScheduledSummaryAndReportsConsumerGroup,
	},
)

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_REDOX_ENABLED" default:"false"`
}

func NewModuleConfig() (ModuleConfig, error) {
	config := ModuleConfig{}
	err := envconfig.Process("", &config)
	return config, err
}

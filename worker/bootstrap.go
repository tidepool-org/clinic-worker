package worker

import (
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/clinicians"
	"github.com/tidepool-org/clinic-worker/clinics"
	"github.com/tidepool-org/clinic-worker/confirmation"
	"github.com/tidepool-org/clinic-worker/migration"
	"github.com/tidepool-org/clinic-worker/patients"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
)

var dependencies = fx.Provide(
	loggerProvider,
	healthCheckServerProvider,
	configProvider,
	httpClientProvider,
	seagullProvider,
	shorelineProvider,
	gatekeeperProvider,
	clinicProvider,
	mailerProvider,
)

func New() *fx.App {
	return fx.New(
		dependencies,
		confirmation.Module,
		patients.Module,
		clinics.Module,
		clinicians.Module,
		migration.Module,
		fx.Invoke(
			startConsumers,
			startHealthCheckServer,
		),
	)
}

type Components struct {
	fx.In

	Consumers  []events.EventConsumer `group:"consumers"`
	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
}

func startConsumers(components Components) {
	for _, consumer := range components.Consumers {
		cdc.AttachConsumerGroupHooks(consumer, components.Lifecycle, components.Shutdowner)
	}
}

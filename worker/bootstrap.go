package worker

import (
	"github.com/tidepool-org/clinic-worker/merge"
	"github.com/tidepool-org/clinic-worker/redox"
	"github.com/tidepool-org/go-common/asyncevents"
	"net/http"

	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/clinicians"
	"github.com/tidepool-org/clinic-worker/clinics"
	"github.com/tidepool-org/clinic-worker/datasources"
	"github.com/tidepool-org/clinic-worker/marketo"
	"github.com/tidepool-org/clinic-worker/migration"
	"github.com/tidepool-org/clinic-worker/patients"
	"github.com/tidepool-org/clinic-worker/patientsummary"
	"github.com/tidepool-org/clinic-worker/users"
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
	summariesProvider,
	confirmationsProvider,
	datasourcesProvider,
	authProvider,
	clinicProvider,
	mailerProvider,
)

var Modules = []fx.Option{
	dependencies,
	datasources.Module,
	patients.Module,
	patientsummary.Module,
	clinics.Module,
	clinicians.Module,
	merge.Module,
	migration.Module,
	redox.Module,
	users.Module,
	marketo.Module,
}

func New() *fx.App {
	invokes := fx.Invoke(
		startConsumers,
		startHealthCheckServer,
	)
	return fx.New(append(Modules, invokes)...)
}

type Components struct {
	fx.In

	Consumers         []events.EventConsumer           `group:"consumers"`
	Runners           []asyncevents.SaramaEventsRunner `group:"runners"`
	HealthCheckServer *http.Server
	Lifecycle         fx.Lifecycle
	Shutdowner        fx.Shutdowner
}

func startConsumers(components Components) {
	for _, consumer := range components.Consumers {
		cdc.AttachConsumerGroupHooks(consumer, components.Lifecycle, components.Shutdowner)
	}
	for _, runner := range components.Runners {
		cdc.AttachSaramaRunnerHooks(runner, components.Lifecycle, components.Shutdowner)
	}
}

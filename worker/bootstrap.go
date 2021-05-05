package worker

import (
	"context"
	"github.com/tidepool-org/clinic-worker/confirmation"
	"github.com/tidepool-org/clinic-worker/patients"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"log"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			newLogger,
			getSuggaredLogger,
			healthCheckServer,
			configProvider,
			httpClientProvider,
			seagullProvider,
			shorelineProvider,
			confirmation.ConfigProvider,
			confirmation.NewService,
			patients.CreateConsumer,
			eventsConfigProvider,
			events.NewFaultTolerantConsumerGroup,
		),
		fx.Invoke(
			startConsumer,
			startHealthCheckServer,
		),
	)
}

func startConsumer(cg *events.FaultTolerantConsumerGroup, lifecycle fx.Lifecycle, shutdowner fx.Shutdowner) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := cg.Start(); err != nil {
					log.Printf("error from consumer: %v", err)
					if err := shutdowner.Shutdown(); err != nil {
						log.Printf("error shutting down: %v", err)
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return cg.Stop()
		},
	})
}
package consumer

import (
	"context"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
)


func start(cg *events.FaultTolerantConsumerGroup, lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return cg.Start()
		},
		OnStop: func(ctx context.Context) error {
			return cg.Stop()
		},
	})
}

func New() *fx.App {
	return fx.New(fx.Provide(
		configProvider,
		httpClientProvider,
		seagullProvider,
		shorelineProvider,
		CreateConsumer,
		eventsConfigProvider,
		events.NewFaultTolerantConsumerGroup,
		start,
	))
}

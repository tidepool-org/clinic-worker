package cdc

import (
	"context"
	"errors"
	"log"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/tidepool-org/go-common/asyncevents"
	"github.com/tidepool-org/go-common/events"
)

func AttachConsumerGroupHooks(cg events.EventConsumer, lifecycle fx.Lifecycle, shutdowner fx.Shutdowner) {
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

type SaramaFxAdapterConfig struct {
	Consumers  []asyncevents.Runner `group:"runners"`
	Logger     *zap.SugaredLogger
	Shutdowner fx.Shutdowner
}

// SaramaFxAdapter adds fx.Lifecycle and Shutdown support to go-common's
// asyncevents.NonBlockingStartStopAdapter.
type SaramaFxAdapter struct {
	*SaramaFxAdapterConfig

	adapters []*asyncevents.NonBlockingStartStopAdapter
}

type SaramaFxAdapterParams struct {
	fx.In

	SaramaFxAdapterConfig
}

func NewSaramaFxAdapter(p SaramaFxAdapterParams) *SaramaFxAdapter {
	return &SaramaFxAdapter{
		SaramaFxAdapterConfig: &p.SaramaFxAdapterConfig,
	}
}

// Start implements fx.HookFunc.
func (f *SaramaFxAdapter) Start(ctx context.Context) error {
	adapters := []*asyncevents.NonBlockingStartStopAdapter{}
	for _, consumer := range f.Consumers {
		adapter := asyncevents.NewNonBlockingStartStopAdapter(consumer, f.onError)
		if err := adapter.Start(ctx); err != nil {
			return err
		}
		adapters = append(adapters, adapter)
	}
	f.adapters = adapters
	return nil
}

// onError is passed to asyncevents.NonBlockingStartStopAdapter to be called on an error.
//
// Using an fx.Shutdowner, we can then shutdown the service.
func (f *SaramaFxAdapter) onError(err error) {
	f.Logger.With("error", err).Info("consumer exited unexpectedly")
	if err := f.Shutdowner.Shutdown(fx.ExitCode(1)); err != nil {
		f.Logger.With("error", err).Info("shutting down fx")
	}
}

// Start implements fx.HookFunc.
func (f *SaramaFxAdapter) Stop(ctx context.Context) error {
	var joinedErrs error
	for _, adapter := range f.adapters {
		if err := adapter.Stop(ctx); err != nil {
			joinedErrs = errors.Join(joinedErrs, err)
		}
	}
	return joinedErrs
}

var Module = fx.Provide(
	fx.Annotate(
		NewSaramaFxAdapter,
		fx.OnStart(func(ctx context.Context, sfa *SaramaFxAdapter) error {
			return sfa.Start(ctx)
		}),
		fx.OnStop(func(ctx context.Context, sfa *SaramaFxAdapter) error {
			return sfa.Stop(ctx)
		}),
	),
)

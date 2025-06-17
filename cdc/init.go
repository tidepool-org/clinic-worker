package cdc

import (
	"context"
	"github.com/tidepool-org/go-common/asyncevents"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"log"
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

func AttachSaramaRunnerHooks(runner asyncevents.SaramaEventsRunner, lifecycle fx.Lifecycle, shutdowner fx.Shutdowner) {
	adapted := asyncevents.NewSaramaRunner(runner)
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := adapted.Initialize(); err != nil {
					log.Printf("unable to initialize runner: %v", err)
				}
				if err := adapted.Run(); err != nil {
					log.Printf("error from runner: %v", err)
					if err := shutdowner.Shutdown(); err != nil {
						log.Printf("error shutting down runner: %v", err)
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return adapted.Terminate()
		},
	})
}

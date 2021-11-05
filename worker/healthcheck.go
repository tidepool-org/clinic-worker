package worker

import (
	"context"
	"go.uber.org/fx"
	"log"
	"net/http"
)

func healthCheckServerProvider() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
}

func startHealthCheckServer(components Components) {
	components.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := components.HealthCheckServer.ListenAndServe(); err != nil {
					log.Printf("http listen and serve error: %v", err)
					_ = components.Shutdowner.Shutdown()
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return components.HealthCheckServer.Shutdown(ctx)
		},
	})
}

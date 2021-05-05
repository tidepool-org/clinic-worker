package consumer

import (
	"context"
	"go.uber.org/fx"
	"log"
	"net/http"
)

func healthCheckServer() *http.Server {
	server := &http.Server{Addr: ":8080"}
	http.HandleFunc("/status", func (w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return server
}

func startHealthCheckServer(server *http.Server, lifecycle fx.Lifecycle, shutdowner fx.Shutdowner) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil {
					log.Printf("http listen and serve error: %v", err)
					_ = shutdowner.Shutdown()
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}



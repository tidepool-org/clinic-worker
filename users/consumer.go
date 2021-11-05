package users

import (
	clinics "github.com/tidepool-org/clinic/client"
	ev "github.com/tidepool-org/go-common/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	UserEventsTopic = "user-events"
)

var Module = fx.Provide(fx.Annotated{
	Group:  "consumers",
	Target: NewEventConsumer,
})

func NewEventConsumer(clinicService clinics.ClientWithResponsesInterface, logger *zap.SugaredLogger) (ev.EventConsumer, error) {
	config := ev.NewConfig()
	if err := config.LoadFromEnv(); err != nil {
		return nil, err
	}
	config.KafkaTopic = UserEventsTopic

	return ev.NewFaultTolerantConsumerGroup(config, func() (ev.MessageConsumer, error) {
		handler, err := NewUserDataDeletionHandler(clinicService, logger)
		if err != nil {
			return nil, err
		}
		return ev.NewCloudEventsMessageHandler([]ev.EventHandler{
			handler,
		})
	})
}

package cdc

import "github.com/tidepool-org/go-common/events"

func GetConfig() (*events.CloudEventsConfig, error) {
	config := events.NewConfig()
	err := config.LoadFromEnv()
	return config, err
}


package confirmation

import "github.com/kelseyhightower/envconfig"

type Config struct {
	HydrophoneHost string `envconfig:"TIDEPOOL_CONFIRMATION_CLIENT_ADDRESS" default:"http://hydrophone:9157"`
}

func ConfigProvider() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}

package patients

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type EmailRemindersConfig struct {
	// Interval represents how much time from sending an email to send a followup
	// email for connect account and claim account reminders.
	Interval time.Duration `envconfig:"EMAIL_REMINDERS_INTERVAL" default:"168h"`
}

func NewEmailRemindersConfig() (*EmailRemindersConfig, error) {
	cfg := EmailRemindersConfig{}
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

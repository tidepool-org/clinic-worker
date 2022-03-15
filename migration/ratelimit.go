package migration

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/ratelimit"
)

type RateLimiterConfig struct {
	PatientMigrationsPerSecond uint `envconfig:"PATIENT_MIGRATIONS_PER_SECOND_LIMIT" default:"15"`
}

type RateLimiter struct {
	rl ratelimit.Limiter
}

func NewRateLimiter() (*RateLimiter, error) {
	cfg := RateLimiterConfig{}
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	return &RateLimiter{
		rl: ratelimit.New(int(cfg.PatientMigrationsPerSecond)),
	}, nil
}

// WaitOrContinue blocks if the rate limit is exceeded
func (r *RateLimiter) WaitOrContinue() {
	r.rl.Take()
}

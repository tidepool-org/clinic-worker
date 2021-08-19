package cdc

import (
	"github.com/Shopify/sarama"
	"github.com/avast/retry-go"
	"github.com/tidepool-org/go-common/events"
	"time"
)

var (
	DefaultAttempts  = uint(5000)
	DefaultDelay     = 1 * time.Minute
	DefaultDelayType = retry.FixedDelay
)

type RetryingConsumer struct {
	attempts  uint
	delay     time.Duration
	delayType retry.DelayTypeFunc
	delegate  events.MessageConsumer
}

func NewRetryingConsumer(delegate events.MessageConsumer) events.MessageConsumer {
	return &RetryingConsumer{
		attempts:  DefaultAttempts,
		delay:     DefaultDelay,
		delayType: DefaultDelayType,
		delegate:  delegate,
	}
}

func (r *RetryingConsumer) Initialize(config *events.CloudEventsConfig) error {
	return r.delegate.Initialize(config)
}

func (r *RetryingConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	retryFn := func() error { return r.delegate.HandleKafkaMessage(cm) }
	return retry.Do(
		retryFn,
		retry.Attempts(r.attempts),
		retry.Delay(r.delay),
		retry.DelayType(r.delayType),
	)
}

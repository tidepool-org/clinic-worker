package cdc

import (
	"time"

	"github.com/IBM/sarama"
	"github.com/avast/retry-go"
	"github.com/tidepool-org/go-common/events"
)

var (
	DefaultAttempts  = uint(5000)
	DefaultDelay     = 1 * time.Minute
	DefaultDelayType = retry.FixedDelay
)

type RetryOptions struct {
	Attempts  uint
	Delay     time.Duration
	DelayType retry.DelayTypeFunc
}

type RetryingConsumer struct {
	opts     RetryOptions
	delegate events.MessageConsumer
}

func NewRetryingConsumer(delegate events.MessageConsumer) events.MessageConsumer {
	return NewRetryingConsumerWithOpts(delegate, RetryOptions{
		Attempts:  DefaultAttempts,
		Delay:     DefaultDelay,
		DelayType: DefaultDelayType,
	})
}

func NewRetryingConsumerWithOpts(delegate events.MessageConsumer, opts RetryOptions) events.MessageConsumer {
	return &RetryingConsumer{
		opts:     opts,
		delegate: delegate,
	}
}

func (r *RetryingConsumer) Initialize(config *events.CloudEventsConfig) error {
	return r.delegate.Initialize(config)
}

func (r *RetryingConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	retryFn := func() error { return r.delegate.HandleKafkaMessage(cm) }
	return retry.Do(
		retryFn,
		retry.Attempts(r.opts.Attempts),
		retry.Delay(r.opts.Delay),
		retry.DelayType(r.opts.DelayType),
	)
}

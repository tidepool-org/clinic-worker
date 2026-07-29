package cdc

import (
	"time"

	"github.com/IBM/sarama"
	"github.com/avast/retry-go"
	"github.com/tidepool-org/go-common/events"
	"go.uber.org/zap"
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
	// CommitOnFailure controls what happens once all retry attempts are exhausted.
	// When true, the final error is logged and swallowed (nil is returned) so the
	// Kafka offset is committed and the message is not redelivered. When false, the
	// error is returned, leaving the offset uncommitted so the message is redelivered.
	CommitOnFailure bool
}

type RetryingConsumer struct {
	logger   *zap.SugaredLogger
	opts     RetryOptions
	delegate events.MessageConsumer
}

func NewRetryingConsumer(logger *zap.SugaredLogger, delegate events.MessageConsumer) events.MessageConsumer {
	return NewRetryingConsumerWithOpts(logger, delegate, RetryOptions{
		Attempts:  DefaultAttempts,
		Delay:     DefaultDelay,
		DelayType: DefaultDelayType,
	})
}

func NewRetryingConsumerWithOpts(logger *zap.SugaredLogger, delegate events.MessageConsumer, opts RetryOptions) events.MessageConsumer {
	return &RetryingConsumer{
		logger:   logger,
		opts:     opts,
		delegate: delegate,
	}
}

func (r *RetryingConsumer) Initialize(config *events.CloudEventsConfig) error {
	return r.delegate.Initialize(config)
}

func (r *RetryingConsumer) HandleKafkaMessage(cm *sarama.ConsumerMessage) error {
	retryFn := func() error { return r.delegate.HandleKafkaMessage(cm) }
	err := retry.Do(
		retryFn,
		retry.Attempts(r.opts.Attempts),
		retry.Delay(r.opts.Delay),
		retry.DelayType(r.opts.DelayType),
	)
	if err != nil && r.opts.CommitOnFailure {
		// Give up and commit the offset so the message is not redelivered.
		r.logger.Warnw("giving up on message and committing offset", "offset", cm.Offset, "attempts", r.opts.Attempts, zap.Error(err))
		return nil
	}
	return err
}

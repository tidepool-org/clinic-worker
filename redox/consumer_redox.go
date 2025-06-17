package redox

import (
	"context"
	"github.com/IBM/sarama"
	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/go-common/asyncevents"
	"time"
)

type Consumer struct {
}

func (c *Consumer) Consume(ctx context.Context, session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) error {

}

func (c *Consumer) Run(ctx context.Context) error {

}

func CreateRedoxConsumer(p MessageCDCConsumerParams) (asyncevents.SaramaEventsRunner, error) {
	if !p.Config.Enabled {
		return &cdc.DisabledSaramaEventsRunner{}, nil
	}

	config, err := cdc.GetConfig()
	if err != nil {
		return nil, err
	}

	config.KafkaTopic = redoxMessageTopic
	config.SaramaConfig.ClientID = config.KafkaTopicPrefix + "clinic-worker"

	prefixedTopics := []string{config.GetPrefixedTopic()}

	runnerCfg := asyncevents.SaramaRunnerConfig{
		Brokers:         config.KafkaBrokers,
		GroupID:         config.KafkaConsumerGroup,
		Topics:          prefixedTopics,
		Sarama:          config.SaramaConfig,
		MessageConsumer: &Consumer{},
	}

	delays := []time.Duration{0, time.Second * 60, time.Second * 300}
	logger := &cdc.AsynceventsLoggerAdapter{
		SugaredLogger: p.Logger,
	}

	eventsRunner := asyncevents.NewCascadingSaramaEventsRunner(runnerCfg, logger, delays, defaultTimeout)
	return eventsRunner, nil
}

package cdc

import "context"

type DisabledEventConsumer struct{}

func (d *DisabledEventConsumer) Start() error {
	return nil
}

func (d *DisabledEventConsumer) Stop() error {
	return nil
}

type DisabledSaramaEventsRunner struct{}

func (d DisabledSaramaEventsRunner) Run(_ context.Context) error {
	return nil
}

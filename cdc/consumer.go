package cdc

type DisabledEventConsumer struct{}

func (d *DisabledEventConsumer) Start() error {
	return nil
}

func (d *DisabledEventConsumer) Stop() error {
	return nil
}

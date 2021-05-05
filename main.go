package main

import (
	"github.com/tidepool-org/clinic-worker/consumer"
)

func main() {
	consumer.New().Run()
}

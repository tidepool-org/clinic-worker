module github.com/tidepool-org/clinic-worker

go 1.16

require (
	github.com/Shopify/sarama v1.28.0
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/deepmap/oapi-codegen v1.9.0
	github.com/golang/mock v1.5.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/tidepool-org/clinic/client v0.0.0-20220302194442-93d92873a9ad
	github.com/tidepool-org/go-common v0.8.3-0.20211014160932-4757171e3914
	go.uber.org/fx v1.13.1
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/ratelimit v0.2.0
	go.uber.org/zap v1.16.0
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
)

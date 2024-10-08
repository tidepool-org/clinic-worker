package worker_test

import (
	"os"

	"github.com/IBM/sarama"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	MockBrokerAddress = "localhost:8432"
)

// TODO samara update broke this test? hangs forever.
//var _ = Describe("Boostrap", func() {
//	Describe("Fx App", func() {
//		var app *fx.App
//		var components worker.Components
//		var broker *sarama.MockBroker
//
//		BeforeEach(func() {
//			SetRequiredEnvVariables()
//
//			broker = NewMockKafkaBroker()
//			Expect(broker).ToNot(BeNil())
//
//			broker.SetHandlerByMap(map[string]sarama.MockResponse{
//				"MetadataRequest": sarama.NewMockMetadataResponse(GinkgoT()).
//					SetBroker(broker.Addr(), broker.BrokerID()).
//					SetLeader("dev1-mailer", 0, broker.BrokerID()),
//				"OffsetRequest": sarama.NewMockOffsetResponse(GinkgoT()).
//					SetOffset("test-topic", 0, sarama.OffsetOldest, 0).
//					SetOffset("test-topic", 0, sarama.OffsetNewest, 0),
//			})
//
//			init := func(c worker.Components) {
//				components = c
//			}
//			opts := append([]fx.Option{}, worker.Modules...)
//			opts = append(opts, fx.Invoke(init), fx.NopLogger)
//
//			app = fx.New(opts...)
//			Expect(app).ToNot(BeNil())
//		})
//
//		AfterEach(func() {
//			if broker != nil {
//				broker.Close()
//			}
//			components = worker.Components{}
//			ClearRequiredEnvVariables()
//		})
//
//		It("build the DI graph successfully", func() {
//			Expect(app.Err()).ToNot(HaveOccurred())
//		})
//
//		It("instantiates a health check server", func() {
//			Expect(components.HealthCheckServer).ToNot(BeNil())
//		})
//
//		It("instantiates workers", func() {
//			// clinic, clinicians, migration, patients, patientsummary, users, datasources, redox, redox-scheduled
//			expectedCount := 9
//			Expect(components.Consumers).To(HaveLen(expectedCount))
//		})
//	})
//})

func NewMockKafkaBroker() *sarama.MockBroker {
	return sarama.NewMockBrokerAddr(GinkgoT(), 0, MockBrokerAddress)
}

func SetRequiredEnvVariables() {
	Expect(os.Setenv("TIDEPOOL_SERVER_SECRET", "dummy")).ToNot(HaveOccurred())
	Expect(os.Setenv("KAFKA_BROKERS", MockBrokerAddress)).ToNot(HaveOccurred())
	Expect(os.Setenv("KAFKA_TOPIC_PREFIX", "local")).ToNot(HaveOccurred())
	Expect(os.Setenv("KAFKA_REQUIRE_SSL", "false")).ToNot(HaveOccurred())
	Expect(os.Setenv("KAFKA_VERSION", "2.6.0")).ToNot(HaveOccurred())
	Expect(os.Setenv("TIDEPOOL_REDOX_CLIENT_ID", "1234567890")).ToNot(HaveOccurred())
	Expect(os.Setenv("TIDEPOOL_REDOX_KEY_ID", "k-123")).ToNot(HaveOccurred())
	Expect(os.Setenv("TIDEPOOL_REDOX_PRIVATE_KEY", "PK")).ToNot(HaveOccurred())
	Expect(os.Setenv("TIDEPOOL_REDOX_SOURCE_ID", "Tidepool-Test-123")).ToNot(HaveOccurred())
	Expect(os.Setenv("TIDEPOOL_REDOX_SOURCE_NAME", "Tidepool-Test")).ToNot(HaveOccurred())
}

func ClearRequiredEnvVariables() {
	Expect(os.Unsetenv("TIDEPOOL_SERVER_SECRET")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("KAFKA_BROKERS")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("KAFKA_TOPIC_PREFIX")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("KAFKA_REQUIRE_SSL")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("KAFKA_VERSION")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("TIDEPOOL_REDOX_CLIENT_ID")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("TIDEPOOL_REDOX_KEY_ID")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("TIDEPOOL_REDOX_PRIVATE_KEY")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("TIDEPOOL_REDOX_SOURCE_ID")).ToNot(HaveOccurred())
	Expect(os.Unsetenv("TIDEPOOL_REDOX_SOURCE_NAME")).ToNot(HaveOccurred())
}

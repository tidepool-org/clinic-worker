package datasources_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic-worker/cdc"
	"github.com/tidepool-org/clinic-worker/datasources"
	clinics "github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/go-common/clients"
)

var _ = Describe("CDCEvent", func() {
	Describe("CreateUpdateBody", func() {
		var event datasources.CDCEvent
		var source clients.DataSource

		BeforeEach(func() {
			dataSourceId := primitive.NewObjectID().Hex()
			providerName := "dexcom"
			state := "connected"
			createdTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
			modifiedTime := createdTime.Add(time.Hour)

			event = datasources.CDCEvent{
				FullDocument: datasources.DataSource{
					ID: &cdc.ObjectId{Value: dataSourceId},
				},
			}
			source = clients.DataSource{
				ProviderName: &providerName,
				State:        &state,
				CreatedTime:  &createdTime,
				ModifiedTime: &modifiedTime,
			}
		})

		It("sets the created time of the data source", func() {
			body := event.CreateUpdateBody(source)
			Expect(body.CreatedTime).ToNot(BeNil())
			Expect(string(*body.CreatedTime)).To(Equal("2025-01-01T00:00:00Z"))
		})

		It("omits the created time when the data source doesn't have one", func() {
			source.CreatedTime = nil
			body := event.CreateUpdateBody(source)
			Expect(body.CreatedTime).To(BeNil())
		})

		It("sets the remaining data source attributes", func() {
			body := event.CreateUpdateBody(source)
			Expect(body.DataSourceId).ToNot(BeNil())
			Expect(*body.DataSourceId).To(Equal(event.FullDocument.ID.Value))
			Expect(string(body.ProviderName)).To(Equal("dexcom"))
			Expect(body.State).To(Equal(clinics.DataSourceV1State("connected")))
			Expect(body.ModifiedTime).ToNot(BeNil())
			Expect(string(*body.ModifiedTime)).To(Equal("2025-01-01T01:00:00Z"))
		})
	})
})

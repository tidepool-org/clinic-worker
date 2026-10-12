package patients_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/patients"
	summaries "github.com/tidepool-org/go-common/clients/summary"
)

var _ = Describe("CreateSummaryUpdateBody", func() {
	var lastUpdatedDate time.Time
	var lastUploadDate time.Time

	BeforeEach(func() {
		lastUpdatedDate = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
		lastUploadDate = time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)
	})

	newSummary := func() *summaries.SummaryV5 {
		return &summaries.SummaryV5{
			Dates: summaries.SummaryDatesV5{
				LastUpdatedDate: lastUpdatedDate,
				LastUploadDate:  lastUploadDate,
			},
		}
	}

	It("exports the CGM and BGM summary dates from their own fields", func() {
		body, err := patients.CreateSummaryUpdateBody(newSummary(), newSummary())
		Expect(err).ToNot(HaveOccurred())

		Expect(body.CgmStats).ToNot(BeNil())
		Expect(body.CgmStats.Dates.LastUpdatedDate).To(HaveValue(Equal(lastUpdatedDate)))
		Expect(body.CgmStats.Dates.HasLastUploadDate).To(BeTrue())
		Expect(body.CgmStats.Dates.LastUploadDate).To(HaveValue(Equal(lastUploadDate)))

		Expect(body.BgmStats).ToNot(BeNil())
		Expect(body.BgmStats.Dates.LastUpdatedDate).To(HaveValue(Equal(lastUpdatedDate)))
		Expect(body.BgmStats.Dates.HasLastUploadDate).To(BeTrue())
		Expect(body.BgmStats.Dates.LastUploadDate).To(HaveValue(Equal(lastUploadDate)))
	})

	It("omits the last upload date when the summaries have none", func() {
		cgmSummary := newSummary()
		cgmSummary.Dates.LastUploadDate = time.Time{}
		bgmSummary := newSummary()
		bgmSummary.Dates.LastUploadDate = time.Time{}

		body, err := patients.CreateSummaryUpdateBody(cgmSummary, bgmSummary)
		Expect(err).ToNot(HaveOccurred())

		Expect(body.CgmStats.Dates.HasLastUploadDate).To(BeFalse())
		Expect(body.CgmStats.Dates.LastUploadDate).To(BeNil())
		Expect(body.BgmStats.Dates.HasLastUploadDate).To(BeFalse())
		Expect(body.BgmStats.Dates.LastUploadDate).To(BeNil())
	})
})

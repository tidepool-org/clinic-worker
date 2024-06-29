package patientsummary_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/patientsummary"
)

var _ = Describe("ShouldTriggerEHRSync", func() {
	var dates *patientsummary.Dates
	var bgmSummary *patientsummary.Summary[patientsummary.BGMStats]
	var cgmSummary *patientsummary.Summary[patientsummary.CGMStats]

	BeforeEach(func() {
		lastUpdatedReason := []string{"UPLOAD_COMPLETED"}
		dates = &patientsummary.Dates{
			LastUpdatedReason: &lastUpdatedReason,
		}
		cgm := "cgm"
		bgm := "bgm"
		bgmSummary = &patientsummary.Summary[patientsummary.BGMStats]{
			Type:  &bgm,
			Dates: dates,
		}
		cgmSummary = &patientsummary.Summary[patientsummary.CGMStats]{
			Type:  &cgm,
			Dates: dates,
		}
	})

	It("should be true for cgm summaries", func() {
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be true for bgm summaries", func() {
		Expect(patientsummary.ShouldTriggerEHRSync(*bgmSummary)).To(BeTrue())
	})

	It("should be false if type is continuous", func() {
		continuous := "continuous"
		cgmSummary.Type = &continuous
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be true when last updated reason is UPLOAD_COMPLETED", func() {
		(*cgmSummary.Dates.LastUpdatedReason)[0] = "UPLOAD_COMPLETED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be true when last updated reason is LEGACY_UPLOAD_COMPLETED", func() {
		(*cgmSummary.Dates.LastUpdatedReason)[0] = "LEGACY_UPLOAD_COMPLETED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be true when last updated reason is LEGACY_DATA_ADDED", func() {
		(*cgmSummary.Dates.LastUpdatedReason)[0] = "LEGACY_UPLOAD_COMPLETED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be false when last updated reason is DATA_ADDED", func() {
		(*cgmSummary.Dates.LastUpdatedReason)[0] = "DATA_ADDED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be false when last updated reason is empty", func() {
		lastUpdatedReason := []string{}
		cgmSummary.Dates.LastUpdatedReason = &lastUpdatedReason
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be false when last updated reason is nil", func() {
		cgmSummary.Dates.LastUpdatedReason = nil
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be false when outdated reason is not empty", func() {
		outdatedReason := []string{"LEGACY_UPLOAD_COMPLETED"}
		cgmSummary.Dates.OutdatedReason = &outdatedReason
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be true when outdated reason is not nil but empty", func() {
		outdatedReason := []string{}
		cgmSummary.Dates.OutdatedReason = &outdatedReason
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})
})

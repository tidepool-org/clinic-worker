package patientsummary_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic-worker/patientsummary"
)

var _ = Describe("ShouldTriggerEHRSync", func() {
	var dates *patientsummary.Dates
	var bgmSummary *patientsummary.Summary
	var cgmSummary *patientsummary.Summary

	BeforeEach(func() {
		lastUpdatedReason := []string{"UPLOAD_COMPLETED"}
		dates = &patientsummary.Dates{
			LastUpdatedReason: lastUpdatedReason,
		}
		cgm := "cgm"
		bgm := "bgm"
		bgmSummary = &patientsummary.Summary{
			BaseSummary: patientsummary.BaseSummary{
				Type:  bgm,
				Dates: *dates,
			},
		}
		cgmSummary = &patientsummary.Summary{
			BaseSummary: patientsummary.BaseSummary{
				Type:  cgm,
				Dates: *dates,
			},
		}
	})

	It("should be true for cgm summaries", func() {
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be true for bgm summaries", func() {
		Expect(patientsummary.ShouldTriggerEHRSync(*bgmSummary)).To(BeTrue())
	})

	It("should be false if type is continuous", func() {
		cgmSummary.Type = "continuous"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be true when last updated reason is UPLOAD_COMPLETED", func() {
		cgmSummary.Dates.LastUpdatedReason[0] = "UPLOAD_COMPLETED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be true when last updated reason is LEGACY_UPLOAD_COMPLETED", func() {
		cgmSummary.Dates.LastUpdatedReason[0] = "LEGACY_UPLOAD_COMPLETED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be true when last updated reason is LEGACY_DATA_ADDED", func() {
		cgmSummary.Dates.LastUpdatedReason[0] = "LEGACY_UPLOAD_COMPLETED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})

	It("should be false when last updated reason is DATA_ADDED", func() {
		cgmSummary.Dates.LastUpdatedReason[0] = "DATA_ADDED"
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be false when last updated reason is empty", func() {
		cgmSummary.Dates.LastUpdatedReason = []string{}
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be false when last updated reason is nil", func() {
		cgmSummary.Dates.LastUpdatedReason = nil
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be false when outdated reason is not empty", func() {
		cgmSummary.Dates.OutdatedReason = []string{"LEGACY_UPLOAD_COMPLETED"}
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeFalse())
	})

	It("should be true when outdated reason is not nil but empty", func() {
		cgmSummary.Dates.OutdatedReason = []string{}
		Expect(patientsummary.ShouldTriggerEHRSync(*cgmSummary)).To(BeTrue())
	})
})

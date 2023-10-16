package test

import (
	"context"
	"github.com/tidepool-org/clinic-worker/redox"
)

type ScheduledOrderProcessor struct {
	Scheduled []redox.ScheduledSummaryAndReport
}

func (t *ScheduledOrderProcessor) ProcessOrder(ctx context.Context, scheduled redox.ScheduledSummaryAndReport) error {
	t.Scheduled = append(t.Scheduled, scheduled)
	return nil
}

var _ redox.ScheduledSummaryAndReportProcessor = &ScheduledOrderProcessor{}

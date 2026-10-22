package handlers

import (
	"testing"
	"time"

	"bdscdailyreport/internal/window"
)

// The range ListSuCo queries for report date D must contain exactly the
// rows ReportDateForEntry labels D — both the auto-imported RSM row stamped
// at the window start and a manual row typed in the next morning.
func TestSuCoReportDateRangeMatchesLabel(t *testing.T) {
	rh, rm := 9, 0
	d := time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local)
	start, _ := window.ReportWindow(reportRunAt(d, rh, rm), rh, rm)
	_, end := window.ReportWindow(reportRunAt(d, rh, rm), rh, rm)

	in := []time.Time{
		time.Date(2026, 10, 21, 9, 1, 0, 0, time.Local),  // RSM auto-import (window start)
		time.Date(2026, 10, 22, 7, 30, 0, 0, time.Local), // manual, next morning before cutoff
	}
	out := []time.Time{
		time.Date(2026, 10, 21, 8, 30, 0, 0, time.Local), // previous report date
		time.Date(2026, 10, 22, 9, 1, 0, 0, time.Local),  // next report date
	}
	for _, ts := range in {
		if ts.Before(start) || ts.After(end) {
			t.Errorf("%v should be inside [%v, %v]", ts, start, end)
		}
		if got := window.ReportDateForEntry(ts, rh, rm).Format("2006-01-02"); got != "2026-10-21" {
			t.Errorf("ReportDateForEntry(%v) = %s, want 2026-10-21", ts, got)
		}
	}
	for _, ts := range out {
		if !ts.Before(start) && !ts.After(end) {
			t.Errorf("%v should be outside [%v, %v]", ts, start, end)
		}
	}
}

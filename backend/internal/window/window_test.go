package window

import (
	"testing"
	"time"
)

func TestApprovalDeadline(t *testing.T) {
	day := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, time.Local) }
	cases := []struct {
		name       string
		reportDate time.Time
		h, m       int
		rh, rm     int
		want       time.Time
	}{
		// Report 05/11 is generated 06/11 09:00 -> 08:50 deadline is 07/11 08:50, not 06/11.
		{"before report time -> next morning", day(5, 0, 0), 8, 50, 9, 0, day(7, 8, 50)},
		{"just before report time", day(5, 0, 0), 8, 59, 9, 0, day(7, 8, 59)},
		{"same minute as report time -> next day", day(5, 0, 0), 9, 0, 9, 0, day(7, 9, 0)},
		{"after report time -> generation day", day(5, 0, 0), 10, 0, 9, 0, day(6, 10, 0)},
		{"time-of-day of reportDate is ignored", day(5, 15, 30), 8, 50, 9, 0, day(7, 8, 50)},
	}
	for _, c := range cases {
		if got := ApprovalDeadline(c.reportDate, c.h, c.m, c.rh, c.rm); !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// At 08:50 the report up for review is the one generated yesterday at 09:00,
// and its deadline is exactly now; the report generated at 09:00 today must
// not be overdue at 09:01.
func TestApprovalDeadlineWithCurrentReport(t *testing.T) {
	at850 := time.Date(2026, 11, 7, 8, 50, 0, 0, time.Local)
	cur := CurrentReportDate(at850, 9, 0) // 05/11
	if at850.Before(ApprovalDeadline(cur, 8, 50, 9, 0)) {
		t.Errorf("08:50: report %s should be due", cur.Format("2006-01-02"))
	}
	at901 := time.Date(2026, 11, 7, 9, 1, 0, 0, time.Local)
	cur = CurrentReportDate(at901, 9, 0) // 06/11, just generated
	if !at901.Before(ApprovalDeadline(cur, 8, 50, 9, 0)) {
		t.Errorf("09:01: freshly generated report %s must not be due yet", cur.Format("2006-01-02"))
	}
}

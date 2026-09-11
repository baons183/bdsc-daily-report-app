// Package window implements the "báo cáo ngày" reporting window: each daily
// report covers the period from 08:01 the previous day to 07:59 the current
// day. This matches the exact rule carried over from the app's earlier
// Power Apps implementation.
package window

import "time"

// EntryWindow returns the window a new data-entry row at time `now` belongs
// to. Before 08:01 local time, `now` still belongs to the window that is
// about to close (yesterday 08:01 -> today 07:59). At or after 08:01, it
// belongs to the window that just opened (today 08:01 -> tomorrow 07:59).
func EntryWindow(now time.Time) (start, end time.Time) {
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 8, 1, 0, 0, now.Location())
	if now.Before(cutoff) {
		y, m, d := now.AddDate(0, 0, -1).Date()
		start = time.Date(y, m, d, 8, 1, 0, 0, now.Location())
		y, m, d = now.Date()
		end = time.Date(y, m, d, 7, 59, 59, 0, now.Location())
		return
	}
	y, m, d := now.Date()
	start = time.Date(y, m, d, 8, 1, 0, 0, now.Location())
	y, m, d = now.AddDate(0, 0, 1).Date()
	end = time.Date(y, m, d, 7, 59, 59, 0, now.Location())
	return
}

// ReportWindow returns the fixed, always-backward-looking window used when
// generating the daily report that runs at 08:00: yesterday 08:01 through
// today 07:59, relative to `runAt` (normally the moment the report job
// fires).
// SQL formats a time.Time as a plain "YYYY-MM-DD HH:MM:SS.mmm" literal for
// use as a query parameter compared against a DATETIME column. Passing a
// Go time.Time value directly lets the driver pick its own wire type
// (observed to sometimes disagree with a plain DATETIME column's implicit
// conversion, silently shifting comparisons) — a plain string sidesteps
// that ambiguity entirely, the same way NgayBaoCao's DATE comparisons
// already do via a formatted string.
func SQL(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}

func ReportWindow(runAt time.Time) (start, end time.Time) {
	y, m, d := runAt.Date()
	end = time.Date(y, m, d, 7, 59, 59, 0, runAt.Location())
	y, m, d = runAt.AddDate(0, 0, -1).Date()
	start = time.Date(y, m, d, 8, 1, 0, 0, runAt.Location())
	return
}

// Package window implements the "báo cáo ngày" reporting window: each daily
// report covers the period from just after one day's report-send time to
// just before the next day's — e.g. with the (Admin-configurable, default
// 08:00) send time left at its default, that is 08:01 the previous day to
// 07:59 the current day, matching the exact rule carried over from the
// app's earlier Power Apps implementation. Every function here takes the
// currently configured (reportHour, reportMinute) explicitly rather than
// hardcoding 8:00, so moving the send time in the Admin panel immediately
// reshapes which rows fall in which day's report — callers get the pair
// via db.ReportTime.
package window

import "time"

// cutoff returns the instant on t's calendar date that reportHour:reportMinute
// names — the moment the daily report is configured to run.
func cutoff(t time.Time, reportHour, reportMinute int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), reportHour, reportMinute, 0, 0, t.Location())
}

// EntryWindow returns the window a new data-entry row at time `now` belongs
// to. Before 1 minute past the report time, `now` still belongs to the
// window that is about to close (yesterday's cutoff+1min -> today's
// cutoff-1min-1sec). At or after that, it belongs to the window that just
// opened. The ±1 minute/second buffer around the exact report time keeps a
// row saved in the same instant the report runs from being double-counted
// by two adjacent windows' inclusive SQL ranges.
func EntryWindow(now time.Time, reportHour, reportMinute int) (start, end time.Time) {
	c := cutoff(now, reportHour, reportMinute).Add(time.Minute)
	if now.Before(c) {
		start = c.AddDate(0, 0, -1)
		end = c.Add(-time.Minute - time.Second)
		return
	}
	start = c
	end = c.AddDate(0, 0, 1).Add(-time.Minute - time.Second)
	return
}

// ReportWindow returns the fixed, always-backward-looking window used when
// generating the daily report that runs at reportHour:reportMinute:
// yesterday's cutoff+1min through today's cutoff-1min-1sec, relative to
// `runAt` (normally the moment the report job fires — only its calendar
// date matters, not its time-of-day).
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

func ReportWindow(runAt time.Time, reportHour, reportMinute int) (start, end time.Time) {
	c := cutoff(runAt, reportHour, reportMinute).Add(time.Minute)
	end = c.Add(-time.Minute - time.Second)
	start = c.AddDate(0, 0, -1)
	return
}

// DisplayBounds returns the clean, human-facing start/end for a report
// window — the exact configured report time on each boundary's calendar
// day, undoing the internal ±1 minute/second buffer EntryWindow/
// ReportWindow use internally. Given start/end from either of those (whose
// calendar dates are already correct), this just overwrites the
// time-of-day with reportHour:reportMinute so "Dữ liệu từ ... đến ..."
// reads e.g. "09:00 .../... đến 09:00 ..." instead of "09:01/08:59" when
// the report time is set to 9:00.
func DisplayBounds(start, end time.Time, reportHour, reportMinute int) (displayStart, displayEnd time.Time) {
	displayStart = cutoff(start, reportHour, reportMinute)
	displayEnd = cutoff(end, reportHour, reportMinute)
	return
}

// ReportDateForEntry returns the "printed" report date (report.Generate's
// reportDate = RunAt-1 convention) that a data row entered at time `t`
// will end up appearing in once its window's report is generated — used
// to check whether that specific row's own report has been reviewed/
// approved (and is therefore locked) regardless of what time it is now.
func ReportDateForEntry(t time.Time, reportHour, reportMinute int) time.Time {
	_, end := EntryWindow(t, reportHour, reportMinute)
	return end.AddDate(0, 0, -1)
}

// CurrentReportDate returns the printed report date that is "active" for
// review/approval right now, at time `t` — the most recently generated
// daily report: today's (t's date minus 1) if today's report run has
// already fired (t is at/after the report-time+1min cutoff), otherwise
// still yesterday's run's report (t's date minus 2), since today's hasn't
// been generated yet.
func CurrentReportDate(t time.Time, reportHour, reportMinute int) time.Time {
	c := cutoff(t, reportHour, reportMinute).Add(time.Minute)
	if t.Before(c) {
		return t.AddDate(0, 0, -2)
	}
	return t.AddDate(0, 0, -1)
}

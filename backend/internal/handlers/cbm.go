package handlers

import (
	"net/http"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

// ListCBM shows the exact same rows (filtered to the report window of the
// given "?date=YYYY-MM-DD", or today if omitted) that would appear in
// section 3.1 of that day's PDF — read-only, since this data comes entirely
// from the external online-monitoring system's Excel export, not from
// anything users enter in this app.
func (e *Env) ListCBM(w http.ResponseWriter, r *http.Request) {
	runAt := time.Now()
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			writeErr(w, 400, "ngay khong hop le, can dang YYYY-MM-DD")
			return
		}
		runAt = time.Date(d.Year(), d.Month(), d.Day(), 8, 0, 0, 0, time.Local)
	}
	start, end := window.ReportWindow(runAt)
	rows := report.ReadCBMExcel(e.CBMExcelPath, start, end)
	if rows == nil {
		rows = []models.CBMRow{}
	}
	writeJSON(w, 200, rows)
}

package handlers

import (
	"net/http"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

// ListCBM shows the exact same rows (filtered to the current report window)
// that would appear in section 3.1 of today's PDF — read-only, since this
// data comes entirely from the external online-monitoring system's Excel
// export, not from anything users enter in this app.
func (e *Env) ListCBM(w http.ResponseWriter, r *http.Request) {
	start, end := window.ReportWindow(time.Now())
	rows := report.ReadCBMExcel(e.CBMExcelPath, start, end)
	if rows == nil {
		rows = []models.CBMRow{}
	}
	writeJSON(w, 200, rows)
}

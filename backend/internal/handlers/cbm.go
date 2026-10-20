package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"bdscdailyreport/internal/auth"
	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

// cbmRowResp is models.CBMRow plus whether the row is currently ticked
// "đưa vào báo cáo" — Included is false by default and only true once a
// user has explicitly ticked this exact row (dbo.CBMInclusions), so the
// CBM tab can show its checkbox state while report generation
// (report.FilterCBMIncluded) keeps only rows ticked in.
type cbmRowResp struct {
	models.CBMRow
	Included bool `json:"included"`
}

// ListCBM shows the exact same rows (filtered to the report window of the
// given "?date=YYYY-MM-DD", or today if omitted) that would appear in
// section 3.1 of that day's PDF — data itself comes entirely from the
// external online-monitoring system's Excel export, not anything users
// enter in this app, but each row's "đưa vào báo cáo" checkbox IS editable
// here (see SetCBMInclusion).
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
	rh, rm := db.ReportTime(e.DB, 8, 0)
	start, end := window.ReportWindow(runAt, rh, rm)
	rows := report.ReadCBMExcel(e.effectiveCBMExcelPath(), start, end)
	included, err := db.GetCBMInclusionSet(e.DB)
	if err != nil {
		included = map[string]bool{}
	}
	out := make([]cbmRowResp, 0, len(rows))
	for _, row := range rows {
		out = append(out, cbmRowResp{
			CBMRow:   row,
			Included: included[models.CBMRowKey(row.Tagname, row.Description, row.DataTime)],
		})
	}
	writeJSON(w, 200, out)
}

type setCBMInclusionReq struct {
	Tagname     string    `json:"tagname"`
	Description string    `json:"description"`
	DataTime    time.Time `json:"dataTime"`
	Included    bool      `json:"included"`
}

// SetCBMInclusion lets a user tick/untick whether one specific CBM alarm
// row is included the next time a report is generated (PDF/email/Thống
// kê/Báo cáo tuần) — the raw CBM tab itself always lists every row
// regardless of this setting (unticked by default). Restricted to
// GiamSat/Moderator/Approver (Admin always passes via auth.HasRole's own
// bypass) — the CBM tab itself has no role gate (role: null, so even a
// view-only registered account sees it), but deciding what goes in the
// actual report is not a view-only action.
func (e *Env) SetCBMInclusion(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil || !(auth.HasRole(user.Roles, "GiamSat") || auth.HasRole(user.Roles, "Moderator") || auth.HasRole(user.Roles, "Approver")) {
		writeErr(w, 403, "ban khong co quyen chon dua/khong dua du lieu nay vao bao cao")
		return
	}
	var in setCBMInclusionReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Tagname == "" {
		writeErr(w, 400, "du lieu khong hop le")
		return
	}
	updatedBy := user.EmployeeCode
	if err := db.SetCBMInclusion(e.DB, in.Tagname, in.Description, in.DataTime, in.Included, updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

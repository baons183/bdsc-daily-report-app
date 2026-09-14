package handlers

import (
	"encoding/json"
	"net/http"
)

type reportScheduleResp struct {
	ReportHour   int `json:"reportHour"`
	ReportMinute int `json:"reportMinute"`
}

// GetReportSchedule returns the current auto-run time for the daily report
// (default 08:00 if the row is somehow missing).
func (e *Env) GetReportSchedule(w http.ResponseWriter, r *http.Request) {
	var out reportScheduleResp
	err := e.DB.QueryRowContext(r.Context(), `SELECT ReportHour, ReportMinute FROM dbo.ReportSchedule WHERE Id = 1`).
		Scan(&out.ReportHour, &out.ReportMinute)
	if err != nil {
		out = reportScheduleResp{ReportHour: 8, ReportMinute: 0}
	}
	writeJSON(w, 200, out)
}

type setReportScheduleReq struct {
	ReportHour   int `json:"reportHour"`
	ReportMinute int `json:"reportMinute"`
}

// SetReportSchedule lets an Admin change the daily report's auto-run time
// without editing .env or restarting the app — scheduleDailyReport polls
// this table every minute, so a change takes effect within a minute.
func (e *Env) SetReportSchedule(w http.ResponseWriter, r *http.Request) {
	var in setReportScheduleReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.ReportHour < 0 || in.ReportHour > 23 || in.ReportMinute < 0 || in.ReportMinute > 59 {
		writeErr(w, 400, "gio/phut khong hop le")
		return
	}
	user := currentUser(r)
	updatedBy := ""
	if user != nil {
		updatedBy = user.EmployeeCode
	}
	if _, err := e.DB.ExecContext(r.Context(), `
UPDATE dbo.ReportSchedule SET ReportHour = ?, ReportMinute = ?, UpdatedBy = ?, UpdatedAt = GETDATE() WHERE Id = 1`,
		in.ReportHour, in.ReportMinute, updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

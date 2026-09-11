package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type ScheduleOverride struct {
	Id           int    `json:"id"`
	OverrideDate string `json:"overrideDate"`
	CreatedBy    string `json:"createdBy"`
}

// ListScheduleOverrides shows the specific Saturdays/Sundays configured to
// still run the daily report despite the normal weekend skip.
func (e *Env) ListScheduleOverrides(w http.ResponseWriter, r *http.Request) {
	rows, err := e.DB.QueryContext(r.Context(),
		`SELECT Id, CONVERT(varchar(10), OverrideDate, 120), ISNULL(CreatedBy,'') FROM dbo.ScheduleOverrides ORDER BY OverrideDate`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []ScheduleOverride{}
	for rows.Next() {
		var s ScheduleOverride
		if err := rows.Scan(&s.Id, &s.OverrideDate, &s.CreatedBy); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, s)
	}
	writeJSON(w, 200, out)
}

type addOverrideReq struct {
	Date string `json:"date"`
}

// AddScheduleOverride marks one specific weekend date to still run.
func (e *Env) AddScheduleOverride(w http.ResponseWriter, r *http.Request) {
	var in addOverrideReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Date == "" {
		writeErr(w, 400, "thieu date")
		return
	}
	if _, err := time.Parse("2006-01-02", in.Date); err != nil {
		writeErr(w, 400, "ngay khong hop le, can dang YYYY-MM-DD")
		return
	}
	user := currentUser(r)
	createdBy := ""
	if user != nil {
		createdBy = user.EmployeeCode
	}
	if _, err := e.DB.ExecContext(r.Context(), `
IF NOT EXISTS (SELECT 1 FROM dbo.ScheduleOverrides WHERE OverrideDate = ?)
INSERT INTO dbo.ScheduleOverrides (OverrideDate, CreatedBy) VALUES (?, ?)`,
		in.Date, in.Date, createdBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

func (e *Env) DeleteScheduleOverride(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `DELETE FROM dbo.ScheduleOverrides WHERE Id = ?`, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

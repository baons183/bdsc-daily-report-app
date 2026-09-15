package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
)

// GetRSMReview shows every IV.2 row (filtered to Ban BDSC's own
// departments) for one given "?date=YYYY-MM-DD" report date, WITH each
// row's current "đưa vào báo cáo" tick state — unlike report.
// FetchRSMSection (which drops excluded rows entirely), this lists
// everything so the checkbox itself has something to render regardless of
// state.
func (e *Env) GetRSMReview(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		writeErr(w, 400, "thieu tham so date (YYYY-MM-DD)")
		return
	}
	d, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		writeErr(w, 400, "ngay khong hop le, can dang YYYY-MM-DD")
		return
	}

	rows := report.GatherRSMRowsForDate(e.effectiveRSMPdfFolder(), d)
	excluded, err := db.GetRSMExclusionSet(e.DB)
	if err != nil {
		excluded = map[string]bool{}
	}
	out := make([]models.RSMRow, len(rows))
	for i, row := range rows {
		row.Included = !excluded[models.RSMIssueKey(row.TagName, row.Issue)]
		out[i] = row
	}
	writeJSON(w, 200, out)
}

type setRSMExclusionReq struct {
	TagName  string `json:"tagName"`
	Issue    string `json:"issue"`
	Included bool   `json:"included"`
}

// SetRSMSectionExclusion lets a user tick/untick whether one specific
// IV.2 issue (identified by TagName+Issue, so the decision carries
// forward automatically across every day the same open issue keeps
// reappearing) shows up the next time report section 3.3 is generated.
func (e *Env) SetRSMSectionExclusion(w http.ResponseWriter, r *http.Request) {
	var in setRSMExclusionReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.TagName == "" {
		writeErr(w, 400, "du lieu khong hop le")
		return
	}
	user := currentUser(r)
	updatedBy := ""
	if user != nil {
		updatedBy = user.EmployeeCode
	}
	if err := db.SetRSMExclusion(e.DB, in.TagName, in.Issue, !in.Included, updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

package report

import (
	"database/sql"
	"time"

	appdb "bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/rsm"
)

// GatherRSMRowsForDate parses reportDate's RSM Night Shift Report PDF (if
// the configured folder has a matching file) and filters it down to Ban
// BDSC's own departments (rsm.FilterByDepartment). Best-effort: a blank
// folder, a missing file for that date, or a PDF that fails to parse all
// return nil rather than an error, since section 3.3 is optional/
// supplementary — never something that should block the rest of the
// report.
func GatherRSMRowsForDate(folder string, reportDate time.Time) []models.RSMRow {
	path, found := rsm.FindPDF(folder, reportDate)
	if !found {
		return nil
	}
	rows, err := rsm.ParsePDF(path)
	if err != nil {
		return nil
	}
	filtered := rsm.FilterByDepartment(rows, rsm.DepartmentKeywords)
	out := make([]models.RSMRow, 0, len(filtered))
	for _, r := range filtered {
		dueDate := ""
		if dt, ok := rsm.ParseDueDate(r.DueDate); ok {
			dueDate = dt.Format("2006-01-02")
		}
		out = append(out, models.RSMRow{
			Depart:         r.Depart,
			TagName:        r.TagName,
			Issue:          r.Issue,
			MitigationPlan: r.MitigationPlan,
			ActionBy:       r.ActionBy,
			DueDate:        dueDate,
			Status:         r.Status,
			Included:       true,
		})
	}
	return out
}

// FetchRSMSection gathers report section 3.3 for the window [start, end]
// — for the daily report this is a single calendar date; for Báo cáo
// tuần it's every date in range. The same issue (models.RSMIssueKey)
// typically keeps reappearing day after day until resolved, so rows are
// deduped across the range keeping each issue's LATEST day's version
// (same "keep the newest" rule the weekly report already applies to
// Công việc), then anything a user has unticked "đưa vào báo cáo" for
// (dbo.RSMSectionExclusions) is dropped.
func FetchRSMSection(conn *sql.DB, folder string, start, end time.Time) ([]models.RSMRow, error) {
	byKey := map[string]models.RSMRow{}
	var order []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		for _, row := range GatherRSMRowsForDate(folder, d) {
			key := models.RSMIssueKey(row.TagName, row.Issue)
			if _, ok := byKey[key]; !ok {
				order = append(order, key)
			}
			byKey[key] = row // a later date's version overwrites -> keeps the latest
		}
	}

	excluded, err := appdb.GetRSMExclusionSet(conn)
	if err != nil {
		excluded = map[string]bool{}
	}
	out := make([]models.RSMRow, 0, len(order))
	for _, key := range order {
		if excluded[key] {
			continue
		}
		out = append(out, byKey[key])
	}
	return out, nil
}

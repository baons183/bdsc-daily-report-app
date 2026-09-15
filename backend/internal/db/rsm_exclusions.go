package db

import (
	"database/sql"

	"bdscdailyreport/internal/models"
)

// GetRSMExclusionSet reads every recorded IV.2 issue exclusion (report
// section 3.3's "đưa vào báo cáo" unticked ones) into a set keyed by
// models.RSMIssueKey, for report.FetchRSMSection to consult.
func GetRSMExclusionSet(conn *sql.DB) (map[string]bool, error) {
	rows, err := conn.Query(`SELECT TagName, Issue FROM dbo.RSMSectionExclusions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var tag, issue string
		if err := rows.Scan(&tag, &issue); err != nil {
			return nil, err
		}
		out[models.RSMIssueKey(tag, issue)] = true
	}
	return out, rows.Err()
}

// SetRSMExclusion records or clears whether one specific IV.2 issue
// (identified by TagName+Issue) should be left out of report section 3.3.
// Idempotent either way.
func SetRSMExclusion(conn *sql.DB, tagName, issue string, excluded bool, updatedBy string) error {
	if !excluded {
		_, err := conn.Exec(`DELETE FROM dbo.RSMSectionExclusions WHERE TagName = ? AND Issue = ?`, tagName, issue)
		return err
	}
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM dbo.RSMSectionExclusions WHERE TagName = ? AND Issue = ?`,
		tagName, issue).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := conn.Exec(`INSERT INTO dbo.RSMSectionExclusions (TagName, Issue, ExcludedBy) VALUES (?, ?, ?)`,
		tagName, issue, updatedBy)
	return err
}

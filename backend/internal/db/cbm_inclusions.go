package db

import (
	"database/sql"
	"time"

	"bdscdailyreport/internal/models"
)

// GetCBMInclusionSet reads every recorded CBM inclusion (rows a user has
// explicitly ticked "đưa vào báo cáo" for in the Giám sát trực tuyến tab)
// into a set keyed by models.CBMRowKey, for report.FilterCBMIncluded to
// consult when building an actual report — a row with no entry here is
// left out of the report by default (opt-in).
func GetCBMInclusionSet(conn *sql.DB) (map[string]bool, error) {
	rows, err := conn.Query(`SELECT Tagname, Description, DataTime FROM dbo.CBMInclusions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var tag, desc string
		var dt time.Time
		if err := rows.Scan(&tag, &desc, &dt); err != nil {
			return nil, err
		}
		out[models.CBMRowKey(tag, desc, dt)] = true
	}
	return out, rows.Err()
}

// SetCBMInclusion records or clears whether one specific CBM alarm row
// (identified by Tagname+Description+DataTime) should be included in the
// generated report. Idempotent either way — ticking an already-included
// row or unticking an already-excluded one is a no-op.
//
// DataTime is formatted to a plain literal string before binding, same as
// window.SQL()/every other DATETIME comparison in this app — passing a Go
// time.Time value directly lets the driver pick its own wire type, which
// has been observed to disagree with a plain DATETIME column's implicit
// conversion and silently fail an equality comparison.
func SetCBMInclusion(conn *sql.DB, tagname, description string, dataTime time.Time, included bool, updatedBy string) error {
	dt := dataTime.Format("2006-01-02 15:04:05.000")
	if !included {
		_, err := conn.Exec(`DELETE FROM dbo.CBMInclusions WHERE Tagname = ? AND Description = ? AND DataTime = ?`,
			tagname, description, dt)
		return err
	}
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM dbo.CBMInclusions WHERE Tagname = ? AND Description = ? AND DataTime = ?`,
		tagname, description, dt).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := conn.Exec(`INSERT INTO dbo.CBMInclusions (Tagname, Description, DataTime, IncludedBy) VALUES (?, ?, ?, ?)`,
		tagname, description, dt, updatedBy)
	return err
}

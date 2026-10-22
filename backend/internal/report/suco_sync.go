package report

import (
	"database/sql"
	"time"

	"bdscdailyreport/internal/rsm"
	"bdscdailyreport/internal/window"
)

// SyncSuCoFromRSM auto-imports reportDate's RSM PDF section IV.3 (NEW
// TE/IC IN SHIFT — new Technical Event/Incident logged during the shift),
// filtered to Ban BDSC's own departments, into dbo.SuCo for the window
// [start, end], skipping any PDF entry (TagName|Issue) already imported
// into that window — matched by the row's RsmKey, which stays fixed when a
// user edits the row — or listed in dbo.SuCoRsmDismissed (a user deleted
// it), so it can be called repeatedly (every report generation, every time
// a user opens the Sự cố tab) without ever creating duplicates or undoing
// a user's edit/delete.
//
// IV.3 is very often empty (no new TE/IC that shift) — this is the normal
// case and simply results in nothing being imported, not an error.
// LoaiSuCo is defaulted to "Technical Event" (this section's own name is
// literally "TE/IC" — Technical Event/Incident — and LoaiSuCo has no
// "Incident" category of its own), editable afterward like any other
// field if a user reclassifies it; note this also feeds the KPI tab's
// auto-counted Sự cố/Technical event tally, same as a manually entered
// row would.
//
// Best-effort: any error here is swallowed, since this convenience sync
// should never block viewing or generating the rest of the report.
func SyncSuCoFromRSM(db *sql.DB, folder string, reportDate, start, end time.Time) {
	path, found := rsm.FindPDF(folder, reportDate)
	if !found {
		return
	}
	pdfRows, err := rsm.ParseNewTEIC(path)
	if err != nil {
		return
	}
	filtered := rsm.FilterByDepartment(pdfRows, rsm.DepartmentKeywords)
	if len(filtered) == 0 {
		return
	}

	existing := map[string]bool{}
	rows, err := db.Query(`
SELECT ISNULL(RsmKey, ThietBi + '|' + ISNULL(TinhTrang,'')) FROM dbo.SuCo WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?
UNION ALL
SELECT RsmKey FROM dbo.SuCoRsmDismissed WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?`,
		window.SQL(start), window.SQL(end), window.SQL(start), window.SQL(end))
	if err != nil {
		return
	}
	for rows.Next() {
		var key string
		if rows.Scan(&key) == nil {
			existing[key] = true
		}
	}
	rows.Close()

	for _, r := range filtered {
		key := r.TagName + "|" + r.Issue
		if existing[key] {
			continue
		}
		var thoiHan sql.NullTime
		if dt, ok := rsm.ParseDueDate(r.DueDate); ok {
			thoiHan = sql.NullTime{Time: dt, Valid: true}
		}
		donVi := rsm.KeepDepartmentTokens(r.ActionBy, rsm.DepartmentKeywords)
		if insertSuCoRow(db, r.TagName, r.Issue, r.MitigationPlan, donVi, thoiHan, start, end) {
			existing[key] = true // in case the PDF itself lists the same issue twice
		}
	}
}

// insertSuCoRow appends one auto-imported row at the next TT in
// [start, end]'s window, the same UPDLOCK/HOLDLOCK-serialized numbering
// every manual Create handler uses (see handlers.nextTTLocked).
func insertSuCoRow(db *sql.DB, thietBi, tinhTrang, phuongAn, donVi string, thoiHan sql.NullTime, start, end time.Time) bool {
	tx, err := db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM dbo.SuCo WITH (UPDLOCK, HOLDLOCK) WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?`,
		window.SQL(start), window.SQL(end)).Scan(&count); err != nil {
		return false
	}
	if _, err := tx.Exec(`INSERT INTO dbo.SuCo (TT, ThietBi, LoaiSuCo, TinhTrang, PhuongAn, DonVi, ThoiHan, NgayBaoCao, CreatedBy, RsmKey) VALUES (?, ?, 'Technical Event', ?, ?, ?, ?, ?, '', ?)`,
		count+1, thietBi, tinhTrang, phuongAn, donVi, thoiHan, start, thietBi+"|"+tinhTrang); err != nil {
		return false
	}
	return tx.Commit() == nil
}

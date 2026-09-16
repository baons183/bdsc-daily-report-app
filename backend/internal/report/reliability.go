package report

import (
	"context"
	"database/sql"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/rsm"
	"bdscdailyreport/internal/window"
)

// FetchReliability returns dbo.Reliability rows (report section "3.3. Vấn
// đề khác liên quan tới độ tin cậy và toàn vẹn thiết bị") for the window
// [start, end] — a mix of rows a user typed in by hand and rows
// SyncReliabilityFromRSM auto-imported, indistinguishable once saved.
// Newest-added row first (ORDER BY Id DESC) — TT is then renumbered
// in-memory (1..N in that display order), same as FetchSuCo/FetchGiamSat/
// FetchCongViec; the stored TT value in the DB is untouched.
func FetchReliability(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.Reliability, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(VanDe,''), ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.Reliability WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY Id DESC`, window.SQL(start), window.SQL(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Reliability{}
	for rows.Next() {
		var r models.Reliability
		if err := rows.Scan(&r.Id, &r.TT, &r.ThietBi, &r.VanDe, &r.PhuongAn, &r.DonVi, &r.ThoiHan, &r.NgayBaoCao, &r.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	for i := range out {
		out[i].TT = i + 1
	}
	return out, nil
}

// SyncReliabilityFromRSM auto-imports reportDate's RSM PDF issues (section
// IV.2, filtered to Ban BDSC's own departments — see internal/rsm) into
// dbo.Reliability for the window [start, end], skipping any (ThietBi,
// VanDe) pair already present in that window so calling this repeatedly
// (every report generation, every time a user opens the Reliability tab)
// never creates duplicates. An auto-imported row's CreatedBy is blank;
// once saved it's indistinguishable from, and just as editable/deletable
// as, a row a user typed in by hand — there is no separate "exclude from
// report" step anymore, deleting the row IS how you leave it out.
//
// Best-effort: any error here is swallowed, since this convenience sync
// should never block viewing or generating the rest of the report.
func SyncReliabilityFromRSM(db *sql.DB, folder string, reportDate, start, end time.Time) {
	path, found := rsm.FindPDF(folder, reportDate)
	if !found {
		return
	}
	pdfRows, err := rsm.ParsePDF(path)
	if err != nil {
		return
	}
	filtered := rsm.FilterByDepartment(pdfRows, rsm.DepartmentKeywords)
	if len(filtered) == 0 {
		return
	}

	existing := map[string]bool{}
	rows, err := db.Query(`SELECT ThietBi, ISNULL(VanDe,'') FROM dbo.Reliability WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?`,
		window.SQL(start), window.SQL(end))
	if err != nil {
		return
	}
	for rows.Next() {
		var tb, vd string
		if rows.Scan(&tb, &vd) == nil {
			existing[tb+"|"+vd] = true
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
		if insertReliabilityRow(db, r.TagName, r.Issue, r.MitigationPlan, donVi, thoiHan, start, end) {
			existing[key] = true // in case the PDF itself lists the same issue twice
		}
	}
}

// insertReliabilityRow appends one auto-imported row at the next TT in
// [start, end]'s window, the same UPDLOCK/HOLDLOCK-serialized numbering
// every manual Create handler uses (see handlers.nextTTLocked) so an
// auto-import running concurrently with a manual save can never collide
// on the same TT.
func insertReliabilityRow(db *sql.DB, thietBi, vanDe, phuongAn, donVi string, thoiHan sql.NullTime, start, end time.Time) bool {
	tx, err := db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM dbo.Reliability WITH (UPDLOCK, HOLDLOCK) WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?`,
		window.SQL(start), window.SQL(end)).Scan(&count); err != nil {
		return false
	}
	if _, err := tx.Exec(`INSERT INTO dbo.Reliability (TT, ThietBi, VanDe, PhuongAn, DonVi, ThoiHan, NgayBaoCao, CreatedBy) VALUES (?, ?, ?, ?, ?, ?, ?, '')`,
		count+1, thietBi, vanDe, phuongAn, donVi, thoiHan, start); err != nil {
		return false
	}
	return tx.Commit() == nil
}

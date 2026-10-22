package report

import (
	"context"
	"database/sql"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/rsm"
	"bdscdailyreport/internal/window"
)

// FetchReliability returns EVERY dbo.Reliability row (report section "3.3.
// Vấn đề khác liên quan tới độ tin cậy và toàn vẹn thiết bị") for the
// window [start, end], included or not — a mix of rows a user typed in by
// hand and rows SyncReliabilityFromRSM auto-imported, indistinguishable
// once saved. Callers that build the actual report (PDF/email/Thống
// kê/Báo cáo tuần) must run the result through FilterReliabilityIncluded;
// the Reliability tab itself wants every row (so a user can toggle an
// excluded one back on), so it calls this directly.
// Newest-added row first (ORDER BY Id DESC) — TT is then renumbered
// in-memory (1..N in that display order), same as FetchSuCo/FetchGiamSat/
// FetchCongViec; the stored TT value in the DB is untouched.
func FetchReliability(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.Reliability, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(VanDe,''), ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, NgayBaoCao, ISNULL(CreatedBy,''), Included, TrangThai
FROM dbo.Reliability WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY Id DESC`, window.SQL(start), window.SQL(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Reliability{}
	for rows.Next() {
		var r models.Reliability
		if err := rows.Scan(&r.Id, &r.TT, &r.ThietBi, &r.VanDe, &r.PhuongAn, &r.DonVi, &r.ThoiHan, &r.NgayBaoCao, &r.CreatedBy, &r.Included, &r.TrangThai); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	for i := range out {
		out[i].TT = i + 1
	}
	return out, nil
}

// FilterReliabilityIncluded drops any row a user has unticked "đưa vào báo
// cáo" for, then renumbers TT 1..N over what's left — used only when
// actually building the report (PDF/email/Thống kê/Báo cáo tuần), never
// for the Reliability tab's own listing (which wants every row, TT and
// all, so a user can still find and toggle an excluded one back on).
// Without this renumbering, TT would carry gaps wherever an excluded row
// used to sit (FetchReliability numbers 1..N over the FULL list before
// any filtering happens) — same fix CBM's own render-time (i+1) numbering
// gets "for free" by filtering before it ever assigns a number.
func FilterReliabilityIncluded(rows []models.Reliability) []models.Reliability {
	out := make([]models.Reliability, 0, len(rows))
	for _, r := range rows {
		if r.Included {
			out = append(out, r)
		}
	}
	for i := range out {
		out[i].TT = i + 1
	}
	return out
}

// CarryForwardReliability copies EVERY one of yesterday's issues — included
// or not — into today's window [start, end], from yesterday's window
// [yStart, yEnd], preserving each one's Included flag as-is. Most "Vấn đề
// kỹ thuật ảnh hưởng toàn vẹn và độ tin cậy thiết bị" issues stay open for
// several days until actually resolved, so today's list defaults to "same
// as yesterday" instead of starting empty — and an issue a user already
// unticked "đưa vào báo cáo" for stays unticked today too, instead of
// reappearing, since it's carried forward (excluded) rather than skipped:
// skipping it here would leave no row for that tag in today's window, and
// SyncReliabilityFromRSM's dedup would then treat it as "new" and re-add
// it fresh — silently undoing the exclusion. Carrying it forward with
// Included=0 keeps the tag occupied, so that dedup correctly skips it
// instead. Skips any tag already present in today's window (e.g. a manual
// entry made earlier that morning) — matched by TagKey, the same rule
// SyncReliabilityFromRSM and Báo cáo tuần use, so "C-301" today blocks
// yesterday's "C-301 RECYCLE GAS COMPRESSOR" from being carried in too.
//
// Deduped by tag ALONE, not (tag, VanDe) — BSR's own PDF export
// reworded the same ongoing issue's text slightly from one day's export to
// the next often enough that keying on the exact Vấn đề string let the
// same real issue re-insert itself as a "new" row day after day instead of
// being recognized as a continuation, snowballing one tag into a dozen
// near-duplicate rows over a couple of weeks. One tag now holds at most
// one open row at a time; if yesterday somehow still had more than one
// (leftover from before this change), only the first wins here — an
// included row over an excluded one, then the newest — and the rest
// quietly stop being carried forward, converging within a day or two.
//
// Callers should run this BEFORE SyncReliabilityFromRSM (see
// ListReliability/GatherData/GetWeeklyReport), so if today's RSM PDF
// already lists an issue for the same tag, its own dedup check (which
// looks at whatever is already in the window at the time it runs) finds
// this carried-forward row and updates it in place (see
// SyncReliabilityFromRSM) instead of inserting a second row for that tag.
//
// Best-effort: any error here is swallowed, same as SyncReliabilityFromRSM
// — this convenience should never block viewing or generating the report.
func CarryForwardReliability(db *sql.DB, yStart, yEnd, start, end time.Time) {
	yRows, err := db.Query(`SELECT ThietBi, ISNULL(VanDe,''), ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, Included, TrangThai,
       RsmTag, CAST(CASE WHEN Edited = 1 OR ISNULL(CreatedBy,'') <> '' THEN 1 ELSE 0 END AS BIT)
FROM dbo.Reliability WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY Included DESC, Id DESC`,
		window.SQL(yStart), window.SQL(yEnd))
	if err != nil {
		return
	}
	type row struct {
		thietBi, vanDe, phuongAn, donVi, trangThai string
		thoiHan                                    sql.NullTime
		included, edited                           bool
		rsmTag                                     sql.NullString
	}
	var yesterday []row
	for yRows.Next() {
		var r row
		if yRows.Scan(&r.thietBi, &r.vanDe, &r.phuongAn, &r.donVi, &r.thoiHan, &r.included, &r.trangThai, &r.rsmTag, &r.edited) == nil {
			yesterday = append(yesterday, r)
		}
	}
	yRows.Close()
	if len(yesterday) == 0 {
		return
	}

	existing := map[string]bool{}
	tRows, err := db.Query(`SELECT ThietBi, RsmTag FROM dbo.Reliability WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?`,
		window.SQL(start), window.SQL(end))
	if err != nil {
		return
	}
	for tRows.Next() {
		var tb string
		var rsmTag sql.NullString
		if tRows.Scan(&tb, &rsmTag) == nil {
			for _, k := range reliabilityKeys(tb, rsmTag) {
				existing[k] = true
			}
		}
	}
	tRows.Close()

	for _, r := range yesterday {
		keys := reliabilityKeys(r.thietBi, r.rsmTag)
		if existing[keys[0]] || (len(keys) > 1 && existing[keys[1]]) {
			continue
		}
		if insertReliabilityRow(db, r.thietBi, r.vanDe, r.phuongAn, r.donVi, r.thoiHan, r.included, r.trangThai, r.rsmTag, r.edited, start, end) {
			for _, k := range keys {
				existing[k] = true
			}
		}
	}
}

// SyncReliabilityFromRSM auto-imports reportDate's RSM PDF issues (section
// IV.2, filtered to Ban BDSC's own departments — see internal/rsm) into
// dbo.Reliability for the window [start, end], deduped by tag (TagKey,
// the same rule CarryForwardReliability and Báo cáo tuần use) ALONE rather
// than (ThietBi, VanDe) — see CarryForwardReliability's comment on why:
// BSR's own export reworded the same ongoing issue's text often enough
// between days that matching on the exact Vấn đề string let the same real
// issue pile up as a "new" row repeatedly instead of being recognized as a
// continuation. A tag already present in the window (auto-imported,
// CreatedBy == "") gets its content fields (VanDe, PhuongAn, DonVi,
// ThoiHan) refreshed in place to the PDF's latest wording, and its ThietBi
// renamed to the shorter spelling (see ShorterTag), instead of a second
// row being inserted — unless someone has edited that row by hand
// (Edited, set by handlers.UpdateReliability), in which case their wording
// stays. Included/TrangThai are left
// untouched, since those reflect a person's own decision about this tag,
// not BSR's text. A tag whose existing row was typed in by hand
// (CreatedBy != "") is left alone entirely: that row belongs to whoever
// created it, and a same-tag RSM entry must not silently overwrite their
// own wording. Calling this repeatedly (every report generation, every
// time a user opens the Reliability tab) stays idempotent either way.
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

	type existingRow struct {
		id        int64
		thietBi   string
		createdBy string
		edited    bool
	}
	existing := map[string]existingRow{} // keyed by TagKey of ThietBi and of RsmTag
	rows, err := db.Query(`SELECT Id, ThietBi, ISNULL(CreatedBy,''), Edited, RsmTag FROM dbo.Reliability WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY Included DESC, Id DESC`,
		window.SQL(start), window.SQL(end))
	if err != nil {
		return
	}
	for rows.Next() {
		var ex existingRow
		var rsmTag sql.NullString
		if rows.Scan(&ex.id, &ex.thietBi, &ex.createdBy, &ex.edited, &rsmTag) == nil {
			for _, k := range reliabilityKeys(ex.thietBi, rsmTag) {
				if _, ok := existing[k]; !ok {
					existing[k] = ex
				}
			}
		}
	}
	rows.Close()

	for _, r := range filtered {
		var thoiHan sql.NullTime
		if dt, ok := rsm.ParseDueDate(r.DueDate); ok {
			thoiHan = sql.NullTime{Time: dt, Valid: true}
		}
		donVi := rsm.KeepDepartmentTokens(r.ActionBy, rsm.DepartmentKeywords)

		key := TagKey(r.TagName)
		if ex, ok := existing[key]; ok {
			// A row someone edited by hand keeps their wording — RSM only
			// keeps refreshing rows nobody has touched.
			if ex.createdBy == "" && !ex.edited && ex.id != 0 {
				updateReliabilityRowContent(db, ex.id, ShorterTag(ex.thietBi, r.TagName), r.Issue, r.MitigationPlan, donVi, thoiHan)
			}
			continue
		}
		if insertReliabilityRow(db, r.TagName, r.Issue, r.MitigationPlan, donVi, thoiHan, true, "in_progress", sql.NullString{String: r.TagName, Valid: true}, false, start, end) {
			existing[key] = existingRow{createdBy: ""} // in case the PDF itself lists the same tag twice
		}
	}
}

// reliabilityKeys returns the TagKeys a row answers to: its current
// ThietBi, plus the RSM tag it was first imported under (RsmTag) if a user
// has since renamed it — so the PDF entry it came from still finds it.
func reliabilityKeys(thietBi string, rsmTag sql.NullString) []string {
	keys := []string{TagKey(thietBi)}
	if rsmTag.Valid {
		if k := TagKey(rsmTag.String); k != keys[0] {
			keys = append(keys, k)
		}
	}
	return keys
}

// updateReliabilityRowContent refreshes an auto-imported row's content
// fields in place (VanDe/PhuongAn/DonVi/ThoiHan) when SyncReliabilityFromRSM
// finds a same-tag entry already present for the window, and renames its
// ThietBi to the shorter of the two spellings (see ShorterTag) — leaves
// Included/TrangThai/CreatedBy untouched, since those reflect a person's
// own decision about this tag, not BSR's text.
func updateReliabilityRowContent(db *sql.DB, id int64, thietBi, vanDe, phuongAn, donVi string, thoiHan sql.NullTime) {
	db.Exec(`UPDATE dbo.Reliability SET ThietBi=?, VanDe=?, PhuongAn=?, DonVi=?, ThoiHan=? WHERE Id=?`,
		thietBi, vanDe, phuongAn, donVi, thoiHan, id)
}

// insertReliabilityRow appends one auto-imported row at the next TT in
// [start, end]'s window, the same UPDLOCK/HOLDLOCK-serialized numbering
// every manual Create handler uses (see handlers.nextTTLocked) so an
// auto-import running concurrently with a manual save can never collide
// on the same TT. included is set explicitly rather than left to the
// column's own DB default, since CarryForwardReliability needs to persist
// a previously-excluded issue's Included=0 across days (see its comment);
// rsmTag/edited likewise carry over which RSM entry the row came from and
// whether someone has edited it (see SyncReliabilityFromRSM).
func insertReliabilityRow(db *sql.DB, thietBi, vanDe, phuongAn, donVi string, thoiHan sql.NullTime, included bool, trangThai string, rsmTag sql.NullString, edited bool, start, end time.Time) bool {
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
	if _, err := tx.Exec(`INSERT INTO dbo.Reliability (TT, ThietBi, VanDe, PhuongAn, DonVi, ThoiHan, NgayBaoCao, CreatedBy, Included, TrangThai, RsmTag, Edited) VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?)`,
		count+1, thietBi, vanDe, phuongAn, donVi, thoiHan, start, included, trangThai, rsmTag, edited); err != nil {
		return false
	}
	return tx.Commit() == nil
}

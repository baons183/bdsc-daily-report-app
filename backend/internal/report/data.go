package report

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/window"
)

// FetchSuCo returns every dbo.SuCo row whose NgayBaoCao falls in
// [start, end] — exported so both report generation and the "Báo cáo tuần"
// tab can share the same query. Newest-added row first (ORDER BY Id DESC)
// — TT is then renumbered in-memory (1..N in that display order) so it
// still reads as a clean top-down sequence rather than the row's original
// creation-order stamp; the stored TT value in the DB is untouched.
func FetchSuCo(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.SuCo, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(DM,''), ISNULL(LoaiSuCo,''), ISNULL(TinhTrang,''), ThoiGianDungPX, ThoiGianDungTB,
       ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, ISNULL(DeXuat,''), ISNULL(LuuY,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.SuCo WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY Id DESC`, window.SQL(start), window.SQL(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.SuCo{}
	for rows.Next() {
		var s models.SuCo
		if err := rows.Scan(&s.Id, &s.TT, &s.ThietBi, &s.DM, &s.LoaiSuCo, &s.TinhTrang, &s.ThoiGianDungPX, &s.ThoiGianDungTB,
			&s.PhuongAn, &s.DonVi, &s.ThoiHan, &s.DeXuat, &s.LuuY, &s.NgayBaoCao, &s.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	for i := range out {
		out[i].TT = i + 1
	}
	return out, nil
}

// FetchGiamSat returns every dbo.GiamSat row whose NgayBaoCao falls in
// [start, end] — exported so both report generation and the "Báo cáo tuần"
// tab can share the same query. Newest-added row first — see FetchSuCo's
// comment on the ORDER BY Id DESC + in-memory TT renumbering.
func FetchGiamSat(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.GiamSat, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(DM,''), ISNULL(TinhTrang,''), ISNULL(RuiRo,''),
       ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, ISNULL(DeXuat,''), ISNULL(LuuY,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.GiamSat WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY Id DESC`, window.SQL(start), window.SQL(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.GiamSat{}
	for rows.Next() {
		var s models.GiamSat
		if err := rows.Scan(&s.Id, &s.TT, &s.ThietBi, &s.DM, &s.TinhTrang, &s.RuiRo,
			&s.PhuongAn, &s.DonVi, &s.ThoiHan, &s.DeXuat, &s.LuuY, &s.NgayBaoCao, &s.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	for i := range out {
		out[i].TT = i + 1
	}
	return out, nil
}

// FetchCongViec returns every dbo.CongViec row (with attachments) whose
// NgayBaoCao falls in [start, end] — exported so both report generation
// and the "Báo cáo tuần" tab can share the same query. Newest-added row
// first — see FetchSuCo's comment on the ORDER BY Id DESC + in-memory TT
// renumbering.
func FetchCongViec(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.CongViec, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(DM,''), ISNULL(NoiDung,''), ISNULL(KeHoach,''), TienDo,
       ISNULL(HomNay,''), ISNULL(NgayMai,''), ISNULL(DeXuat,''), ISNULL(LuuY,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.CongViec WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY Id DESC`, window.SQL(start), window.SQL(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CongViec{}
	for rows.Next() {
		var c models.CongViec
		if err := rows.Scan(&c.Id, &c.TT, &c.ThietBi, &c.DM, &c.NoiDung, &c.KeHoach, &c.TienDo,
			&c.HomNay, &c.NgayMai, &c.DeXuat, &c.LuuY, &c.NgayBaoCao, &c.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	for i := range out {
		out[i].TT = i + 1
	}
	for i := range out {
		aRows, err := db.QueryContext(ctx, `SELECT Id, CongViecId, FileName, FilePath, ISNULL(ContentType,'') FROM dbo.CongViecAttachments WHERE CongViecId = ?`, out[i].Id)
		if err != nil {
			continue
		}
		for aRows.Next() {
			var a models.Attachment
			if err := aRows.Scan(&a.Id, &a.CongViecId, &a.FileName, &a.FilePath, &a.ContentType); err == nil {
				out[i].Attachments = append(out[i].Attachments, a)
			}
		}
		aRows.Close()
	}
	return out, nil
}

func fetchKPI(ctx context.Context, db *sql.DB, reportDate, start, end time.Time) ([]models.KPIEntry, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, ChiTieu, ISNULL(Nhom,''), ISNULL(DVT,''), KeHoach, ThucHien, LuyKeThang, ISNULL(DanhGia,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.KPIEntries WHERE NgayBaoCao = ? ORDER BY Id`, reportDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.KPIEntry{}
	for rows.Next() {
		var k models.KPIEntry
		if err := rows.Scan(&k.Id, &k.ChiTieu, &k.Nhom, &k.DVT, &k.KeHoach, &k.ThucHien, &k.LuyKeThang, &k.DanhGia, &k.NgayBaoCao, &k.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if err := ApplyKPIAutoCounts(ctx, db, out, start, end); err != nil {
		return nil, err
	}
	return out, nil
}

// KPIAutoCountMap maps each KPI ChiTieu that's auto-counted from Sự cố
// entries (instead of typed in by hand) to the matching LoaiSuCo value.
var KPIAutoCountMap = map[string]string{
	"Sự cố":           "Accident",
	"Near Miss":       "Near Miss",
	"Technical event": "Technical Event",
}

// CountSuCoByLoai counts dbo.SuCo rows of the given LoaiSuCo within
// [start, end] — the same window the "2. Sự cố" section of the same report
// uses, so the KPI count always matches what's actually printed there.
func CountSuCoByLoai(ctx context.Context, db *sql.DB, loai string, start, end time.Time) (float64, error) {
	var n int
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM dbo.SuCo WHERE LoaiSuCo = ? AND NgayBaoCao >= ? AND NgayBaoCao <= ?`,
		loai, window.SQL(start), window.SQL(end)).Scan(&n)
	return float64(n), err
}

// KPIDanhGia is the single "Đánh giá" rule for every KPI row: đạt when
// Thực hiện is at most Kế hoạch. A never-set Kế hoạch or Thực hiện defaults
// to 0, so every row always gets a real Đánh giá even before anyone has
// typed anything for it.
func KPIDanhGia(keHoach, thucHien *float64) string {
	kh := 0.0
	if keHoach != nil {
		kh = *keHoach
	}
	th := 0.0
	if thucHien != nil {
		th = *thucHien
	}
	if th <= kh {
		return "Đạt"
	}
	return "Không đạt"
}

// SumKPIThucHienMonth sums the ThucHien already saved for chiTieu across
// every day from monthStart through end — the month-to-date "Lũy kế tháng"
// for the 2 categories that have no other data source than each day's own
// manually-entered KPIEntries row.
func SumKPIThucHienMonth(ctx context.Context, db *sql.DB, chiTieu string, monthStart, end time.Time) (float64, error) {
	var sum sql.NullFloat64
	err := db.QueryRowContext(ctx, `
SELECT SUM(ThucHien) FROM dbo.KPIEntries WHERE ChiTieu = ? AND NgayBaoCao >= ? AND NgayBaoCao <= ?`,
		chiTieu, monthStart.Format("2006-01-02"), end.Format("2006-01-02")).Scan(&sum)
	if err != nil {
		return 0, err
	}
	if !sum.Valid {
		return 0, nil
	}
	return sum.Float64, nil
}

// GetKPIBaseline returns the one-time starting offset an Admin set for
// chiTieu in yearMonth ("2006-01"), or 0 if none was ever set (which is
// the normal case for every month except the one this app went live in).
func GetKPIBaseline(ctx context.Context, db *sql.DB, chiTieu, yearMonth string) (float64, error) {
	var v float64
	err := db.QueryRowContext(ctx, `SELECT BaselineValue FROM dbo.KPIBaseline WHERE ChiTieu = ? AND YearMonth = ?`,
		chiTieu, yearMonth).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return v, err
}

// ApplyKPIAutoCounts overrides ThucHien and LuyKeThang for every row with a
// live figure instead of whatever was last saved, then (re)computes
// DanhGia via KPIDanhGia:
//   - The 3 Sự cố-based categories (KPIAutoCountMap) count dbo.SuCo directly
//     — ThucHien over [start, end] (today's report window), LuyKeThang over
//     the calendar month through end.
//   - The other 2 categories have no such source, so LuyKeThang instead
//     sums each day's own saved ThucHien for the month (ThucHien itself
//     stays whatever was typed in and saved for that one category).
//   - Either way, LuyKeThang also adds GetKPIBaseline's one-time offset for
//     the current month, so going live mid-month doesn't understate it.
func ApplyKPIAutoCounts(ctx context.Context, db *sql.DB, rows []models.KPIEntry, start, end time.Time) error {
	monthStart := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())
	yearMonth := end.Format("2006-01")
	for i := range rows {
		chiTieu := rows[i].ChiTieu
		var lk float64
		if loai, ok := KPIAutoCountMap[chiTieu]; ok {
			n, err := CountSuCoByLoai(ctx, db, loai, start, end)
			if err != nil {
				return err
			}
			rows[i].ThucHien = &n
			lk, err = CountSuCoByLoai(ctx, db, loai, monthStart, end)
			if err != nil {
				return err
			}
		} else {
			var err error
			lk, err = SumKPIThucHienMonth(ctx, db, chiTieu, monthStart, end)
			if err != nil {
				return err
			}
		}
		base, err := GetKPIBaseline(ctx, db, chiTieu, yearMonth)
		if err != nil {
			return err
		}
		lk += base
		rows[i].LuyKeThang = &lk
		rows[i].DanhGia = KPIDanhGia(rows[i].KeHoach, rows[i].ThucHien)
	}
	return nil
}

// FetchWeeklyKPI aggregates each of the 5 fixed KPI categories over an
// arbitrary [start, end] range for the "Báo cáo tuần" tab. Kế hoạch and
// Thực hiện are SUMMED across the whole range (each day's figure added
// up); Lũy kế tháng is deliberately NOT summed — it's already a running
// total, so summing it across days would multiply-count everything before
// the range. Instead it's read as of `end` (the last day), same as a
// single daily report would show, plus GetKPIBaseline's offset.
func FetchWeeklyKPI(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.KPIEntry, error) {
	monthStart := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())
	yearMonth := end.Format("2006-01")

	rangeThucHien := func(chiTieu string, s, e time.Time) (float64, error) {
		if loai, ok := KPIAutoCountMap[chiTieu]; ok {
			return CountSuCoByLoai(ctx, db, loai, s, e)
		}
		return SumKPIThucHienMonth(ctx, db, chiTieu, s, e)
	}

	out := make([]models.KPIEntry, 0, len(kpiCategories))
	for _, cat := range kpiCategories {
		var khVal sql.NullFloat64
		err := db.QueryRowContext(ctx, `
		SELECT TOP 1 KeHoach FROM dbo.KPIEntries
		WHERE ChiTieu = ? AND NgayBaoCao >= ? AND NgayBaoCao <= ?
		ORDER BY NgayBaoCao DESC`,
			cat.ChiTieu, start.Format("2006-01-02"), end.Format("2006-01-02")).Scan(&khVal)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		keHoach := 0.0
		if khVal.Valid {
			keHoach = khVal.Float64
		}

		thucHien, err := rangeThucHien(cat.ChiTieu, start, end)
		if err != nil {
			return nil, err
		}

		luyKe, err := rangeThucHien(cat.ChiTieu, monthStart, end)
		if err != nil {
			return nil, err
		}
		base, err := GetKPIBaseline(ctx, db, cat.ChiTieu, yearMonth)
		if err != nil {
			return nil, err
		}
		luyKe += base

		out = append(out, models.KPIEntry{
			ChiTieu:    cat.ChiTieu,
			Nhom:       cat.Nhom,
			DVT:        cat.DVT,
			KeHoach:    &keHoach,
			ThucHien:   &thucHien,
			LuyKeThang: &luyKe,
			DanhGia:    KPIDanhGia(&keHoach, &thucHien),
		})
	}
	return out, nil
}

// NoteItem is one row of the consolidated Đề xuất/Lưu ý tables — same shape
// used by both the PDF and the web "Thống kê" view.
type NoteItem struct {
	TT      int    `json:"tt"`
	ThietBi string `json:"thietBi"`
	Note    string `json:"note"`
}

// CombineDeXuat and CombineLuuY pool the Đề xuất/Lưu ý free-text fields from
// all three data-entry tables into one numbered list, skipping "nothing to
// say" values (see excludedNote) — the exact same rule the PDF applies.
func CombineDeXuat(suco []models.SuCo, giamsat []models.GiamSat, congviec []models.CongViec) []NoteItem {
	out := []NoteItem{}
	n := 1
	for _, s := range suco {
		if !excludedNote(s.DeXuat) {
			out = append(out, NoteItem{TT: n, ThietBi: s.ThietBi, Note: s.DeXuat})
			n++
		}
	}
	for _, s := range giamsat {
		if !excludedNote(s.DeXuat) {
			out = append(out, NoteItem{TT: n, ThietBi: s.ThietBi, Note: s.DeXuat})
			n++
		}
	}
	for _, c := range congviec {
		if !excludedNote(c.DeXuat) {
			out = append(out, NoteItem{TT: n, ThietBi: c.ThietBi, Note: c.DeXuat})
			n++
		}
	}
	return out
}

func CombineLuuY(suco []models.SuCo, giamsat []models.GiamSat, congviec []models.CongViec) []NoteItem {
	out := []NoteItem{}
	n := 1
	for _, s := range suco {
		if !excludedNote(s.LuuY) {
			out = append(out, NoteItem{TT: n, ThietBi: s.ThietBi, Note: s.LuuY})
			n++
		}
	}
	for _, s := range giamsat {
		if !excludedNote(s.LuuY) {
			out = append(out, NoteItem{TT: n, ThietBi: s.ThietBi, Note: s.LuuY})
			n++
		}
	}
	for _, c := range congviec {
		if !excludedNote(c.LuuY) {
			out = append(out, NoteItem{TT: n, ThietBi: c.ThietBi, Note: c.LuuY})
			n++
		}
	}
	return out
}

// excludedNote reports whether a Đề xuất/Lưu ý free-text value should be
// treated as "nothing to say" (N/A, KHÔNG, etc.) and therefore left out of
// the consolidated Đề xuất/Lưu ý tables.
func excludedNote(s string) bool {
	v := strings.ToUpper(strings.TrimSpace(s))
	if v == "" {
		return true
	}
	switch v {
	case "N/A", "NA", "KHÔNG", "KHONG", "KHÔNG CÓ", "KHONG CO", "K CÓ", "K CO", "KO":
		return true
	}
	return false
}

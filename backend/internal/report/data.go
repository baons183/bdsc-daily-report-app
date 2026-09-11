package report

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/window"
)

func fetchSuCo(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.SuCo, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(DM,''), ISNULL(TinhTrang,''), ThoiGianDungPX, ThoiGianDungTB,
       ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, ISNULL(DeXuat,''), ISNULL(LuuY,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.SuCo WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY TT`, window.SQL(start), window.SQL(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.SuCo{}
	for rows.Next() {
		var s models.SuCo
		if err := rows.Scan(&s.Id, &s.TT, &s.ThietBi, &s.DM, &s.TinhTrang, &s.ThoiGianDungPX, &s.ThoiGianDungTB,
			&s.PhuongAn, &s.DonVi, &s.ThoiHan, &s.DeXuat, &s.LuuY, &s.NgayBaoCao, &s.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func fetchGiamSat(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.GiamSat, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(DM,''), ISNULL(TinhTrang,''), ISNULL(RuiRo,''),
       ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, ISNULL(DeXuat,''), ISNULL(LuuY,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.GiamSat WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY TT`, window.SQL(start), window.SQL(end))
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
	return out, nil
}

func fetchCongViec(ctx context.Context, db *sql.DB, start, end time.Time) ([]models.CongViec, error) {
	rows, err := db.QueryContext(ctx, `
SELECT Id, TT, ThietBi, ISNULL(DM,''), ISNULL(NoiDung,''), ISNULL(KeHoach,''), TienDo,
       ISNULL(HomNay,''), ISNULL(NgayMai,''), ISNULL(DeXuat,''), ISNULL(LuuY,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.CongViec WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY TT`, window.SQL(start), window.SQL(end))
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

func fetchKPI(ctx context.Context, db *sql.DB, reportDate time.Time) ([]models.KPIEntry, error) {
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

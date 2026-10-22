package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

// ListSuCo syncs this window's RSM PDF section IV.3 (report.
// SyncSuCoFromRSM) before reading — see ListReliability's comment for why
// window.ReportDateForEntry is the right report date to sync against here
// (the file for report date D lands ~06:30, still inside D's still-open
// window, so opening this tab any time after that picks it up, editable/
// deletable right here like any manually typed row).
//
// With "?start=&end=YYYY-MM-DD" it instead lists every row whose printed
// report date (window.ReportDateForEntry — the date in the report PDF's
// own name) falls in that range, inclusive, so older rows can be found and
// edited from this tab. No RSM sync in that mode, since the sync only ever
// targets the current entry window anyway.
func (e *Env) ListSuCo(w http.ResponseWriter, r *http.Request) {
	rh, rm := db.ReportTime(e.DB, 8, 0)
	var start, end time.Time
	if startStr, endStr := r.URL.Query().Get("start"), r.URL.Query().Get("end"); startStr != "" || endStr != "" {
		startD, err1 := time.ParseInLocation("2006-01-02", startStr, time.Local)
		endD, err2 := time.ParseInLocation("2006-01-02", endStr, time.Local)
		if err1 != nil || err2 != nil {
			writeErr(w, 400, "start/end khong hop le, can dang YYYY-MM-DD")
			return
		}
		if endD.Before(startD) {
			writeErr(w, 400, "ngay ket thuc phai sau ngay bat dau")
			return
		}
		// Report date D's window is the one the D+1 report run closes.
		start, _ = window.ReportWindow(reportRunAt(startD, rh, rm), rh, rm)
		_, end = window.ReportWindow(reportRunAt(endD, rh, rm), rh, rm)
	} else {
		now := time.Now()
		start, end = window.EntryWindow(now, rh, rm)
		reportDate := window.ReportDateForEntry(now, rh, rm)
		report.SyncSuCoFromRSM(e.DB, e.effectiveRSMPdfFolder(), reportDate, start, end)
	}

	out, err := report.FetchSuCo(r.Context(), e.DB, start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	for i := range out {
		out[i].ReportDate = window.ReportDateForEntry(out[i].NgayBaoCao, rh, rm).Format("2006-01-02")
	}
	writeJSON(w, 200, out)
}

// reportRunAt is when the report for report date d is generated: d+1 at
// the configured report time.
func reportRunAt(d time.Time, rh, rm int) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), rh, rm, 0, 0, time.Local).AddDate(0, 0, 1)
}

func (e *Env) CreateSuCo(w http.ResponseWriter, r *http.Request) {
	var in models.SuCo
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.ThietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
		return
	}
	if !validLoaiSuCo(in.LoaiSuCo) {
		writeErr(w, 400, "thieu hoac sai Loai su co (Accident/Near Miss/Technical Event)")
		return
	}
	now := time.Now()
	if !checkNotLocked(w, r, e.DB, now) {
		return
	}
	rh, rm := db.ReportTime(e.DB, 8, 0)
	start, end := window.EntryWindow(now, rh, rm)

	tx, err := e.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()

	tt, err := nextTTLocked(r.Context(), tx, "SuCo", start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	user := currentUser(r)
	createdBy := ""
	if user != nil {
		createdBy = user.EmployeeCode
	}
	in.TrangThai = normalizeTrangThai(in.TrangThai)
	var newID int64
	err = tx.QueryRowContext(r.Context(), `
INSERT INTO dbo.SuCo (TT, ThietBi, DM, LoaiSuCo, TinhTrang, ThoiGianDungPX, ThoiGianDungTB, PhuongAn, DonVi, ThoiHan, DeXuat, LuuY, NgayBaoCao, CreatedBy, TrangThai)
OUTPUT CAST(INSERTED.Id AS BIGINT)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tt, in.ThietBi, in.DM, in.LoaiSuCo, in.TinhTrang, in.ThoiGianDungPX, in.ThoiGianDungTB, in.PhuongAn, in.DonVi, in.ThoiHan, in.DeXuat, in.LuuY, now, createdBy, in.TrangThai).Scan(&newID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	in.Id = int(newID)
	in.TT = tt
	in.NgayBaoCao = now
	in.CreatedBy = createdBy
	writeJSON(w, 201, in)
}

func (e *Env) DeleteSuCo(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var ngayBaoCao time.Time
	var createdBy string
	var rsmKey sql.NullString
	if err := e.DB.QueryRowContext(r.Context(), `SELECT NgayBaoCao, ISNULL(CreatedBy,''), RsmKey FROM dbo.SuCo WHERE Id = ?`, id).Scan(&ngayBaoCao, &createdBy, &rsmKey); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !checkOwnership(w, r, createdBy) {
		return
	}
	if !checkNotLocked(w, r, e.DB, ngayBaoCao) {
		return
	}
	tx, err := e.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	// An RSM-imported row is remembered as dismissed, otherwise the next
	// SyncSuCoFromRSM would import the same PDF entry right back.
	if rsmKey.Valid {
		dismissedBy := ""
		if user := currentUser(r); user != nil {
			dismissedBy = user.EmployeeCode
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO dbo.SuCoRsmDismissed (RsmKey, NgayBaoCao, DismissedBy) VALUES (?, ?, ?)`,
			rsmKey.String, window.SQL(ngayBaoCao), dismissedBy); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM dbo.SuCo WHERE Id = ?`, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	rh, rm := db.ReportTime(e.DB, 8, 0)
	start, end := window.EntryWindow(ngayBaoCao, rh, rm)
	if err := renumber(r.Context(), e.DB, "SuCo", start, end); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (e *Env) UpdateSuCo(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var createdBy string
	var ngayBaoCao time.Time
	if err := e.DB.QueryRowContext(r.Context(), `SELECT ISNULL(CreatedBy,''), NgayBaoCao FROM dbo.SuCo WHERE Id = ?`, id).Scan(&createdBy, &ngayBaoCao); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !checkEditOwnership(w, r, createdBy) {
		return
	}
	if !checkNotLocked(w, r, e.DB, ngayBaoCao) {
		return
	}
	var in models.SuCo
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.ThietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
		return
	}
	if !validLoaiSuCo(in.LoaiSuCo) {
		writeErr(w, 400, "thieu hoac sai Loai su co (Accident/Near Miss/Technical Event)")
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `
UPDATE dbo.SuCo SET ThietBi=?, DM=?, LoaiSuCo=?, TinhTrang=?, ThoiGianDungPX=?, ThoiGianDungTB=?, PhuongAn=?, DonVi=?, ThoiHan=?, DeXuat=?, LuuY=?, TrangThai=?
WHERE Id=?`,
		in.ThietBi, in.DM, in.LoaiSuCo, in.TinhTrang, in.ThoiGianDungPX, in.ThoiGianDungTB, in.PhuongAn, in.DonVi, in.ThoiHan, in.DeXuat, in.LuuY, normalizeTrangThai(in.TrangThai), id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type setSuCoTrangThaiReq struct {
	TrangThai string `json:"trangThai"`
}

// SetSuCoTrangThai changes "trạng thái thực hiện" for one row inline from
// the Sự cố table's own dropdown, without going through the full Sửa form
// — same permission and lock checks as UpdateSuCo.
func (e *Env) SetSuCoTrangThai(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var createdBy string
	var ngayBaoCao time.Time
	if err := e.DB.QueryRowContext(r.Context(), `SELECT ISNULL(CreatedBy,''), NgayBaoCao FROM dbo.SuCo WHERE Id = ?`, id).Scan(&createdBy, &ngayBaoCao); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !checkEditOwnership(w, r, createdBy) {
		return
	}
	if !checkNotLocked(w, r, e.DB, ngayBaoCao) {
		return
	}
	var in setSuCoTrangThaiReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `UPDATE dbo.SuCo SET TrangThai = ? WHERE Id = ?`, normalizeTrangThai(in.TrangThai), id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// validLoaiSuCo checks that a Sự cố row is classified as one of the 3 fixed
// incident types — required because the KPI tab auto-counts these rows by
// type (see report.KPIAutoCountMap), so an unclassified row would silently
// never be counted anywhere.
func validLoaiSuCo(v string) bool {
	switch v {
	case "Accident", "Near Miss", "Technical Event":
		return true
	default:
		return false
	}
}

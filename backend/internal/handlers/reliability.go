package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"bdscdailyreport/internal/auth"
	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

// canTouchReliability allows editing/deleting a row if the caller is
// Admin, Approver (treated as equal to Admin everywhere except the Quản
// trị tab/APIs — see auth.HasRole), the row's own creator, OR the row has
// no personal creator at all (CreatedBy == "" — auto-imported from the RSM
// PDF, see report.SyncReliabilityFromRSM) — an auto-imported row belongs to
// the team, not to one person, so anyone holding the role that can see
// this tab may edit or delete it, same as they could a colleague's
// manually typed row via checkEditOwnership elsewhere.
func canTouchReliability(r *http.Request, createdBy string) bool {
	claims := currentUser(r)
	if claims == nil {
		return false
	}
	return auth.HasRole(claims.Roles, "Admin") || auth.HasRole(claims.Roles, "Approver") || createdBy == "" || claims.EmployeeCode == createdBy
}

// ListReliability defaults to the exact same "today's currently open
// window" concept as ListSuCo/ListGiamSat/ListCongViec (window.EntryWindow)
// — consistent with every other data-entry tab: a row typed in right now
// belongs to the SAME window this reads back. Before reading, it also (1)
// carries forward yesterday's still-included issues into today's window
// (report.CarryForwardReliability) — most of these issues stay open for
// days, so today's list defaults to "same as yesterday" instead of empty
// — and (2) syncs today's window's RSM PDF (report.SyncReliabilityFromRSM,
// which skips anything CarryForwardReliability already added), using
// window.ReportDateForEntry — the exact report date THIS window prints
// under — so an issue shows up here the moment its file exists, not just
// once the actual PDF/email generates: the file for report date D is
// produced overnight and lands ~06:30, still inside D's still-open window
// (which only closes at the next cutoff), so opening this tab any time
// from then until the window closes picks it up, editable/deletable right
// here like any manually typed row.
//
// An optional "?date=YYYY-MM-DD" switches to browsing a specific (usually
// past) window instead — same date->window convention as ListCBM (the
// chosen date is the window's closing cutoff), so a past window still
// syncs/shows its own RSM PDF and can be edited/deleted (via
// checkNotLocked) as long as that report date hasn't been Kiểm tra/Phê
// duyệt'd yet.
func (e *Env) ListReliability(w http.ResponseWriter, r *http.Request) {
	rh, rm := db.ReportTime(e.DB, 8, 0)
	var start, end, reportDate time.Time
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			writeErr(w, 400, "ngay khong hop le, can dang YYYY-MM-DD")
			return
		}
		runAt := time.Date(d.Year(), d.Month(), d.Day(), rh, rm, 0, 0, time.Local)
		start, end = window.ReportWindow(runAt, rh, rm)
		reportDate = runAt.AddDate(0, 0, -1)
	} else {
		now := time.Now()
		start, end = window.EntryWindow(now, rh, rm)
		reportDate = window.ReportDateForEntry(now, rh, rm)
	}
	yStart, yEnd := window.ReportWindow(reportDate, rh, rm)
	report.CarryForwardReliability(e.DB, yStart, yEnd, start, end)
	report.SyncReliabilityFromRSM(e.DB, e.effectiveRSMPdfFolder(), reportDate, start, end)

	out, err := report.FetchReliability(r.Context(), e.DB, start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, out)
}

func (e *Env) CreateReliability(w http.ResponseWriter, r *http.Request) {
	var in models.Reliability
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.ThietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
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

	tt, err := nextTTLocked(r.Context(), tx, "Reliability", start, end)
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
INSERT INTO dbo.Reliability (TT, ThietBi, VanDe, PhuongAn, DonVi, ThoiHan, NgayBaoCao, CreatedBy, TrangThai)
OUTPUT CAST(INSERTED.Id AS BIGINT)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tt, in.ThietBi, in.VanDe, in.PhuongAn, in.DonVi, in.ThoiHan, now, createdBy, in.TrangThai).Scan(&newID)
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

func (e *Env) DeleteReliability(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var ngayBaoCao time.Time
	var createdBy string
	if err := e.DB.QueryRowContext(r.Context(), `SELECT NgayBaoCao, ISNULL(CreatedBy,'') FROM dbo.Reliability WHERE Id = ?`, id).Scan(&ngayBaoCao, &createdBy); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !canTouchReliability(r, createdBy) {
		writeErr(w, 403, "ban khong co quyen xoa dong nay")
		return
	}
	if !checkNotLocked(w, r, e.DB, ngayBaoCao) {
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `DELETE FROM dbo.Reliability WHERE Id = ?`, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	rh, rm := db.ReportTime(e.DB, 8, 0)
	start, end := window.EntryWindow(ngayBaoCao, rh, rm)
	if err := renumber(r.Context(), e.DB, "Reliability", start, end); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type setReliabilityIncludeReq struct {
	Included bool `json:"included"`
}

// SetReliabilityInclude ticks/unticks "đưa vào báo cáo" for one row — the
// only real way to leave an auto-imported (RSM PDF) row out of the report,
// since deleting it would just get it reinserted on the next sync (its
// (ThietBi, VanDe) key would no longer match anything, see
// report.SyncReliabilityFromRSM). Same permission model as edit/delete
// (canTouchReliability); no checkNotLocked here, matching CBM's own
// SetCBMExclusion — the same lightweight, always-adjustable mechanism.
func (e *Env) SetReliabilityInclude(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var createdBy string
	if err := e.DB.QueryRowContext(r.Context(), `SELECT ISNULL(CreatedBy,'') FROM dbo.Reliability WHERE Id = ?`, id).Scan(&createdBy); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !canTouchReliability(r, createdBy) {
		writeErr(w, 403, "ban khong co quyen doi trang thai dong nay")
		return
	}
	var in setReliabilityIncludeReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `UPDATE dbo.Reliability SET Included = ? WHERE Id = ?`, in.Included, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (e *Env) UpdateReliability(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var createdBy string
	var ngayBaoCao time.Time
	if err := e.DB.QueryRowContext(r.Context(), `SELECT ISNULL(CreatedBy,''), NgayBaoCao FROM dbo.Reliability WHERE Id = ?`, id).Scan(&createdBy, &ngayBaoCao); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !canTouchReliability(r, createdBy) {
		writeErr(w, 403, "ban khong co quyen sua dong nay")
		return
	}
	if !checkNotLocked(w, r, e.DB, ngayBaoCao) {
		return
	}
	var in models.Reliability
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.ThietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `
UPDATE dbo.Reliability SET ThietBi=?, VanDe=?, PhuongAn=?, DonVi=?, ThoiHan=?, TrangThai=?
WHERE Id=?`,
		in.ThietBi, in.VanDe, in.PhuongAn, in.DonVi, in.ThoiHan, normalizeTrangThai(in.TrangThai), id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

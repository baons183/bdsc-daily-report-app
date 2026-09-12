package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

// kpiReportDate returns the calendar date KPI entries made "now" should be
// filed under — the same report window SuCo/GiamSat/CongViec use (8:01
// today -> 7:59 tomorrow), so a KPI update after the 8:01 cutoff rolls into
// tomorrow's report exactly like the other sections do, instead of always
// filing under the literal wall-clock date.
func kpiReportDate(now time.Time) string {
	_, end := window.EntryWindow(now)
	return end.Format("2006-01-02")
}

func (e *Env) ListKPI(w http.ResponseWriter, r *http.Request) {
	today := kpiReportDate(time.Now())
	rows, err := e.DB.QueryContext(r.Context(), `
SELECT Id, ChiTieu, ISNULL(Nhom,''), ISNULL(DVT,''), KeHoach, ThucHien, LuyKeThang, ISNULL(DanhGia,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.KPIEntries WHERE NgayBaoCao = ? ORDER BY Id`, today)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []models.KPIEntry{}
	for rows.Next() {
		var k models.KPIEntry
		if err := rows.Scan(&k.Id, &k.ChiTieu, &k.Nhom, &k.DVT, &k.KeHoach, &k.ThucHien, &k.LuyKeThang, &k.DanhGia, &k.NgayBaoCao, &k.CreatedBy); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, k)
	}
	start, end := window.EntryWindow(time.Now())
	if err := report.ApplyKPIAutoCounts(r.Context(), e.DB, out, start, end); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, out)
}

// SaveKPI replaces today's KPI rows with whatever the client sends (normally
// the 5 fixed categories: Sự cố / Near Miss / Technical event / Rework /
// Dừng ngoài kế hoạch).
func (e *Env) SaveKPI(w http.ResponseWriter, r *http.Request) {
	var in []models.KPIEntry
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	today := kpiReportDate(time.Now())
	user := currentUser(r)
	createdBy := ""
	if user != nil {
		createdBy = user.EmployeeCode
	}

	tx, err := e.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(), `DELETE FROM dbo.KPIEntries WHERE NgayBaoCao = ?`, today); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	for _, k := range in {
		if _, err := tx.ExecContext(r.Context(), `
INSERT INTO dbo.KPIEntries (ChiTieu, Nhom, DVT, KeHoach, ThucHien, LuyKeThang, DanhGia, NgayBaoCao, CreatedBy)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			k.ChiTieu, k.Nhom, k.DVT, k.KeHoach, k.ThucHien, k.LuyKeThang, k.DanhGia, today, createdBy); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

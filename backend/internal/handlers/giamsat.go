package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/window"
)

func (e *Env) ListGiamSat(w http.ResponseWriter, r *http.Request) {
	start, end := window.EntryWindow(time.Now())
	rows, err := e.DB.QueryContext(r.Context(), `
SELECT Id, TT, ThietBi, ISNULL(DM,''), ISNULL(TinhTrang,''), ISNULL(RuiRo,''),
       ISNULL(PhuongAn,''), ISNULL(DonVi,''), ThoiHan, ISNULL(DeXuat,''), ISNULL(LuuY,''), NgayBaoCao, ISNULL(CreatedBy,'')
FROM dbo.GiamSat WHERE NgayBaoCao >= ? AND NgayBaoCao <= ? ORDER BY TT`, window.SQL(start), window.SQL(end))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []models.GiamSat{}
	for rows.Next() {
		var s models.GiamSat
		if err := rows.Scan(&s.Id, &s.TT, &s.ThietBi, &s.DM, &s.TinhTrang, &s.RuiRo,
			&s.PhuongAn, &s.DonVi, &s.ThoiHan, &s.DeXuat, &s.LuuY, &s.NgayBaoCao, &s.CreatedBy); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, s)
	}
	writeJSON(w, 200, out)
}

func (e *Env) CreateGiamSat(w http.ResponseWriter, r *http.Request) {
	var in models.GiamSat
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.ThietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
		return
	}
	now := time.Now()
	start, end := window.EntryWindow(now)

	tx, err := e.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()

	tt, err := nextTTLocked(r.Context(), tx, "GiamSat", start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	user := currentUser(r)
	createdBy := ""
	if user != nil {
		createdBy = user.EmployeeCode
	}
	var newID int64
	err = tx.QueryRowContext(r.Context(), `
INSERT INTO dbo.GiamSat (TT, ThietBi, DM, TinhTrang, RuiRo, PhuongAn, DonVi, ThoiHan, DeXuat, LuuY, NgayBaoCao, CreatedBy)
OUTPUT CAST(INSERTED.Id AS BIGINT)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tt, in.ThietBi, in.DM, in.TinhTrang, in.RuiRo, in.PhuongAn, in.DonVi, in.ThoiHan, in.DeXuat, in.LuuY, now, createdBy).Scan(&newID)
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

func (e *Env) DeleteGiamSat(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var ngayBaoCao time.Time
	var createdBy string
	if err := e.DB.QueryRowContext(r.Context(), `SELECT NgayBaoCao, ISNULL(CreatedBy,'') FROM dbo.GiamSat WHERE Id = ?`, id).Scan(&ngayBaoCao, &createdBy); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !checkOwnership(w, r, createdBy) {
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `DELETE FROM dbo.GiamSat WHERE Id = ?`, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	start, end := window.EntryWindow(ngayBaoCao)
	if err := renumber(r.Context(), e.DB, "GiamSat", start, end); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (e *Env) UpdateGiamSat(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var createdBy string
	if err := e.DB.QueryRowContext(r.Context(), `SELECT ISNULL(CreatedBy,'') FROM dbo.GiamSat WHERE Id = ?`, id).Scan(&createdBy); err != nil {
		writeErr(w, 404, "khong tim thay dong nay")
		return
	}
	if !checkOwnership(w, r, createdBy) {
		return
	}
	var in models.GiamSat
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.ThietBi == "" {
		writeErr(w, 400, "thieu Thiet bi")
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `
UPDATE dbo.GiamSat SET ThietBi=?, DM=?, TinhTrang=?, RuiRo=?, PhuongAn=?, DonVi=?, ThoiHan=?, DeXuat=?, LuuY=?
WHERE Id=?`,
		in.ThietBi, in.DM, in.TinhTrang, in.RuiRo, in.PhuongAn, in.DonVi, in.ThoiHan, in.DeXuat, in.LuuY, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

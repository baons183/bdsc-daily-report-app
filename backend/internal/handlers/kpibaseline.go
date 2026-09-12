package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// kpiChiTieuList mirrors the 5 fixed rows the KPI tab always shows
// (web/app.js's KPI_CATEGORIES) — the baseline editor offers exactly these,
// not whatever ad-hoc ChiTieu strings might exist in old data.
var kpiChiTieuList = []string{"Sự cố", "Near Miss", "Technical event", "Rework", "Dừng ngoài kế hoạch"}

type kpiBaselineRow struct {
	ChiTieu       string  `json:"chiTieu"`
	YearMonth     string  `json:"yearMonth"`
	BaselineValue float64 `json:"baselineValue"`
}

// ListKPIBaseline returns the current month's baseline for all 5 KPI
// categories (0 for any never set) — the one-time starting offset added
// into "Lũy kế tháng" to account for this app going live mid-month.
func (e *Env) ListKPIBaseline(w http.ResponseWriter, r *http.Request) {
	yearMonth := time.Now().Format("2006-01")
	rows, err := e.DB.QueryContext(r.Context(),
		`SELECT ChiTieu, BaselineValue FROM dbo.KPIBaseline WHERE YearMonth = ?`, yearMonth)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	values := map[string]float64{}
	for rows.Next() {
		var chiTieu string
		var v float64
		if err := rows.Scan(&chiTieu, &v); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		values[chiTieu] = v
	}
	out := make([]kpiBaselineRow, len(kpiChiTieuList))
	for i, c := range kpiChiTieuList {
		out[i] = kpiBaselineRow{ChiTieu: c, YearMonth: yearMonth, BaselineValue: values[c]}
	}
	writeJSON(w, 200, out)
}

type setBaselineReq struct {
	ChiTieu       string  `json:"chiTieu"`
	BaselineValue float64 `json:"baselineValue"`
}

// SetKPIBaseline upserts the current month's baseline for one category.
func (e *Env) SetKPIBaseline(w http.ResponseWriter, r *http.Request) {
	var in setBaselineReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	valid := false
	for _, c := range kpiChiTieuList {
		if c == in.ChiTieu {
			valid = true
			break
		}
	}
	if !valid {
		writeErr(w, 400, "chi tieu khong hop le")
		return
	}
	user := currentUser(r)
	createdBy := ""
	if user != nil {
		createdBy = user.EmployeeCode
	}
	yearMonth := time.Now().Format("2006-01")

	tx, err := e.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM dbo.KPIBaseline WHERE ChiTieu = ? AND YearMonth = ?`, in.ChiTieu, yearMonth); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
INSERT INTO dbo.KPIBaseline (ChiTieu, YearMonth, BaselineValue, CreatedBy) VALUES (?, ?, ?, ?)`,
		in.ChiTieu, yearMonth, in.BaselineValue, createdBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

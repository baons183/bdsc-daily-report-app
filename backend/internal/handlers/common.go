package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bdscdailyreport/internal/auth"
	"bdscdailyreport/internal/window"
)

type Env struct {
	DB           *sql.DB
	UploadsDir   string
	AssetDBName  string
	CBMExcelPath string
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// nextTTLocked returns the next TT (1-based sequence) for rows of `table`
// falling inside [start, end]. Must run inside a transaction: the UPDLOCK +
// HOLDLOCK hints serialize concurrent callers against the same window so two
// users saving at the same time can never be handed the same TT.
func nextTTLocked(ctx context.Context, tx *sql.Tx, table string, start, end time.Time) (int, error) {
	q := fmt.Sprintf(`SELECT COUNT(*) FROM dbo.%s WITH (UPDLOCK, HOLDLOCK) WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?`, table)
	var count int
	if err := tx.QueryRowContext(ctx, q, window.SQL(start), window.SQL(end)).Scan(&count); err != nil {
		return 0, err
	}
	return count + 1, nil
}

// renumber closes gaps left by a delete: every row of `table` inside the
// window is renumbered 1..N in Id order.
func renumber(ctx context.Context, db *sql.DB, table string, start, end time.Time) error {
	q := fmt.Sprintf(`
;WITH ordered AS (
  SELECT Id, ROW_NUMBER() OVER (ORDER BY Id) AS rn
  FROM dbo.%s WHERE NgayBaoCao >= ? AND NgayBaoCao <= ?
)
UPDATE t SET TT = o.rn
FROM dbo.%s t JOIN ordered o ON t.Id = o.Id`, table, table)
	_, err := db.ExecContext(ctx, q, window.SQL(start), window.SQL(end))
	return err
}

func currentUser(r *http.Request) *auth.Claims {
	return auth.FromContext(r.Context())
}

// checkOwnership refuses the request (writing a 403) unless the caller is
// either an Admin or the original creator of the row. Every delete handler
// must call this before touching the row.
func checkOwnership(w http.ResponseWriter, r *http.Request, createdBy string) bool {
	claims := currentUser(r)
	if claims == nil {
		writeErr(w, 401, "chua dang nhap")
		return false
	}
	if auth.HasRole(claims.Roles, "Admin") || claims.EmployeeCode == createdBy {
		return true
	}
	writeErr(w, 403, "ban khong co quyen sua/xoa du lieu cua nguoi khac")
	return false
}

// checkEditOwnership is checkOwnership plus an allowance for the Moderator
// role, which may edit anyone's row but must never be able to delete one —
// checkOwnership (used by every Delete handler) deliberately does not grant
// Moderator that bypass. Every Update handler calls this one instead.
func checkEditOwnership(w http.ResponseWriter, r *http.Request, createdBy string) bool {
	claims := currentUser(r)
	if claims == nil {
		writeErr(w, 401, "chua dang nhap")
		return false
	}
	if auth.HasRole(claims.Roles, "Admin") || auth.HasRole(claims.Roles, "Moderator") || claims.EmployeeCode == createdBy {
		return true
	}
	writeErr(w, 403, "ban khong co quyen sua du lieu cua nguoi khac")
	return false
}

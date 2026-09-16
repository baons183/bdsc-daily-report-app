package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"bdscdailyreport/internal/auth"
)

// Accounts/passwords no longer live in this app's own dbo.Users table - they
// come from the shared MaintenanceUserAccount/SDTC_BDSC tables (same DB the
// sibling MaintenanceReportApp/checksheet-app already use for their own
// login), cross-database queried via e.AssetDBName same as assets.go does
// for Asset_Master. Only this app's OWN role assignment (UserAppRole,
// AppKey='bdscDailyReport') is app-specific - see database.py/db.js on the
// sibling apps for the shared schema. Creating/resetting a shared password
// is no longer this app's job at all (use MaintenanceReportApp's own
// register / forgot-password flow).

type loginReq struct {
	EmployeeCode string `json:"employeeCode"`
	Password     string `json:"password"`
}

func (e *Env) Login(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in loginReq
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErr(w, 400, "du lieu khong hop le")
			return
		}
		if !validSQLIdent.MatchString(e.AssetDBName) {
			writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
			return
		}
		empCode := strings.ToUpper(strings.TrimSpace(in.EmployeeCode))

		var canonicalCode, salt, hash string
		var isActive bool
		q := fmt.Sprintf(`
			SELECT EmployeeCode, PasswordSalt, PasswordHash, IsActive
			FROM %s.dbo.MaintenanceUserAccount
			WHERE UPPER(LTRIM(RTRIM(EmployeeCode))) = ?`, e.AssetDBName)
		err := e.DB.QueryRowContext(r.Context(), q, empCode).
			Scan(&canonicalCode, &salt, &hash, &isActive)
		if err != nil || !auth.VerifySharedPassword(in.Password, salt, hash) {
			writeErr(w, 401, "sai ma nhan vien hoac mat khau")
			return
		}
		if !isActive {
			writeErr(w, 403, "tai khoan da bi khoa")
			return
		}

		var fullName string
		qName := fmt.Sprintf(`
			SELECT RequesterName FROM %s.dbo.SDTC_BDSC
			WHERE UPPER(LTRIM(RTRIM(EmployeeCode))) = ?`, e.AssetDBName)
		_ = e.DB.QueryRowContext(r.Context(), qName, canonicalCode).Scan(&fullName)

		var roles string
		qRole := fmt.Sprintf(`
			SELECT Role FROM %s.dbo.UserAppRole
			WHERE EmployeeCode = ? AND AppKey = 'bdscDailyReport'`, e.AssetDBName)
		errRole := e.DB.QueryRowContext(r.Context(), qRole, canonicalCode).Scan(&roles)
		if errRole != nil || strings.TrimSpace(roles) == "" {
			writeErr(w, 403, "tai khoan chua duoc cap quyen dung app nay - lien he Admin")
			return
		}

		tok, err := auth.IssueToken(jwtSecret, canonicalCode, fullName, roles)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{
			"token": tok, "employeeCode": canonicalCode, "fullName": fullName, "roles": roles,
		})
	}
}

type registerReq struct {
	EmployeeCode string `json:"employeeCode"`
	Password     string `json:"password"`
	FullName     string `json:"fullName"`
}

// RegisterViewer no longer creates a new account/password - it verifies the
// caller's EXISTING shared credentials (from MaintenanceReportApp/
// checksheet-app) and, if they don't have a BDSC Daily Report role yet,
// grants them Viewer (read-only: Thống kê + Báo cáo tuần). An Admin can add
// more roles afterwards from the Quản trị tab. FullName in the request body
// is accepted but ignored (kept for frontend request-shape compatibility) -
// the real name comes from SDTC_BDSC.
func (e *Env) RegisterViewer(w http.ResponseWriter, r *http.Request) {
	var in registerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.EmployeeCode == "" || in.Password == "" {
		writeErr(w, 400, "thieu ma nhan vien/mat khau")
		return
	}
	if !validSQLIdent.MatchString(e.AssetDBName) {
		writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
		return
	}
	empCode := strings.ToUpper(strings.TrimSpace(in.EmployeeCode))

	var salt, hash string
	var isActive bool
	q := fmt.Sprintf(`
		SELECT PasswordSalt, PasswordHash, IsActive FROM %s.dbo.MaintenanceUserAccount
		WHERE UPPER(LTRIM(RTRIM(EmployeeCode))) = ?`, e.AssetDBName)
	err := e.DB.QueryRowContext(r.Context(), q, empCode).Scan(&salt, &hash, &isActive)
	if err != nil {
		writeErr(w, 404, "Chua co tai khoan dung chung - dang nhap Maintenance Report App hoac Checksheet truoc de tao tai khoan, roi quay lai dang ky o day.")
		return
	}
	if !isActive || !auth.VerifySharedPassword(in.Password, salt, hash) {
		writeErr(w, 401, "sai ma nhan vien hoac mat khau")
		return
	}

	var count int
	qc := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.dbo.UserAppRole
		WHERE EmployeeCode = ? AND AppKey = 'bdscDailyReport'`, e.AssetDBName)
	_ = e.DB.QueryRowContext(r.Context(), qc, empCode).Scan(&count)
	if count > 0 {
		writeErr(w, 409, "tai khoan nay da duoc cap quyen dung app roi, vui long dang nhap")
		return
	}

	qi := fmt.Sprintf(`
		INSERT INTO %s.dbo.UserAppRole (EmployeeCode, AppKey, Role)
		VALUES (?, 'bdscDailyReport', 'Viewer')`, e.AssetDBName)
	if _, err := e.DB.ExecContext(r.Context(), qi, empCode); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

type userResp struct {
	EmployeeCode string `json:"employeeCode"`
	FullName     string `json:"fullName"`
	Roles        string `json:"roles"`
}

// ListUsers returns every account granted a BDSC Daily Report role (Admin
// only) - no PasswordHash to omit anymore, that lives entirely in the
// shared MaintenanceUserAccount table this app no longer touches.
func (e *Env) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !validSQLIdent.MatchString(e.AssetDBName) {
		writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
		return
	}
	q := fmt.Sprintf(`
		SELECT ur.EmployeeCode, ISNULL(s.RequesterName, ''), ur.Role
		FROM %s.dbo.UserAppRole ur
		LEFT JOIN %s.dbo.SDTC_BDSC s ON UPPER(LTRIM(RTRIM(s.EmployeeCode))) = ur.EmployeeCode
		WHERE ur.AppKey = 'bdscDailyReport'
		ORDER BY ur.EmployeeCode`, e.AssetDBName, e.AssetDBName)
	rows, err := e.DB.QueryContext(r.Context(), q)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []userResp{}
	for rows.Next() {
		var u userResp
		if err := rows.Scan(&u.EmployeeCode, &u.FullName, &u.Roles); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, u)
	}
	writeJSON(w, 200, out)
}

type createUserReq struct {
	EmployeeCode string `json:"employeeCode"`
	Roles        string `json:"roles"`
}

// CreateUser lets an Admin grant BDSC Daily Report roles (SuCo/GiamSat/
// CongViec/KPI/Admin/Approver/Moderator/Viewer, comma-separated) to someone
// who already has a shared account - it can no longer create the
// account/password itself. Acts as an upsert (MERGE) so re-submitting for
// an EmployeeCode that already has a role here just updates it, matching
// what UpdateUser used to do.
func (e *Env) CreateUser(w http.ResponseWriter, r *http.Request) {
	var in createUserReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.EmployeeCode == "" {
		writeErr(w, 400, "thieu employeeCode")
		return
	}
	if !validSQLIdent.MatchString(e.AssetDBName) {
		writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
		return
	}
	empCode := strings.ToUpper(strings.TrimSpace(in.EmployeeCode))

	var exists int
	qc := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.dbo.MaintenanceUserAccount
		WHERE UPPER(LTRIM(RTRIM(EmployeeCode))) = ?`, e.AssetDBName)
	_ = e.DB.QueryRowContext(r.Context(), qc, empCode).Scan(&exists)
	if exists == 0 {
		writeErr(w, 404, "Nguoi nay chua co tai khoan dung chung - can dang ky qua Maintenance Report App hoac Checksheet truoc.")
		return
	}

	qm := fmt.Sprintf(`
		MERGE %s.dbo.UserAppRole AS t
		USING (SELECT ? AS EmployeeCode, 'bdscDailyReport' AS AppKey) AS s
		ON t.EmployeeCode = s.EmployeeCode AND t.AppKey = s.AppKey
		WHEN MATCHED THEN UPDATE SET Role = ?, UpdatedAt = GETDATE()
		WHEN NOT MATCHED THEN INSERT (EmployeeCode, AppKey, Role) VALUES (s.EmployeeCode, s.AppKey, ?);`,
		e.AssetDBName)
	if _, err := e.DB.ExecContext(r.Context(), qm, empCode, in.Roles, in.Roles); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

type updateUserReq struct {
	Roles string `json:"roles"`
}

// UpdateUser edits an existing account's Roles for this app (Admin only).
// Keyed by EmployeeCode (path param) now, not a numeric Id - UserAppRole has
// no surrogate id, its natural key is (EmployeeCode, AppKey). FullName and
// Password are no longer editable here: FullName always comes live from
// SDTC_BDSC, and password changes belong to the shared account (see
// RegisterViewer's comment).
func (e *Env) UpdateUser(w http.ResponseWriter, r *http.Request) {
	empCode := strings.ToUpper(strings.TrimSpace(r.PathValue("employeeCode")))
	if empCode == "" {
		writeErr(w, 400, "thieu ma nhan vien")
		return
	}
	var in updateUserReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le")
		return
	}
	if !validSQLIdent.MatchString(e.AssetDBName) {
		writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
		return
	}
	q := fmt.Sprintf(`
		UPDATE %s.dbo.UserAppRole SET Role = ?, UpdatedAt = GETDATE()
		WHERE EmployeeCode = ? AND AppKey = 'bdscDailyReport'`, e.AssetDBName)
	res, err := e.DB.ExecContext(r.Context(), q, in.Roles, empCode)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "khong tim thay quyen cua nguoi nay cho app")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

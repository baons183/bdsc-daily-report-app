package handlers

import (
	"context"
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
// for Asset_Master. This app has no login screen of its own anymore either -
// see SsoExchange below: the web UI redirects an unauthenticated visitor to
// the shared Portal (http://pcu-mnt-250223:8100) and exchanges the token it
// hands back for one of this app's own (role-bearing) tokens.

// resolveAppUser looks up an already-identified EmployeeCode's display name
// (SDTC_BDSC) and this app's own role assignment (UserAppRole,
// AppKey='bdscDailyReport'). Returns a non-nil err if the person has no role
// here yet - the caller isn't authenticated by password here, just already
// proven to be who they say by a verified token, so "no role" is the only
// failure mode left to check.
func (e *Env) resolveAppUser(ctx context.Context, empCode string) (fullName, roles string, err error) {
	qName := fmt.Sprintf(`
		SELECT RequesterName FROM %s.dbo.SDTC_BDSC
		WHERE UPPER(LTRIM(RTRIM(EmployeeCode))) = ?`, e.AssetDBName)
	_ = e.DB.QueryRowContext(ctx, qName, empCode).Scan(&fullName)

	qRole := fmt.Sprintf(`
		SELECT Role FROM %s.dbo.UserAppRole
		WHERE EmployeeCode = ? AND AppKey = 'bdscDailyReport'`, e.AssetDBName)
	errRole := e.DB.QueryRowContext(ctx, qRole, empCode).Scan(&roles)
	if errRole != nil || strings.TrimSpace(roles) == "" {
		return fullName, "", fmt.Errorf("no role assigned")
	}
	return fullName, roles, nil
}

type ssoExchangeReq struct {
	Token string `json:"token"`
}

// SsoExchange takes a token issued by any sibling app sharing the same
// JWT_SECRET_KEY (Portal, MaintenanceReportApp, checksheet-app - identified
// by the shared "sub" claim) and, if the EmployeeCode it names is active and
// has a role here, exchanges it for one of this app's own tokens (which,
// unlike the incoming one, carries this app's own Roles claim - see
// auth.IssueToken/auth.Middleware, which trusts that claim directly rather
// than re-querying the DB on every request). Public route - the caller has
// no BDSC Daily Report token yet by definition.
func (e *Env) SsoExchange(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in ssoExchangeReq
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Token == "" {
			writeErr(w, 400, "thieu token")
			return
		}
		claims, err := auth.ParseToken(jwtSecret, in.Token)
		if err != nil || claims.EmployeeCode == "" {
			writeErr(w, 401, "token khong hop le hoac het han, vui long dang nhap lai")
			return
		}
		if !validSQLIdent.MatchString(e.AssetDBName) {
			writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
			return
		}
		empCode := strings.ToUpper(strings.TrimSpace(claims.EmployeeCode))

		var isActive bool
		qActive := fmt.Sprintf(`
			SELECT IsActive FROM %s.dbo.MaintenanceUserAccount
			WHERE UPPER(LTRIM(RTRIM(EmployeeCode))) = ?`, e.AssetDBName)
		if err := e.DB.QueryRowContext(r.Context(), qActive, empCode).Scan(&isActive); err != nil {
			writeErr(w, 401, "tai khoan khong ton tai")
			return
		}
		if !isActive {
			writeErr(w, 403, "tai khoan da bi khoa")
			return
		}

		fullName, roles, err := e.resolveAppUser(r.Context(), empCode)
		if err != nil {
			// Distinguishable from other failures so the frontend can offer
			// "Đăng ký dùng app này" instead of just an error message.
			writeJSON(w, 403, map[string]string{
				"error": "tai khoan chua duoc cap quyen dung app nay",
				"code":  "NO_ROLE", "employeeCode": empCode, "fullName": fullName,
			})
			return
		}

		tok, err := auth.IssueToken(jwtSecret, empCode, fullName, roles)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		e.recordLogin(r.Context(), empCode)
		writeJSON(w, 200, map[string]string{
			"token": tok, "employeeCode": empCode, "fullName": fullName, "roles": roles,
		})
	}
}

// recordLogin upserts today's login timestamp into the shared
// dbo.UserAppLastLogin (EmployeeCode+AppKey -> LastLoginAt) for AppKey
// 'bdscDailyReport' - same table AdminPanel's/checksheet-app's/
// MaintenanceReportApp's own db layer already creates (this app never
// creates it itself, same as it never creates dbo.UserAppRole - it only
// ever reads/writes rows in a table one of the others owns). Called once per
// SsoExchange (this app's real login moment - there's no separate password-
// login endpoint anymore), so no throttling needed here unlike the other
// apps' per-request auth middleware. Best-effort: a failure here shouldn't
// block the person from actually logging in.
func (e *Env) recordLogin(ctx context.Context, empCode string) {
	if !validSQLIdent.MatchString(e.AssetDBName) {
		return
	}
	q := fmt.Sprintf(`
		MERGE %s.dbo.UserAppLastLogin AS target
		USING (SELECT ? AS EmployeeCode, 'bdscDailyReport' AS AppKey) AS src
		ON target.EmployeeCode = src.EmployeeCode AND target.AppKey = src.AppKey
		WHEN MATCHED THEN UPDATE SET LastLoginAt = GETDATE()
		WHEN NOT MATCHED THEN INSERT (EmployeeCode, AppKey, LastLoginAt) VALUES (?, 'bdscDailyReport', GETDATE());`,
		e.AssetDBName)
	if _, err := e.DB.ExecContext(ctx, q, empCode, empCode); err != nil {
		fmt.Println("recordLogin loi:", err)
	}
}

type ssoRegisterReq struct {
	Token string `json:"token"`
}

// RegisterViewerSSO grants Viewer access (read-only: Báo cáo ngày + Báo cáo
// tuần) to whoever a verified SSO token identifies, if they don't already
// have a role here - the self-serve "no role yet" screen calls this instead
// of asking for a password again (identity is already proven by the token).
// An Admin can grant more roles afterwards from the Quản trị tab.
func (e *Env) RegisterViewerSSO(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in ssoRegisterReq
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Token == "" {
			writeErr(w, 400, "thieu token")
			return
		}
		claims, err := auth.ParseToken(jwtSecret, in.Token)
		if err != nil || claims.EmployeeCode == "" {
			writeErr(w, 401, "token khong hop le hoac het han, vui long dang nhap lai")
			return
		}
		if !validSQLIdent.MatchString(e.AssetDBName) {
			writeErr(w, 500, "cau hinh ASSET_DB_NAME khong hop le")
			return
		}
		empCode := strings.ToUpper(strings.TrimSpace(claims.EmployeeCode))

		var count int
		qc := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s.dbo.UserAppRole
			WHERE EmployeeCode = ? AND AppKey = 'bdscDailyReport'`, e.AssetDBName)
		_ = e.DB.QueryRowContext(r.Context(), qc, empCode).Scan(&count)
		if count > 0 {
			writeErr(w, 409, "tai khoan nay da duoc cap quyen dung app roi")
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
// RegisterViewerSSO's comment).
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

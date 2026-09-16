package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"bdscdailyreport/internal/auth"
)

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
		var id int
		var canonicalCode, fullName, hash, roles string
		err := e.DB.QueryRowContext(r.Context(),
			`SELECT Id, EmployeeCode, FullName, PasswordHash, Roles FROM dbo.Users WHERE EmployeeCode = ?`, in.EmployeeCode).
			Scan(&id, &canonicalCode, &fullName, &hash, &roles)
		if err != nil || !auth.CheckPassword(hash, in.Password) {
			writeErr(w, 401, "sai ma nhan vien hoac mat khau")
			return
		}
		// Always carry the DB's own casing of EmployeeCode in the token (not
		// whatever the user happened to type), so CreatedBy stays consistent
		// across logins even if SQL Server's collation matched case-insensitively.
		tok, err := auth.IssueToken(jwtSecret, canonicalCode, fullName, roles)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"token": tok, "employeeCode": canonicalCode, "fullName": fullName, "roles": roles})
	}
}

type bootstrapReq struct {
	EmployeeCode string `json:"employeeCode"`
	Password     string `json:"password"`
	FullName     string `json:"fullName"`
}

// BootstrapFirstAdmin creates the very first Admin user, but only while the
// Users table is still empty — after that it always refuses, so it is safe
// to leave this endpoint reachable without auth.
func (e *Env) BootstrapFirstAdmin(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := e.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM dbo.Users`).Scan(&count); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if count > 0 {
		writeErr(w, 403, "he thong da co nguoi dung, khong the khoi tao lai")
		return
	}
	var in bootstrapReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.EmployeeCode == "" || in.Password == "" {
		writeErr(w, 400, "thieu employeeCode/password/fullName")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if _, err := e.DB.ExecContext(r.Context(),
		`INSERT INTO dbo.Users (EmployeeCode, FullName, PasswordHash, Roles) VALUES (?, ?, ?, 'Admin')`,
		in.EmployeeCode, in.FullName, hash); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

type registerReq struct {
	EmployeeCode string `json:"employeeCode"`
	Password     string `json:"password"`
	FullName     string `json:"fullName"`
}

// RegisterViewer lets anyone create their own account without an Admin —
// always hard-codes the Viewer role (read-only: Thống kê + Báo cáo tuần
// tabs only) regardless of what the client sends, so self-registration can
// never grant edit/admin rights. An Admin can add more roles afterwards
// from the Quản trị tab if the person actually needs them.
func (e *Env) RegisterViewer(w http.ResponseWriter, r *http.Request) {
	var in registerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.EmployeeCode == "" || in.Password == "" {
		writeErr(w, 400, "thieu ma nhan vien/mat khau")
		return
	}
	if len(in.Password) < 6 {
		writeErr(w, 400, "mat khau phai co it nhat 6 ky tu")
		return
	}
	var count int
	if err := e.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM dbo.Users WHERE EmployeeCode = ?`, in.EmployeeCode).Scan(&count); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if count > 0 {
		writeErr(w, 409, "ma nhan vien nay da co tai khoan, vui long dang nhap hoac lien he Admin neu quen mat khau")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if _, err := e.DB.ExecContext(r.Context(),
		`INSERT INTO dbo.Users (EmployeeCode, FullName, PasswordHash, Roles) VALUES (?, ?, ?, 'Viewer')`,
		in.EmployeeCode, in.FullName, hash); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

type changePasswordReq struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// ChangePassword lets the currently logged-in user (identified from their
// own JWT, not a path/body-supplied employee code) set a new password,
// after confirming they still know the current one.
func (e *Env) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := currentUser(r)
	if claims == nil {
		writeErr(w, 401, "chua dang nhap")
		return
	}
	var in changePasswordReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le")
		return
	}
	if len(in.NewPassword) < 6 {
		writeErr(w, 400, "mat khau moi phai co it nhat 6 ky tu")
		return
	}
	var hash string
	if err := e.DB.QueryRowContext(r.Context(),
		`SELECT PasswordHash FROM dbo.Users WHERE EmployeeCode = ?`, claims.EmployeeCode).Scan(&hash); err != nil {
		writeErr(w, 404, "khong tim thay tai khoan")
		return
	}
	if !auth.CheckPassword(hash, in.OldPassword) {
		writeErr(w, 401, "mat khau hien tai khong dung")
		return
	}
	newHash, err := auth.HashPassword(in.NewPassword)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if _, err := e.DB.ExecContext(r.Context(),
		`UPDATE dbo.Users SET PasswordHash = ? WHERE EmployeeCode = ?`, newHash, claims.EmployeeCode); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type createUserReq struct {
	EmployeeCode string `json:"employeeCode"`
	Password     string `json:"password"`
	FullName     string `json:"fullName"`
	Roles        string `json:"roles"`
}

// CreateUser lets an Admin add more accounts (SuCo/GiamSat/CongViec/KPI/Admin
// roles, comma-separated).
func (e *Env) CreateUser(w http.ResponseWriter, r *http.Request) {
	var in createUserReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.EmployeeCode == "" || in.Password == "" {
		writeErr(w, 400, "thieu employeeCode/password")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if _, err := e.DB.ExecContext(r.Context(),
		`INSERT INTO dbo.Users (EmployeeCode, FullName, PasswordHash, Roles) VALUES (?, ?, ?, ?)`,
		in.EmployeeCode, in.FullName, hash, in.Roles); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

type userResp struct {
	Id           int    `json:"id"`
	EmployeeCode string `json:"employeeCode"`
	FullName     string `json:"fullName"`
	Roles        string `json:"roles"`
}

// ListUsers returns every account (Admin only) — PasswordHash is never
// included in the response.
func (e *Env) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := e.DB.QueryContext(r.Context(),
		`SELECT Id, EmployeeCode, ISNULL(FullName,''), ISNULL(Roles,'') FROM dbo.Users ORDER BY EmployeeCode`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []userResp{}
	for rows.Next() {
		var u userResp
		if err := rows.Scan(&u.Id, &u.EmployeeCode, &u.FullName, &u.Roles); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, u)
	}
	writeJSON(w, 200, out)
}

type updateUserReq struct {
	FullName string `json:"fullName"`
	Roles    string `json:"roles"`
	Password string `json:"password"` // optional: blank keeps the current password
}

// UpdateUser lets an Admin edit an existing account's FullName/Roles, and
// optionally reset its password (leave Password blank to keep the current
// one). EmployeeCode itself is never editable here — it's the natural key
// every CreatedBy column across the app (SuCo/GiamSat/CongViec/Reliability)
// stores directly, so changing it would silently orphan that user's
// already-saved rows instead of renaming their ownership.
func (e *Env) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErr(w, 400, "id khong hop le")
		return
	}
	var in updateUserReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le")
		return
	}
	if in.Password == "" {
		if _, err := e.DB.ExecContext(r.Context(),
			`UPDATE dbo.Users SET FullName=?, Roles=? WHERE Id=?`, in.FullName, in.Roles, id); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if len(in.Password) < 6 {
		writeErr(w, 400, "mat khau moi phai co it nhat 6 ky tu")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if _, err := e.DB.ExecContext(r.Context(),
		`UPDATE dbo.Users SET FullName=?, Roles=?, PasswordHash=? WHERE Id=?`, in.FullName, in.Roles, hash, id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

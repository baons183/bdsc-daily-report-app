package handlers

import (
	"encoding/json"
	"net/http"

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

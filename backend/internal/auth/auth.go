package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type ctxKey string

const userCtxKey ctxKey = "user"

type Claims struct {
	EmployeeCode string `json:"sub"`
	FullName     string `json:"name"`
	Roles        string `json:"roles"`
	jwt.RegisteredClaims
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func IssueToken(secret, employeeCode, fullName, roles string) (string, error) {
	claims := Claims{
		EmployeeCode: employeeCode,
		FullName:     fullName,
		Roles:        roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(8 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

func ParseToken(secret, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		if err == nil {
			err = jwt.ErrTokenInvalidClaims
		}
		return nil, err
	}
	return claims, nil
}

// Middleware validates the Bearer token and stores the parsed claims in the
// request context; handlers read them back with FromContext.
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "thieu token dang nhap", http.StatusUnauthorized)
				return
			}
			claims, err := ParseToken(secret, strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				http.Error(w, "token khong hop le hoac het han", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(userCtxKey).(*Claims)
	return c
}

// HasRole reports whether roles (comma-separated) satisfies role. Admin
// always passes every check. Approver is treated as equal to Admin
// everywhere EXCEPT the Admin-only routes/panel themselves (role=="Admin")
// — so an Approver-only user passes every other role gate (SuCo, GiamSat,
// CongViec, KPI, Moderator...) the same way Admin does, but still can't
// reach the Quản trị tab or its APIs.
func HasRole(roles, role string) bool {
	for _, r := range strings.Split(roles, ",") {
		r = strings.TrimSpace(r)
		if r == role || r == "Admin" {
			return true
		}
		if r == "Approver" && role != "Admin" {
			return true
		}
	}
	return false
}

// RequireRole wraps a handler so only users carrying `role` (or the Admin
// role, which can do everything) may call it.
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := FromContext(r.Context())
		if claims == nil || !HasRole(claims.Roles, role) {
			http.Error(w, "khong co quyen truy cap muc nay", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// RequireEditRole is RequireRole plus the Moderator role — used only on
// "edit" (PUT) routes. Moderator can fix anyone's Sự cố/Giám sát/Công việc
// data but must never be able to delete it, so DELETE routes deliberately
// keep using plain RequireRole (which Moderator doesn't hold) instead.
func RequireEditRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := FromContext(r.Context())
		if claims == nil || !(HasRole(claims.Roles, role) || HasRole(claims.Roles, "Moderator")) {
			http.Error(w, "khong co quyen truy cap muc nay", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

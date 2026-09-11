package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"bdscdailyreport/internal/auth"
	"bdscdailyreport/internal/config"
	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/emailer"
	"bdscdailyreport/internal/handlers"
	"bdscdailyreport/internal/report"
)

func main() {
	cfg := config.Load()

	conn, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("loi ket noi database: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		log.Fatalf("loi tao bang: %v", err)
	}
	log.Printf("da ket noi database %s/%s", cfg.DBServer, cfg.DBDatabase)

	env := &handlers.Env{DB: conn, UploadsDir: cfg.UploadsDir, AssetDBName: cfg.AssetDBName, CBMExcelPath: cfg.CBMExcelPath}

	mux := http.NewServeMux()

	// public
	mux.HandleFunc("POST /api/login", env.Login(cfg.JWTSecret))
	mux.HandleFunc("POST /api/setup/first-admin", env.BootstrapFirstAdmin)

	// authenticated
	authMw := auth.Middleware(cfg.JWTSecret)
	mux.Handle("GET /api/assets/search", authMw(http.HandlerFunc(env.SearchAssets)))
	mux.Handle("GET /api/cbm", authMw(http.HandlerFunc(env.ListCBM)))
	mux.Handle("GET /api/report/view", authMw(http.HandlerFunc(env.GetReportView)))

	mux.Handle("GET /api/suco", authMw(http.HandlerFunc(env.ListSuCo)))
	mux.Handle("POST /api/suco", authMw(http.HandlerFunc(auth.RequireRole("SuCo", env.CreateSuCo))))
	mux.Handle("PUT /api/suco/{id}", authMw(http.HandlerFunc(auth.RequireRole("SuCo", env.UpdateSuCo))))
	mux.Handle("DELETE /api/suco/{id}", authMw(http.HandlerFunc(auth.RequireRole("SuCo", env.DeleteSuCo))))

	mux.Handle("GET /api/giamsat", authMw(http.HandlerFunc(env.ListGiamSat)))
	mux.Handle("POST /api/giamsat", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.CreateGiamSat))))
	mux.Handle("PUT /api/giamsat/{id}", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.UpdateGiamSat))))
	mux.Handle("DELETE /api/giamsat/{id}", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.DeleteGiamSat))))

	mux.Handle("GET /api/congviec", authMw(http.HandlerFunc(env.ListCongViec)))
	mux.Handle("POST /api/congviec", authMw(http.HandlerFunc(auth.RequireRole("CongViec", env.CreateCongViec))))
	mux.Handle("PUT /api/congviec/{id}", authMw(http.HandlerFunc(auth.RequireRole("CongViec", env.UpdateCongViec))))
	mux.Handle("DELETE /api/congviec/{id}", authMw(http.HandlerFunc(auth.RequireRole("CongViec", env.DeleteCongViec))))

	mux.Handle("GET /api/kpi", authMw(http.HandlerFunc(env.ListKPI)))
	mux.Handle("POST /api/kpi", authMw(http.HandlerFunc(auth.RequireRole("KPI", env.SaveKPI))))

	mux.Handle("POST /api/users", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.CreateUser))))

	mux.Handle("POST /api/report/run-now", authMw(http.HandlerFunc(auth.RequireRole("Admin", func(w http.ResponseWriter, r *http.Request) {
		runAt, err := parseRunAt(r)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		path, err := report.RunDaily(conn, cfg.UploadsDir, cfg.ReportsDir, cfg.CBMExcelPath, runAt)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write([]byte(path))
	}))))

	mux.Handle("POST /api/report/send-test-email", authMw(http.HandlerFunc(auth.RequireRole("Admin", func(w http.ResponseWriter, r *http.Request) {
		if cfg.EmailFlowURL == "" {
			http.Error(w, "chua cau hinh EMAIL_FLOW_URL trong .env", 400)
			return
		}
		runAt, err := parseRunAt(r)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		path, err := report.RunDaily(conn, cfg.UploadsDir, cfg.ReportsDir, cfg.CBMExcelPath, runAt)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := emailer.SendReportPDF(cfg.EmailFlowURL, path, runAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write([]byte("Da gui email: " + path))
	}))))

	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))
	mux.Handle("GET /reports/", http.StripPrefix("/reports/", http.FileServer(http.Dir(cfg.ReportsDir))))
	mux.Handle("/", noCache(http.FileServer(http.Dir("./web"))))

	go scheduleDailyReport(conn, cfg)

	log.Printf("dang chay tren cong %s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, withCORS(mux)))
}

// scheduleDailyReport sleeps until the next 08:00 local time on a weekday
// (Mon-Fri, skipping Saturday/Sunday), generates the report, emails it via
// the configured Power Automate Flow, then repeats forever. Runs in its own
// goroutine for the lifetime of the process; a failed run is logged (via
// report.RunDaily's own dbo.ReportRuns row) but never crashes the loop.
func scheduleDailyReport(conn *sql.DB, cfg config.Config) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), cfg.ReportHour, cfg.ReportMinute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
			next = next.AddDate(0, 0, 1)
		}
		sleepDur := next.Sub(now)
		log.Printf("bao cao ngay tiep theo se chay luc %s (con %s)", next.Format("15:04 02/01/2006 (Mon)"), sleepDur)
		time.Sleep(sleepDur)

		runAt := time.Now()
		path, err := report.RunDaily(conn, cfg.UploadsDir, cfg.ReportsDir, cfg.CBMExcelPath, runAt)
		if err != nil {
			log.Printf("loi sinh bao cao ngay: %v", err)
			continue
		}
		log.Printf("da sinh bao cao ngay: %s", path)

		if err := emailer.SendReportPDF(cfg.EmailFlowURL, path, runAt); err != nil {
			log.Printf("loi gui email bao cao: %v", err)
		} else if cfg.EmailFlowURL != "" {
			log.Printf("da gui email bao cao ngay")
		}
	}
}

// parseRunAt reads an optional "?date=YYYY-MM-DD" query param (used by the
// Admin panel's "chọn ngày" test tools) and resolves it to 08:00 that day —
// matching the same time-of-day the real scheduled run always uses, since
// window.ReportWindow only looks at the date part. Falls back to time.Now()
// when the param is absent.
func parseRunAt(r *http.Request) (time.Time, error) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		return time.Now(), nil
	}
	d, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return time.Time{}, errors.New("ngay khong hop le, can dang YYYY-MM-DD")
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 8, 0, 0, 0, time.Local), nil
}

// noCache stops browsers from caching the static web/ assets (index.html,
// app.js) — this app is still actively edited, and a stale cached app.js
// makes a real code fix look like it "didn't take" until a hard refresh.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

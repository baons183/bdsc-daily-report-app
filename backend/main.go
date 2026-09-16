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
	"bdscdailyreport/internal/window"
)

func main() {
	cfg := config.Load()

	// Chờ tối đa 5 phút, thử lại mỗi 5 giây — cần thiết vì app tự chạy bằng
	// Scheduled Task lúc khởi động máy, SQL Server có thể chưa kịp sẵn sàng.
	conn, err := db.ConnectWithRetry(cfg, 5*time.Minute, 5*time.Second)
	if err != nil {
		log.Fatalf("loi ket noi database: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		log.Fatalf("loi tao bang: %v", err)
	}
	log.Printf("da ket noi database %s/%s", cfg.DBServer, cfg.DBDatabase)

	env := &handlers.Env{DB: conn, UploadsDir: cfg.UploadsDir, AssetDBName: cfg.AssetDBName, CBMExcelPath: cfg.CBMExcelPath, EmailFlowApprovalURL: cfg.EmailFlowApprovalURL, AppBaseURL: cfg.AppBaseURL, RSMPdfFolder: cfg.RSMPdfFolder, ReportsDir: cfg.ReportsDir, ReportSharePointURLPrefix: cfg.ReportSharePointURLPrefix}

	mux := http.NewServeMux()

	// public
	mux.HandleFunc("POST /api/login", env.Login(cfg.JWTSecret))
	mux.HandleFunc("POST /api/setup/first-admin", env.BootstrapFirstAdmin)
	mux.HandleFunc("POST /api/register", env.RegisterViewer)

	// authenticated
	authMw := auth.Middleware(cfg.JWTSecret)
	mux.Handle("GET /api/assets/search", authMw(http.HandlerFunc(env.SearchAssets)))
	mux.Handle("POST /api/change-password", authMw(http.HandlerFunc(env.ChangePassword)))
	mux.Handle("GET /api/cbm", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.ListCBM))))
	mux.Handle("POST /api/cbm/exclude", authMw(http.HandlerFunc(env.SetCBMExclusion)))
	mux.Handle("GET /api/report/view", authMw(http.HandlerFunc(env.GetReportView)))
	mux.Handle("GET /api/report/weekly", authMw(http.HandlerFunc(env.GetWeeklyReport)))

	mux.Handle("GET /api/suco", authMw(http.HandlerFunc(env.ListSuCo)))
	mux.Handle("POST /api/suco", authMw(http.HandlerFunc(auth.RequireRole("SuCo", env.CreateSuCo))))
	mux.Handle("PUT /api/suco/{id}", authMw(http.HandlerFunc(auth.RequireEditRole("SuCo", env.UpdateSuCo))))
	mux.Handle("DELETE /api/suco/{id}", authMw(http.HandlerFunc(auth.RequireRole("SuCo", env.DeleteSuCo))))

	mux.Handle("GET /api/giamsat", authMw(http.HandlerFunc(env.ListGiamSat)))
	mux.Handle("POST /api/giamsat", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.CreateGiamSat))))
	mux.Handle("PUT /api/giamsat/{id}", authMw(http.HandlerFunc(auth.RequireEditRole("GiamSat", env.UpdateGiamSat))))
	mux.Handle("DELETE /api/giamsat/{id}", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.DeleteGiamSat))))

	mux.Handle("GET /api/congviec", authMw(http.HandlerFunc(env.ListCongViec)))
	mux.Handle("POST /api/congviec", authMw(http.HandlerFunc(auth.RequireRole("CongViec", env.CreateCongViec))))
	mux.Handle("PUT /api/congviec/{id}", authMw(http.HandlerFunc(auth.RequireEditRole("CongViec", env.UpdateCongViec))))
	mux.Handle("DELETE /api/congviec/{id}", authMw(http.HandlerFunc(auth.RequireRole("CongViec", env.DeleteCongViec))))

	mux.Handle("GET /api/kpi", authMw(http.HandlerFunc(env.ListKPI)))
	mux.Handle("POST /api/kpi", authMw(http.HandlerFunc(auth.RequireRole("KPI", env.SaveKPI))))

	mux.Handle("GET /api/kpi-baseline", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.ListKPIBaseline))))
	mux.Handle("POST /api/kpi-baseline", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.SetKPIBaseline))))

	mux.Handle("POST /api/users", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.CreateUser))))

	mux.Handle("GET /api/schedule-overrides", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.ListScheduleOverrides))))
	mux.Handle("POST /api/schedule-overrides", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.AddScheduleOverride))))
	mux.Handle("DELETE /api/schedule-overrides/{id}", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.DeleteScheduleOverride))))

	mux.Handle("GET /api/report-schedule", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.GetReportSchedule))))
	mux.Handle("POST /api/report-schedule", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.SetReportSchedule))))

	mux.Handle("GET /api/cbm-excel-path", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.GetCBMExcelPathSetting))))
	mux.Handle("POST /api/cbm-excel-path", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.SetCBMExcelPathSetting))))

	mux.Handle("GET /api/rsm-pdf-folder", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.GetRSMPdfFolderSetting))))
	mux.Handle("POST /api/rsm-pdf-folder", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.SetRSMPdfFolderSetting))))
	mux.Handle("GET /api/reliability", authMw(http.HandlerFunc(env.ListReliability)))
	mux.Handle("POST /api/reliability", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.CreateReliability))))
	mux.Handle("PUT /api/reliability/{id}", authMw(http.HandlerFunc(auth.RequireEditRole("GiamSat", env.UpdateReliability))))
	mux.Handle("DELETE /api/reliability/{id}", authMw(http.HandlerFunc(auth.RequireRole("GiamSat", env.DeleteReliability))))

	mux.Handle("GET /api/send-all-days", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.GetSendAllDays))))
	mux.Handle("POST /api/send-all-days", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.SetSendAllDays))))

	mux.Handle("GET /api/approval-schedule", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.GetApprovalSchedule))))
	mux.Handle("POST /api/approval-schedule", authMw(http.HandlerFunc(auth.RequireRole("Admin", env.SetApprovalSchedule))))

	mux.Handle("GET /api/report-approval", authMw(http.HandlerFunc(env.GetReportApproval)))
	mux.Handle("POST /api/report-approval/review", authMw(http.HandlerFunc(auth.RequireRole("Moderator", env.ReviewReport))))
	mux.Handle("POST /api/report-approval/approve", authMw(http.HandlerFunc(auth.RequireRole("Approver", env.ApproveReport))))
	mux.Handle("POST /api/report-approval/reject", authMw(http.HandlerFunc(auth.RequireRole("Approver", env.RejectReport))))

	mux.Handle("POST /api/report/run-now", authMw(http.HandlerFunc(auth.RequireRole("Admin", func(w http.ResponseWriter, r *http.Request) {
		runAt, err := parseRunAt(r)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		path, _, err := report.RunDaily(conn, cfg.UploadsDir, cfg.ReportsDir, db.GetSetting(conn, "CBM_EXCEL_PATH", cfg.CBMExcelPath), db.GetSetting(conn, "RSM_PDF_FOLDER", cfg.RSMPdfFolder), cfg.ReportSharePointURLPrefix, runAt)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write([]byte(path))
	}))))

	mux.Handle("POST /api/report/send-test-email", authMw(http.HandlerFunc(auth.RequireRole("Admin", func(w http.ResponseWriter, r *http.Request) {
		if cfg.EmailFlowTestURL == "" {
			http.Error(w, "chua cau hinh EMAIL_FLOW_TEST_URL trong .env", 400)
			return
		}
		runAt, err := parseRunAt(r)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		path, emailHTML, err := report.RunDaily(conn, cfg.UploadsDir, cfg.ReportsDir, db.GetSetting(conn, "CBM_EXCEL_PATH", cfg.CBMExcelPath), db.GetSetting(conn, "RSM_PDF_FOLDER", cfg.RSMPdfFolder), cfg.ReportSharePointURLPrefix, runAt)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// Nut test luon di qua Flow test rieng (EMAIL_FLOW_TEST_URL), khong
		// bao gio dung EMAIL_FLOW_URL that - tranh gui nham email that cho
		// Ban Giam doc chi vi dang thu nghiem.
		// Cung ngay hien thi tren PDF/ten file (hom qua, vi cua so du lieu la
		// 8:01 hom qua -> 7:59 hom nay), khong phai ngay sinh bao cao.
		if err := emailer.SendReportPDF(cfg.EmailFlowTestURL, emailHTML, runAt.AddDate(0, 0, -1)); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write([]byte("Da gui email: " + path))
	}))))

	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))
	mux.Handle("GET /reports/", http.StripPrefix("/reports/", http.FileServer(http.Dir(cfg.ReportsDir))))
	mux.Handle("/", noCache(http.FileServer(http.Dir("./web"))))

	go scheduleDailyReport(conn, cfg)
	go scheduleKPICarryForward(conn, cfg)
	go scheduleApprovalDeadlines(conn, env)

	log.Printf("dang chay tren cong %s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, withCORS(mux)))
}

// isRunDay reports whether the daily report should run on date d: true for
// any Mon-Fri, and also true for a Saturday/Sunday that an Admin has
// explicitly added to dbo.ScheduleOverrides as a known exception (e.g. a
// scheduled weekend workday).
func isRunDay(conn *sql.DB, d time.Time) bool {
	if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
		return true
	}
	if db.GetSetting(conn, "SEND_ALL_DAYS", "false") == "true" {
		return true
	}
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM dbo.ScheduleOverrides WHERE OverrideDate = ?`, d.Format("2006-01-02")).Scan(&count)
	if err != nil {
		log.Printf("loi kiem tra ScheduleOverrides cho %s: %v", d.Format("2006-01-02"), err)
		return false
	}
	return count > 0
}

// reportTime reads the Admin-configurable auto-run time from
// dbo.ReportSchedule, falling back to cfg's compiled-in default (8:00) if
// the row is somehow missing or unreadable.
func reportTime(conn *sql.DB, cfg config.Config) (hour, minute int) {
	if err := conn.QueryRow(`SELECT ReportHour, ReportMinute FROM dbo.ReportSchedule WHERE Id = 1`).Scan(&hour, &minute); err != nil {
		return cfg.ReportHour, cfg.ReportMinute
	}
	return hour, minute
}

// scheduleDailyReport polls every minute for whether "now" matches the
// Admin-configured report time (dbo.ReportSchedule, default 08:00) on a day
// isRunDay allows (weekdays, plus any admin-configured weekend exception),
// generates the report, and emails it via the configured Power Automate
// Flow — at most once per calendar day. Polling (rather than sleeping until
// a precomputed time) means a time change made in the Admin panel takes
// effect within a minute, without needing to restart the app. Runs forever
// in its own goroutine; a failed run is logged (via report.RunDaily's own
// dbo.ReportRuns row) but never crashes the loop.
func scheduleDailyReport(conn *sql.DB, cfg config.Config) {
	lastRunDate := ""
	hour, minute := reportTime(conn, cfg)
	log.Printf("bao cao ngay dang cau hinh chay luc %02d:%02d moi ngay (co the doi trong trang Quan tri)", hour, minute)
	for {
		time.Sleep(20 * time.Second)
		now := time.Now()
		hour, minute := reportTime(conn, cfg)
		today := now.Format("2006-01-02")
		if now.Hour() != hour || now.Minute() != minute || today == lastRunDate || !isRunDay(conn, now) {
			continue
		}
		lastRunDate = today

		runAt := now
		path, emailHTML, err := report.RunDaily(conn, cfg.UploadsDir, cfg.ReportsDir, db.GetSetting(conn, "CBM_EXCEL_PATH", cfg.CBMExcelPath), db.GetSetting(conn, "RSM_PDF_FOLDER", cfg.RSMPdfFolder), cfg.ReportSharePointURLPrefix, runAt)
		if err != nil {
			log.Printf("loi sinh bao cao ngay: %v", err)
			continue
		}
		log.Printf("da sinh bao cao ngay: %s", path)

		// Cung ngay hien thi tren PDF/ten file (hom qua, vi cua so du lieu la
		// 8:01 hom qua -> 7:59 hom nay), khong phai ngay sinh bao cao.
		if err := emailer.SendReportPDF(cfg.EmailFlowURL, emailHTML, runAt.AddDate(0, 0, -1)); err != nil {
			log.Printf("loi gui email bao cao: %v", err)
		} else if cfg.EmailFlowURL != "" {
			log.Printf("da gui email bao cao ngay")
		}
	}
}

// scheduleKPICarryForward polls every minute for 5 minutes past the same
// Admin-configurable report time scheduleDailyReport uses (every calendar
// day, weekends included — KPI data entry isn't tied to whether that day's
// PDF gets generated), then calls carryForwardKPIIfEmpty so a day nobody
// touches the KPI tab for still starts with yesterday's numbers as a
// default instead of blank/zero. Runs forever in its own goroutine.
func scheduleKPICarryForward(conn *sql.DB, cfg config.Config) {
	lastRunDate := ""
	for {
		time.Sleep(20 * time.Second)
		now := time.Now()
		hour, minute := reportTime(conn, cfg)
		target := now.Add(-5 * time.Minute)
		today := now.Format("2006-01-02")
		if target.Hour() != hour || target.Minute() != minute || today == lastRunDate {
			continue
		}
		lastRunDate = today

		if err := carryForwardKPIIfEmpty(conn, now); err != nil {
			log.Printf("loi sao chep KPI tu ngay hom truoc: %v", err)
		}
	}
}

// carryForwardKPIIfEmpty duplicates the previous day's KPIEntries rows
// forward into `now`'s report date, but only if that date has no rows yet
// — so it only ever fills in a default, never clobbers whatever a user
// already entered (SaveKPI's own delete+insert is what "nhập thì đè vào"
// means in practice: a manual save always replaces today's rows, whether
// they came from this carry-forward or were still blank).
func carryForwardKPIIfEmpty(conn *sql.DB, now time.Time) error {
	rh, rm := db.ReportTime(conn, 8, 0)
	_, end := window.EntryWindow(now, rh, rm)
	target := end.Format("2006-01-02")
	source := end.AddDate(0, 0, -1).Format("2006-01-02")

	res, err := conn.Exec(`
IF NOT EXISTS (SELECT 1 FROM dbo.KPIEntries WHERE NgayBaoCao = ?)
INSERT INTO dbo.KPIEntries (ChiTieu, Nhom, DVT, KeHoach, ThucHien, LuyKeThang, DanhGia, NgayBaoCao, CreatedBy)
SELECT ChiTieu, Nhom, DVT, KeHoach, ThucHien, LuyKeThang, DanhGia, ?, CreatedBy
FROM dbo.KPIEntries WHERE NgayBaoCao = ?`,
		target, target, source)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("da tu dong sao chep %d dong KPI tu ngay %s sang ngay %s", n, source, target)
	}
	return nil
}

// scheduleApprovalDeadlines polls every 20s for whether "now" matches
// either of the Admin-configured Kiểm tra/Phê duyệt deadline times
// (env.EffectiveApprovalSchedule). If the Kiểm tra deadline passes and
// nobody has reviewed the currently-active report yet (still "draft"), it
// auto-performs Kiểm tra as BS00444 (locking the data) and emails the
// Approvers. If the Phê duyệt deadline passes and nobody has approved yet
// (still "reviewed"), it auto-fills Phê duyệt as BS00742. Both fallback
// employee codes were given explicitly by the user rather than picked by
// this code. window.CurrentReportDate resolves "which printed report is
// currently up for review" at wall-clock time now — NOT the same date
// window.ReportDateForEntry would give a row entered at this same moment
// (see internal/window/window.go for the distinction).
func scheduleApprovalDeadlines(conn *sql.DB, env *handlers.Env) {
	const autoReviewerCode = "BS00444"
	const autoApproverCode = "BS00742"
	lastReviewDate := ""
	lastApproveDate := ""
	for {
		time.Sleep(20 * time.Second)
		now := time.Now()
		kh, km, ph, pm := env.EffectiveApprovalSchedule()
		rh, rm := db.ReportTime(conn, 8, 0)
		reportDate := window.CurrentReportDate(now, rh, rm).Format("2006-01-02")

		if now.Hour() == kh && now.Minute() == km && reportDate != lastReviewDate {
			lastReviewDate = reportDate
			status, err := currentApprovalStatus(conn, reportDate)
			if err != nil {
				log.Printf("loi kiem tra trang thai bao cao %s de tu dong Kiem tra: %v", reportDate, err)
			} else if status == "draft" {
				name := userFullName(conn, autoReviewerCode)
				if err := autoReview(conn, reportDate, autoReviewerCode, name); err != nil {
					log.Printf("loi tu dong Kiem tra bao cao %s: %v", reportDate, err)
				} else {
					log.Printf("da tu dong Kiem tra bao cao %s (qua han, gan cho %s)", reportDate, autoReviewerCode)
					reportTime, _ := time.ParseInLocation("2006-01-02", reportDate, time.Local)
					if err := emailer.SendApprovalReminder(env.EmailFlowApprovalURL, reportTime, name, env.AppBaseURL); err != nil {
						log.Printf("loi gui email nhac Phe duyet cho bao cao %s: %v", reportDate, err)
					}
				}
			}
		}

		if now.Hour() == ph && now.Minute() == pm && reportDate != lastApproveDate {
			lastApproveDate = reportDate
			status, err := currentApprovalStatus(conn, reportDate)
			if err != nil {
				log.Printf("loi kiem tra trang thai bao cao %s de tu dong Phe duyet: %v", reportDate, err)
			} else if status == "reviewed" {
				name := userFullName(conn, autoApproverCode)
				if err := autoApprove(conn, reportDate, autoApproverCode, name); err != nil {
					log.Printf("loi tu dong Phe duyet bao cao %s: %v", reportDate, err)
				} else {
					log.Printf("da tu dong Phe duyet bao cao %s (qua han, gan cho %s)", reportDate, autoApproverCode)
					env.RegenerateApprovedPDF(reportDate)
				}
			}
		}
	}
}

func currentApprovalStatus(conn *sql.DB, reportDate string) (string, error) {
	var status string
	err := conn.QueryRow(`SELECT Status FROM dbo.ReportApprovals WHERE ReportDate = ?`, reportDate).Scan(&status)
	if err == sql.ErrNoRows {
		return "draft", nil
	}
	return status, err
}

func userFullName(conn *sql.DB, employeeCode string) string {
	var name string
	if err := conn.QueryRow(`SELECT FullName FROM dbo.Users WHERE EmployeeCode = ?`, employeeCode).Scan(&name); err != nil {
		return employeeCode
	}
	return name
}

func autoReview(conn *sql.DB, reportDate, code, name string) error {
	_, err := conn.Exec(`
IF EXISTS (SELECT 1 FROM dbo.ReportApprovals WHERE ReportDate = ?)
  UPDATE dbo.ReportApprovals SET Status='reviewed', ReviewedBy=?, ReviewedByName=?, ReviewedAt=GETDATE(), UpdatedAt=GETDATE() WHERE ReportDate = ?
ELSE
  INSERT INTO dbo.ReportApprovals (ReportDate, Status, ReviewedBy, ReviewedByName, ReviewedAt) VALUES (?, 'reviewed', ?, ?, GETDATE())`,
		reportDate, code, name, reportDate, reportDate, code, name)
	return err
}

func autoApprove(conn *sql.DB, reportDate, code, name string) error {
	_, err := conn.Exec(`
UPDATE dbo.ReportApprovals SET Status='approved', ApprovedBy=?, ApprovedByName=?, ApprovedAt=GETDATE(), UpdatedAt=GETDATE()
WHERE ReportDate = ?`, code, name, reportDate)
	return err
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

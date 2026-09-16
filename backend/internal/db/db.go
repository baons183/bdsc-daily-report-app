package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"bdscdailyreport/internal/config"
)

func Connect(cfg config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;encrypt=disable",
		cfg.DBServer, cfg.DBPort, cfg.DBDatabase, cfg.DBUsername, cfg.DBPassword)
	// "mssql" (not "sqlserver") is the driver name that auto-converts "?"
	// placeholders to "@p1, @p2..." for us — every query in this codebase
	// uses "?", matching the convention of the sibling Node/Python apps.
	conn, err := sql.Open("mssql", dsn)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("khong ket noi duoc SQL Server (%s/%s): %w", cfg.DBServer, cfg.DBDatabase, err)
	}
	conn.SetMaxOpenConns(20)
	return conn, nil
}

// ConnectWithRetry keeps retrying Connect every `interval` until it
// succeeds or `timeout` elapses — needed because this app is launched by a
// Scheduled Task at machine boot, which can fire before the SQL Server
// service has finished starting and is ready to accept connections. A
// single failed attempt at boot used to be fatal; this waits it out instead.
func ConnectWithRetry(cfg config.Config, timeout, interval time.Duration) (*sql.DB, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		conn, err := Connect(cfg)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("het thoi gian cho SQL Server san sang (%s): %w", timeout, lastErr)
		}
		log.Printf("SQL Server chua san sang (%v) — thu lai sau %s...", err, interval)
		time.Sleep(interval)
	}
}

// Migrate creates every table this app needs if it does not already exist.
// Safe to run on every startup (idempotent, additive only).
const schemaSQL = `
IF OBJECT_ID('dbo.Users', 'U') IS NULL
CREATE TABLE dbo.Users (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  EmployeeCode NVARCHAR(50) NOT NULL UNIQUE,
  FullName NVARCHAR(200) NOT NULL,
  PasswordHash NVARCHAR(200) NOT NULL,
  Roles NVARCHAR(200) NOT NULL DEFAULT '',
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

IF OBJECT_ID('dbo.SuCo', 'U') IS NULL
CREATE TABLE dbo.SuCo (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  TT INT NOT NULL,
  ThietBi NVARCHAR(300) NOT NULL,
  DM NVARCHAR(20) NULL,
  LoaiSuCo NVARCHAR(30) NULL,
  TinhTrang NVARCHAR(MAX) NULL,
  ThoiGianDungPX FLOAT NULL,
  ThoiGianDungTB FLOAT NULL,
  PhuongAn NVARCHAR(MAX) NULL,
  DonVi NVARCHAR(200) NULL,
  ThoiHan DATE NULL,
  DeXuat NVARCHAR(MAX) NULL,
  LuuY NVARCHAR(MAX) NULL,
  NgayBaoCao DATETIME NOT NULL,
  CreatedBy NVARCHAR(50) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

IF OBJECT_ID('dbo.GiamSat', 'U') IS NULL
CREATE TABLE dbo.GiamSat (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  TT INT NOT NULL,
  ThietBi NVARCHAR(300) NOT NULL,
  DM NVARCHAR(20) NULL,
  TinhTrang NVARCHAR(MAX) NULL,
  RuiRo NVARCHAR(MAX) NULL,
  PhuongAn NVARCHAR(MAX) NULL,
  DonVi NVARCHAR(200) NULL,
  ThoiHan DATE NULL,
  DeXuat NVARCHAR(MAX) NULL,
  LuuY NVARCHAR(MAX) NULL,
  NgayBaoCao DATETIME NOT NULL,
  CreatedBy NVARCHAR(50) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

IF OBJECT_ID('dbo.CongViec', 'U') IS NULL
CREATE TABLE dbo.CongViec (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  TT INT NOT NULL,
  ThietBi NVARCHAR(300) NOT NULL,
  DM NVARCHAR(20) NULL,
  NoiDung NVARCHAR(MAX) NULL,
  KeHoach NVARCHAR(MAX) NULL,
  TienDo FLOAT NULL,
  HomNay NVARCHAR(MAX) NULL,
  NgayMai NVARCHAR(MAX) NULL,
  DeXuat NVARCHAR(MAX) NULL,
  LuuY NVARCHAR(MAX) NULL,
  NgayBaoCao DATETIME NOT NULL,
  CreatedBy NVARCHAR(50) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

IF OBJECT_ID('dbo.CongViecAttachments', 'U') IS NULL
CREATE TABLE dbo.CongViecAttachments (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  CongViecId INT NOT NULL FOREIGN KEY REFERENCES dbo.CongViec(Id) ON DELETE CASCADE,
  FileName NVARCHAR(300) NOT NULL,
  FilePath NVARCHAR(500) NOT NULL,
  ContentType NVARCHAR(100) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

IF OBJECT_ID('dbo.KPIEntries', 'U') IS NULL
CREATE TABLE dbo.KPIEntries (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  ChiTieu NVARCHAR(200) NOT NULL,
  Nhom NVARCHAR(100) NULL,
  DVT NVARCHAR(50) NULL,
  KeHoach FLOAT NULL,
  ThucHien FLOAT NULL,
  LuyKeThang FLOAT NULL,
  DanhGia NVARCHAR(50) NULL,
  NgayBaoCao DATE NOT NULL,
  CreatedBy NVARCHAR(50) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

IF OBJECT_ID('dbo.ReportRuns', 'U') IS NULL
CREATE TABLE dbo.ReportRuns (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  ReportDate DATE NOT NULL,
  FilePath NVARCHAR(500) NULL,
  Status NVARCHAR(50) NOT NULL,
  ErrorMessage NVARCHAR(MAX) NULL,
  GeneratedAt DATETIME NOT NULL DEFAULT GETDATE()
);

-- Specific Saturdays/Sundays that should still run the daily report+email
-- despite the normal weekend skip — set by an Admin for a known exception
-- (e.g. a scheduled weekend workday).
IF OBJECT_ID('dbo.ScheduleOverrides', 'U') IS NULL
CREATE TABLE dbo.ScheduleOverrides (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  OverrideDate DATE NOT NULL UNIQUE,
  CreatedBy NVARCHAR(50) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

-- Additive column for tables that already existed before LoaiSuCo was
-- introduced (CREATE TABLE above only applies to a brand-new database).
IF COL_LENGTH('dbo.SuCo', 'LoaiSuCo') IS NULL
ALTER TABLE dbo.SuCo ADD LoaiSuCo NVARCHAR(30) NULL;

-- One-time starting offset added into "Lũy kế tháng" for a given
-- ChiTieu+YearMonth — needed because this app went live mid-month, so its
-- own auto-computed month-to-date sum is missing whatever already
-- happened earlier that month before the app existed. An Admin sets this
-- once for the current month; a new month simply has no row here yet, so
-- it naturally starts back at 0 with no extra logic needed.
IF OBJECT_ID('dbo.KPIBaseline', 'U') IS NULL
CREATE TABLE dbo.KPIBaseline (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  ChiTieu NVARCHAR(100) NOT NULL,
  YearMonth CHAR(7) NOT NULL,
  BaselineValue FLOAT NOT NULL DEFAULT 0,
  CreatedBy NVARCHAR(50) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE(),
  CONSTRAINT UQ_KPIBaseline UNIQUE (ChiTieu, YearMonth)
);

-- Single-row setting for the daily report's auto-run time (default
-- 08:00), editable by an Admin so this can change without editing .env or
-- restarting the app. Id is always 1 — there is only ever one row.
IF OBJECT_ID('dbo.ReportSchedule', 'U') IS NULL
CREATE TABLE dbo.ReportSchedule (
  Id INT NOT NULL PRIMARY KEY CHECK (Id = 1),
  ReportHour INT NOT NULL DEFAULT 8,
  ReportMinute INT NOT NULL DEFAULT 0,
  UpdatedBy NVARCHAR(50) NULL,
  UpdatedAt DATETIME NOT NULL DEFAULT GETDATE()
);
IF NOT EXISTS (SELECT 1 FROM dbo.ReportSchedule WHERE Id = 1)
INSERT INTO dbo.ReportSchedule (Id, ReportHour, ReportMinute) VALUES (1, 8, 0);

-- Generic key/value store for non-secret settings an Admin can change from
-- the web UI without editing .env or restarting the app (e.g.
-- CBM_EXCEL_PATH). A key with no row here just means "use the .env
-- default" — see db.GetSetting.
IF OBJECT_ID('dbo.AppSettings', 'U') IS NULL
CREATE TABLE dbo.AppSettings (
  SettingKey NVARCHAR(100) NOT NULL PRIMARY KEY,
  SettingValue NVARCHAR(MAX) NULL,
  UpdatedBy NVARCHAR(50) NULL,
  UpdatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

-- Review/approval workflow, one row per printed report date. Status is
-- 'draft' (default, editable normally) -> 'reviewed' (Moderator bấm Kiểm
-- tra — locks the day's SuCo/GiamSat/CongViec against everyone but Admin)
-- -> 'approved' (an Approver bấm Phê duyệt, or auto-filled at the
-- approval deadline if still unapproved). An Approver rejecting sends it
-- back to 'draft' and clears the reviewer fields.
IF OBJECT_ID('dbo.ReportApprovals', 'U') IS NULL
CREATE TABLE dbo.ReportApprovals (
  ReportDate DATE NOT NULL PRIMARY KEY,
  Status NVARCHAR(20) NOT NULL DEFAULT 'draft',
  ReviewedBy NVARCHAR(50) NULL,
  ReviewedByName NVARCHAR(200) NULL,
  ReviewedAt DATETIME NULL,
  ApprovedBy NVARCHAR(50) NULL,
  ApprovedByName NVARCHAR(200) NULL,
  ApprovedAt DATETIME NULL,
  UpdatedAt DATETIME NOT NULL DEFAULT GETDATE()
);

-- Alarm rows a user has unticked "đưa vào báo cáo" for in the Giám sát
-- trực tuyến tab — identified by the same Tagname+Description+DataTime
-- triple dedupeByTagnameEarliest already collapsed repeats down to (see
-- models.CBMExclusionKey). A row here is filtered OUT of the actual report
-- (PDF/email/Thống kê/Báo cáo tuần) but still shown, unchecked, on the raw
-- CBM tab itself so it can be re-included later.
IF OBJECT_ID('dbo.CBMExclusions', 'U') IS NULL
CREATE TABLE dbo.CBMExclusions (
  Id INT IDENTITY PRIMARY KEY,
  Tagname NVARCHAR(200) NOT NULL,
  Description NVARCHAR(500) NOT NULL,
  DataTime DATETIME NOT NULL,
  ExcludedBy NVARCHAR(50) NULL,
  ExcludedAt DATETIME NOT NULL DEFAULT GETDATE()
);

-- Report section "3.3. Vấn đề khác liên quan tới độ tin cậy và toàn vẹn
-- thiết bị" — same shape/lifecycle as dbo.GiamSat (per-report-window TT,
-- editable/deletable like any manually-entered row), except rows can also
-- be auto-imported from BSR's daily RSM PDF (see
-- report.SyncReliabilityFromRSM) — CreatedBy is blank for those.
IF OBJECT_ID('dbo.Reliability', 'U') IS NULL
CREATE TABLE dbo.Reliability (
  Id INT IDENTITY(1,1) PRIMARY KEY,
  TT INT NOT NULL,
  ThietBi NVARCHAR(300) NOT NULL,
  VanDe NVARCHAR(MAX) NULL,
  PhuongAn NVARCHAR(MAX) NULL,
  DonVi NVARCHAR(200) NULL,
  ThoiHan DATE NULL,
  NgayBaoCao DATETIME NOT NULL,
  CreatedBy NVARCHAR(50) NULL,
  CreatedAt DATETIME NOT NULL DEFAULT GETDATE(),
  Included BIT NOT NULL DEFAULT 1
);

-- Additive column for tables that already existed before Included was
-- introduced (CREATE TABLE above only applies to a brand-new database).
-- Deleting an auto-imported (RSM PDF) row used to be pointless — the next
-- sync just re-inserts it since its (ThietBi, VanDe) key still shows up in
-- the source PDF and no longer matches anything in dbo.Reliability. This
-- flag replaces "delete" with "exclude" for that case (see
-- report.SyncReliabilityFromRSM's dedup, which only checks the key still
-- exists here — regardless of Included — so an excluded row is never
-- reinserted as a duplicate) while a manually typed row (which never gets
-- resynced) still deletes for real.
IF COL_LENGTH('dbo.Reliability', 'Included') IS NULL
ALTER TABLE dbo.Reliability ADD Included BIT NOT NULL DEFAULT 1;
`

func Migrate(conn *sql.DB) error {
	_, err := conn.Exec(schemaSQL)
	return err
}

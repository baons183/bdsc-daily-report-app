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
`

func Migrate(conn *sql.DB) error {
	_, err := conn.Exec(schemaSQL)
	return err
}

package db

import (
	"database/sql"
	"fmt"

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
`

func Migrate(conn *sql.DB) error {
	_, err := conn.Exec(schemaSQL)
	return err
}

package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Port         string
	DBServer     string
	DBDatabase   string
	DBUsername   string
	DBPassword   string
	DBPort       string
	JWTSecret    string
	UploadsDir   string
	ReportsDir   string
	CBMExcelPath string
	EmailFlowURL string
	// Separate Flow used only by the "Sinh báo cáo + Gửi email test" button
	// in the Thống kê tab, so testing never touches the real daily flow
	// that emails Ban Giám đốc every morning.
	EmailFlowTestURL string
	// Name of the (separate, shared) database holding Asset_Master — the same
	// equipment master table MaintenanceReportApp/checksheet-app already use
	// for their own Thiết bị lookups. Same SQL Server instance/login as our
	// own DB, so a cross-database query (DBNAME.dbo.Asset_Master) is enough;
	// no second connection needed.
	AssetDBName  string
	ReportHour   int
	ReportMinute int
}

// loadDotEnv reads a .env file (KEY=VALUE per line, # comments, blank lines
// skipped) into the process environment if the key is not already set.
// Mirrors the .env convention used by the sibling Node/Python apps in this
// project family, without pulling in an extra dependency.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() Config {
	loadDotEnv(".env")
	return Config{
		Port:             getEnv("PORT", "8090"),
		DBServer:         getEnv("DB_SERVER", "localhost"),
		DBDatabase:       getEnv("DB_DATABASE", "BDSC_DailyReport"),
		DBUsername:       getEnv("DB_USERNAME", ""),
		DBPassword:       getEnv("DB_PASSWORD", ""),
		DBPort:           getEnv("DB_PORT", "1433"),
		JWTSecret:        getEnv("JWT_SECRET_KEY", "change-me-in-.env"),
		UploadsDir:       getEnv("UPLOADS_DIR", "./uploads"),
		ReportsDir:       getEnv("REPORTS_DIR", "./generated_reports"),
		CBMExcelPath:     getEnv("CBM_EXCEL_PATH", ""),
		EmailFlowURL:     getEnv("EMAIL_FLOW_URL", ""),
		EmailFlowTestURL: getEnv("EMAIL_FLOW_TEST_URL", ""),
		ReportHour:       8,
		ReportMinute:     0,
		AssetDBName:      getEnv("ASSET_DB_NAME", "MNT_Report_Mirror"),
	}
}

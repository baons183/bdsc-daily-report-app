package report

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	appdb "bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/window"
)

// BuildInput carries everything the PDF renderer needs; kept separate from
// the DB layer so Generate() has no side effects other than writing the file.
type BuildInput struct {
	RunAt      time.Time
	Start      time.Time
	End        time.Time
	KPI        []models.KPIEntry
	SuCo       []models.SuCo
	GiamSat    []models.GiamSat
	CongViec   []models.CongViec
	CBM        []models.CBMRow
	UploadsDir string
	OutDir     string
	// Names to print in the PDF's "NGƯỜI LẬP"/"PHÊ DUYỆT" signature boxes —
	// blank if nobody has Kiểm tra/Phê duyệt'd this report date yet.
	ReviewedByName string
	ApprovedByName string
	// The Admin-configured report send time this window was computed
	// against — used only to print a clean "Dữ liệu từ HH:MM ... đến HH:MM
	// ..." label (window.DisplayBounds), since Start/End themselves carry
	// a small internal ±1 minute/second buffer (see internal/window).
	ReportHour   int
	ReportMinute int
}

// GatherData collects everything a report for `runAt`'s window needs,
// without rendering or writing anything — shared by RunDaily (PDF) and the
// web "Thống kê" tab (HTML view of the same data).
func GatherData(db *sql.DB, uploadsDir, cbmExcelPath string, runAt time.Time) (BuildInput, error) {
	ctx := context.Background()
	rh, rm := appdb.ReportTime(db, 8, 0)
	start, end := window.ReportWindow(runAt, rh, rm)

	suco, err := FetchSuCo(ctx, db, start, end)
	if err != nil {
		return BuildInput{}, err
	}
	giamsat, err := FetchGiamSat(ctx, db, start, end)
	if err != nil {
		return BuildInput{}, err
	}
	congviec, err := FetchCongViec(ctx, db, start, end)
	if err != nil {
		return BuildInput{}, err
	}
	kpi, err := fetchKPI(ctx, db, runAt, start, end)
	if err != nil {
		return BuildInput{}, err
	}
	cbm := ReadCBMExcel(cbmExcelPath, start, end)
	if excluded, err := appdb.GetCBMExclusionSet(db); err == nil {
		cbm = FilterCBMExcluded(cbm, excluded)
	}

	reportDate := runAt.AddDate(0, 0, -1).Format("2006-01-02")
	var reviewedByName, approvedByName sql.NullString
	db.QueryRowContext(ctx, `SELECT ReviewedByName, ApprovedByName FROM dbo.ReportApprovals WHERE ReportDate = ?`, reportDate).
		Scan(&reviewedByName, &approvedByName)

	return BuildInput{
		RunAt: runAt, Start: start, End: end,
		KPI: kpi, SuCo: suco, GiamSat: giamsat, CongViec: congviec, CBM: cbm,
		UploadsDir:     uploadsDir,
		ReviewedByName: reviewedByName.String,
		ApprovedByName: approvedByName.String,
		ReportHour:     rh,
		ReportMinute:   rm,
	}, nil
}

// RunDaily gathers today's data window and produces the PDF, logging the
// outcome to dbo.ReportRuns either way. Also returns the HTML email-body
// summary (RenderEmailHTML) built from the exact same data, with its PDF
// link pointing at sharePointURLPrefix + the generated file's name — the
// SharePoint-synced copy of outDir, so the link resolves once sync catches
// up (may lag a little behind the file actually landing on disk).
func RunDaily(db *sql.DB, uploadsDir, outDir, cbmExcelPath, sharePointURLPrefix string, runAt time.Time) (pdfPath, emailHTML string, err error) {
	data, err := GatherData(db, uploadsDir, cbmExcelPath, runAt)
	if err != nil {
		logRun(db, runAt, "", "FAILED", err.Error())
		return "", "", err
	}
	data.OutDir = outDir

	path, err := Generate(data)
	if err != nil {
		logRun(db, runAt, "", "FAILED", err.Error())
		return "", "", err
	}
	logRun(db, runAt, path, "OK", "")

	pdfLinkURL := strings.TrimRight(sharePointURLPrefix, "/") + "/" + url.PathEscape(filepath.Base(path))
	emailHTML = RenderEmailHTML(data, pdfLinkURL)
	return path, emailHTML, nil
}

func logRun(db *sql.DB, runAt time.Time, path, status, errMsg string) {
	db.Exec(`INSERT INTO dbo.ReportRuns (ReportDate, FilePath, Status, ErrorMessage) VALUES (?, ?, ?, ?)`,
		runAt.Format("2006-01-02"), path, status, errMsg)
}

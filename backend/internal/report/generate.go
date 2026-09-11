package report

import (
	"context"
	"database/sql"
	"time"

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
}

// GatherData collects everything a report for `runAt`'s window needs,
// without rendering or writing anything — shared by RunDaily (PDF) and the
// web "Thống kê" tab (HTML view of the same data).
func GatherData(db *sql.DB, uploadsDir, cbmExcelPath string, runAt time.Time) (BuildInput, error) {
	ctx := context.Background()
	start, end := window.ReportWindow(runAt)

	suco, err := fetchSuCo(ctx, db, start, end)
	if err != nil {
		return BuildInput{}, err
	}
	giamsat, err := fetchGiamSat(ctx, db, start, end)
	if err != nil {
		return BuildInput{}, err
	}
	congviec, err := fetchCongViec(ctx, db, start, end)
	if err != nil {
		return BuildInput{}, err
	}
	kpi, err := fetchKPI(ctx, db, runAt)
	if err != nil {
		return BuildInput{}, err
	}
	cbm := ReadCBMExcel(cbmExcelPath, start, end)

	return BuildInput{
		RunAt: runAt, Start: start, End: end,
		KPI: kpi, SuCo: suco, GiamSat: giamsat, CongViec: congviec, CBM: cbm,
		UploadsDir: uploadsDir,
	}, nil
}

// RunDaily gathers today's data window and produces the PDF, logging the
// outcome to dbo.ReportRuns either way.
func RunDaily(db *sql.DB, uploadsDir, outDir, cbmExcelPath string, runAt time.Time) (string, error) {
	data, err := GatherData(db, uploadsDir, cbmExcelPath, runAt)
	if err != nil {
		logRun(db, runAt, "", "FAILED", err.Error())
		return "", err
	}
	data.OutDir = outDir

	path, err := Generate(data)
	if err != nil {
		logRun(db, runAt, "", "FAILED", err.Error())
		return "", err
	}
	logRun(db, runAt, path, "OK", "")
	return path, nil
}

func logRun(db *sql.DB, runAt time.Time, path, status, errMsg string) {
	db.Exec(`INSERT INTO dbo.ReportRuns (ReportDate, FilePath, Status, ErrorMessage) VALUES (?, ?, ?, ?)`,
		runAt.Format("2006-01-02"), path, status, errMsg)
}

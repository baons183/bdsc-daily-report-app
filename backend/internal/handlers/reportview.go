package handlers

import (
	"net/http"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

type reportImage struct {
	ThietBi  string `json:"thietBi"`
	FilePath string `json:"filePath"`
}

type reportViewResponse struct {
	RunAt    string             `json:"runAt"`
	Start    string             `json:"start"`
	End      string             `json:"end"`
	KPI      []models.KPIEntry  `json:"kpi"`
	SuCo     []models.SuCo      `json:"suco"`
	GiamSat  []models.GiamSat   `json:"giamSat"`
	CongViec []models.CongViec  `json:"congViec"`
	CBM      []models.CBMRow    `json:"cbm"`
	RSM      []models.RSMRow    `json:"rsm"`
	DeXuat   []report.NoteItem  `json:"deXuat"`
	LuuY     []report.NoteItem  `json:"luuY"`
	Images   []reportImage      `json:"images"`
	Approval reportApprovalResp `json:"approval"`
}

// GetReportView powers the "Thống kê" tab: the same data that would go into
// a given day's PDF, as JSON, so the web page can render an HTML view of it
// without needing the PDF to have actually been generated for that day.
func (e *Env) GetReportView(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	runAt := time.Now()
	if dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			writeErr(w, 400, "ngay khong hop le, can dang YYYY-MM-DD")
			return
		}
		// Keep the current time-of-day on the chosen date so this always
		// resolves to the same report window RunDaily would have used for
		// that date (RunDaily always fires at 08:00, and ReportWindow only
		// looks at the date part anyway — the time-of-day here doesn't
		// change which window comes out).
		runAt = time.Date(d.Year(), d.Month(), d.Day(), 8, 0, 0, 0, time.Local)
	}

	data, err := report.GatherData(e.DB, e.UploadsDir, e.effectiveCBMExcelPath(), e.effectiveRSMPdfFolder(), runAt)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	images := []reportImage{}
	for _, c := range data.CongViec {
		for _, a := range c.Attachments {
			images = append(images, reportImage{ThietBi: c.ThietBi, FilePath: a.FilePath})
		}
	}

	reportDate := data.RunAt.AddDate(0, 0, -1)
	approval, err := fetchReportApproval(e, r, reportDate.Format("2006-01-02"))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	// The window spans (report time) yesterday -> (report time) today, so
	// the data is really "yesterday's" activity even though it's
	// viewed/generated this morning — the displayed date reflects that,
	// same as the PDF. DisplayBounds shows the clean configured report
	// time on both ends rather than data.Start/End's own ±1 min/sec buffer.
	dispStart, dispEnd := window.DisplayBounds(data.Start, data.End, data.ReportHour, data.ReportMinute)
	writeJSON(w, 200, reportViewResponse{
		RunAt:    reportDate.Format("02/01/2006"),
		Start:    dispStart.Format("15:04 02/01/2006"),
		End:      dispEnd.Format("15:04 02/01/2006"),
		KPI:      data.KPI,
		SuCo:     data.SuCo,
		GiamSat:  data.GiamSat,
		CongViec: data.CongViec,
		CBM:      data.CBM,
		RSM:      data.RSM,
		DeXuat:   report.CombineDeXuat(data.SuCo, data.GiamSat, data.CongViec),
		LuuY:     report.CombineLuuY(data.SuCo, data.GiamSat, data.CongViec),
		Images:   images,
		Approval: approval,
	})
}

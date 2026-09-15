package handlers

import (
	"net/http"
	"time"

	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
)

type weeklyReportResp struct {
	KPI         []models.KPIEntry `json:"kpi"`
	SuCo        []models.SuCo     `json:"suco"`
	SuCoSummary map[string]int    `json:"sucoSummary"`
	GiamSat     []models.GiamSat  `json:"giamsat"`
	CBM         []models.CBMRow   `json:"cbm"`
	CongViec    []models.CongViec `json:"congviec"`
	Images      []reportImage     `json:"images"`
	DeXuat      []report.NoteItem `json:"deXuat"`
	LuuY        []report.NoteItem `json:"luuY"`
}

// GetWeeklyReport powers the "Báo cáo tuần" tab: the same sections as the
// daily report (KPI, Sự cố, cả 2 phần Giám sát, Công việc, Đề xuất, Lưu ý,
// Hình ảnh), aggregated over an arbitrary user-chosen date range instead of
// the daily 08:01-cutoff window the automated PDF uses — just a plain
// calendar range picked via "?start=&end=YYYY-MM-DD", inclusive of both
// endpoints (00:00:00 through 23:59:59).
func (e *Env) GetWeeklyReport(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	if startStr == "" || endStr == "" {
		writeErr(w, 400, "thieu tham so start/end (YYYY-MM-DD)")
		return
	}
	startD, err := time.ParseInLocation("2006-01-02", startStr, time.Local)
	if err != nil {
		writeErr(w, 400, "start khong hop le, can dang YYYY-MM-DD")
		return
	}
	endD, err := time.ParseInLocation("2006-01-02", endStr, time.Local)
	if err != nil {
		writeErr(w, 400, "end khong hop le, can dang YYYY-MM-DD")
		return
	}
	start := time.Date(startD.Year(), startD.Month(), startD.Day(), 0, 0, 0, 0, time.Local)
	end := time.Date(endD.Year(), endD.Month(), endD.Day(), 23, 59, 59, 0, time.Local)
	if end.Before(start) {
		writeErr(w, 400, "ngay ket thuc phai sau ngay bat dau")
		return
	}

	ctx := r.Context()
	kpi, err := report.FetchWeeklyKPI(ctx, e.DB, start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	suco, err := report.FetchSuCo(ctx, e.DB, start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	giamsat, err := report.FetchGiamSat(ctx, e.DB, start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	cbm := report.ReadCBMExcel(e.effectiveCBMExcelPath(), start, end)
	if excluded, err := db.GetCBMExclusionSet(e.DB); err == nil {
		cbm = report.FilterCBMExcluded(cbm, excluded)
	}
	if cbm == nil {
		cbm = []models.CBMRow{}
	}
	congviecAll, err := report.FetchCongViec(ctx, e.DB, start, end)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	// Cùng 1 thiết bị + chuyên ngành (ThietBi+DM) có thể được cập nhật
	// nhiều ngày trong tuần — bảng Công việc chỉ giữ lại đúng bản ghi của
	// ngày MỚI NHẤT cho mỗi cặp đó, phản ánh đúng tình trạng hiện tại thay
	// vì lặp lại từng ngày. Hình ảnh thì vẫn giữ đầy đủ mọi ngày (tư liệu
	// hiện trường, không phải "trạng thái" nên không gộp).
	latestByKey := map[string]models.CongViec{}
	order := []string{}
	for _, c := range congviecAll {
		key := c.ThietBi + "|" + c.DM
		if existing, ok := latestByKey[key]; !ok || c.NgayBaoCao.After(existing.NgayBaoCao) {
			if !ok {
				order = append(order, key)
			}
			latestByKey[key] = c
		}
	}
	congviecLatest := make([]models.CongViec, 0, len(order))
	for _, key := range order {
		congviecLatest = append(congviecLatest, latestByKey[key])
	}

	summary := map[string]int{"Accident": 0, "Near Miss": 0, "Technical Event": 0}
	for _, s := range suco {
		if s.LoaiSuCo != "" {
			summary[s.LoaiSuCo]++
		}
	}

	images := []reportImage{}
	for _, c := range congviecAll {
		for _, a := range c.Attachments {
			images = append(images, reportImage{ThietBi: c.ThietBi, FilePath: a.FilePath})
		}
	}

	writeJSON(w, 200, weeklyReportResp{
		KPI:         kpi,
		SuCo:        suco,
		SuCoSummary: summary,
		GiamSat:     giamsat,
		CBM:         cbm,
		CongViec:    congviecLatest,
		Images:      images,
		DeXuat:      report.CombineDeXuat(suco, giamsat, congviecLatest),
		LuuY:        report.CombineLuuY(suco, giamsat, congviecLatest),
	})
}

package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"bdscdailyreport/internal/db"
	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
	"bdscdailyreport/internal/window"
)

type weeklyReportResp struct {
	KPI         []models.KPIEntry    `json:"kpi"`
	SuCo        []models.SuCo        `json:"suco"`
	SuCoSummary map[string]int       `json:"sucoSummary"`
	GiamSat     []models.GiamSat     `json:"giamsat"`
	CBM         []models.CBMRow      `json:"cbm"`
	Reliability []models.Reliability `json:"reliability"`
	CongViec    []models.CongViec    `json:"congviec"`
	Images      []reportImage        `json:"images"`
	DeXuat      []report.NoteItem    `json:"deXuat"`
	LuuY        []report.NoteItem    `json:"luuY"`
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

	// Sync mỗi ngày trong khoảng đã chọn với file RSM PDF của ngày đó (nếu
	// có) trước khi đọc, để Báo cáo tuần không bị thiếu dữ liệu của những
	// ngày chưa ai mở tab Sự cố/Reliability hoặc sinh báo cáo ngày.
	rh, rm := db.ReportTime(e.DB, 8, 0)
	folder := e.effectiveRSMPdfFolder()
	for d := startD; !d.After(endD); d = d.AddDate(0, 0, 1) {
		runAt := time.Date(d.Year(), d.Month(), d.Day(), rh, rm, 0, 0, time.Local).AddDate(0, 0, 1)
		dayStart, dayEnd := window.ReportWindow(runAt, rh, rm)
		report.SyncSuCoFromRSM(e.DB, folder, d, dayStart, dayEnd)
		yStart, yEnd := window.ReportWindow(d, rh, rm)
		report.CarryForwardReliability(e.DB, yStart, yEnd, dayStart, dayEnd)
		report.SyncReliabilityFromRSM(e.DB, folder, d, dayStart, dayEnd)
	}

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
	reliability, err := report.FetchReliability(ctx, e.DB, start, end)
	if err != nil {
		reliability = []models.Reliability{}
	} else {
		reliability = report.FilterReliabilityIncluded(reliability)
	}

	// Một vấn đề (ThietBi/tag + VanDe) được CarryForwardReliability chép lại
	// mỗi ngày cho tới khi được giải quyết, nên trong khoảng nhiều ngày của
	// Báo cáo tuần, cùng 1 tag name sẽ lặp lại 1 dòng cho mỗi ngày còn mở —
	// chỉ giữ lại đúng bản ghi của ngày MỚI NHẤT cho mỗi cặp đó, cùng cách
	// làm với bảng Công việc ở trên.
	reliabilityLatestByKey := map[string]models.Reliability{}
	reliabilityOrder := []string{}
	for _, r := range reliability {
		key := r.ThietBi + "|" + r.VanDe
		if existing, ok := reliabilityLatestByKey[key]; !ok || r.NgayBaoCao.After(existing.NgayBaoCao) {
			if !ok {
				reliabilityOrder = append(reliabilityOrder, key)
			}
			reliabilityLatestByKey[key] = r
		}
	}
	reliabilityLatest := make([]models.Reliability, 0, len(reliabilityOrder))
	for _, key := range reliabilityOrder {
		reliabilityLatest = append(reliabilityLatest, reliabilityLatestByKey[key])
	}
	for i := range reliabilityLatest {
		reliabilityLatest[i].TT = i + 1
	}
	reliability = reliabilityLatest
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
	// Sắp xếp theo ngày tăng dần để gộp "Hôm nay" đúng thứ tự thời gian.
	sorted := make([]models.CongViec, len(congviecAll))
	copy(sorted, congviecAll)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].NgayBaoCao.Before(sorted[j].NgayBaoCao)
	})

	latestByKey := map[string]models.CongViec{}
	homNayParts := map[string][]string{}
	order := []string{}
	for _, c := range sorted {
		key := c.ThietBi + "|" + c.DM
		if _, ok := latestByKey[key]; !ok {
			order = append(order, key)
		}
		// Đã sort tăng dần nên bản ghi sau luôn mới hơn hoặc bằng bản trước.
		latestByKey[key] = c
		if txt := strings.TrimSpace(c.HomNay); txt != "" {
			homNayParts[key] = append(homNayParts[key], c.NgayBaoCao.Format("02/01")+": "+txt)
		}
	}
	congviecLatest := make([]models.CongViec, 0, len(order))
	for i, key := range order {
		c := latestByKey[key]
		// Chỉ có 1 ngày thì giữ nguyên nội dung gốc, không thêm tiền tố ngày. Sửa lại luôn thêm ngày
		if parts := homNayParts[key]; len(parts) > 0 {
			c.HomNay = strings.Join(parts, "\n")
		}
		c.TT = i + 1
		congviecLatest = append(congviecLatest, c)
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
		Reliability: reliability,
		CongViec:    congviecLatest,
		Images:      images,
		DeXuat:      report.CombineDeXuat(suco, giamsat, congviecLatest),
		LuuY:        report.CombineLuuY(suco, giamsat, congviecLatest),
	})
}

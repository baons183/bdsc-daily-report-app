package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/window"
)

const (
	marginLeft  = 10.0
	marginRight = 10.0
	pageTop     = 10.0
	pageWidth   = 210.0 // A4
	pageBottom  = 280.0
	usableWidth = pageWidth - marginLeft - marginRight
	lineHt      = 5.0
)

type builder struct {
	pdf        *fpdf.Fpdf
	uploadsDir string
}

func newBuilder() *builder {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "A4",
	})
	pdf.SetMargins(marginLeft, pageTop, marginRight)
	pdf.SetAutoPageBreak(true, 15)

	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Times", "I", 9)
		pdf.SetX(marginLeft)
		pdf.CellFormat(usableWidth, 6, fmt.Sprintf("Trang %d/{nb}", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	b := &builder{pdf: pdf}
	pdf.SetHeaderFunc(b.drawHeaderBox)

	timesPath := `C:\Windows\Fonts\times.ttf`
	timesBoldPath := `C:\Windows\Fonts\timesbd.ttf`
	timesItalicPath := `C:\Windows\Fonts\timesi.ttf`
	if _, err := os.Stat(timesPath); err == nil {
		pdf.AddUTF8Font("Times", "", timesPath)
	}
	if _, err := os.Stat(timesBoldPath); err == nil {
		pdf.AddUTF8Font("Times", "B", timesBoldPath)
	}
	if _, err := os.Stat(timesItalicPath); err == nil {
		pdf.AddUTF8Font("Times", "I", timesItalicPath)
	}
	pdf.AddPage()
	pdf.SetFont("Times", "", 11)
	return b
}

const (
	logoBSRPath = "assets/logo-bsr.png"
	logoMNTPath = "assets/logo-mnt.jpg"
	logoHeight  = 20.0
)

// drawHeaderBox draws the logo + company/title box (2 rows x 2 cols) that
// repeats at the top of every page — registered via pdf.SetHeaderFunc so
// fpdf calls it automatically on each AddPage, not just the first.
func (b *builder) drawHeaderBox() {
	pdf := b.pdf
	x0, y0 := marginLeft, pdf.GetY()
	pdf.SetLineWidth(0.3)

	logoW := 38.0
	textW := usableWidth - logoW
	row1H, row2H := 17.0, 9.0
	tbl1H := row1H + row2H

	pdf.Rect(x0, y0, logoW, tbl1H, "D")
	pdf.Rect(x0+logoW, y0, textW, row1H, "D")
	pdf.Rect(x0+logoW, y0+row1H, textW, row2H, "D")

	if _, err := os.Stat(logoBSRPath); err == nil {
		lw, lh := logoW-8, tbl1H-8
		if info := pdf.RegisterImageOptions(logoBSRPath, fpdf.ImageOptions{ImageType: "PNG"}); info != nil {
			if iw, ih := info.Extent(); iw > 0 && ih > 0 {
				scale := lw / iw
				if lh/ih < scale {
					scale = lh / ih
				}
				lw, lh = iw*scale, ih*scale
			}
		}
		pdf.ImageOptions(logoBSRPath, x0+(logoW-lw)/2, y0+(tbl1H-lh)/2, lw, lh, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	}

	pdf.SetXY(x0+logoW, y0+3)
	pdf.SetFont("Times", "B", 12)
	pdf.CellFormat(textW, 6, "TỔNG CÔNG TY LỌC HÓA DẦU VIỆT NAM", "", 2, "C", false, 0, "")
	pdf.SetX(x0 + logoW)
	pdf.CellFormat(textW, 6, "BAN BẢO DƯỠNG SỬA CHỮA", "", 2, "C", false, 0, "")

	pdf.SetXY(x0+logoW, y0+row1H+1.5)
	pdf.CellFormat(textW, 6, "BÁO CÁO NGÀY CÔNG TÁC BẢO DƯỠNG SỬA CHỮA", "", 0, "C", false, 0, "")

	pdf.SetXY(x0, y0+tbl1H)
}

// letterhead reproduces the 3-row memo box (Kính gửi / Copy / Nội dung+Ngày)
// that follows the header box at the top of page 1 only.
func (b *builder) letterhead(runAt, start, end string) {
	pdf := b.pdf
	x0 := marginLeft
	pdf.SetLineWidth(0.3)

	// ---- Box 2: memo (Kính gửi / Copy / Nội dung+Ngày) ----
	y := pdf.GetY()
	labelW := 28.0
	contentW := usableWidth - labelW
	r1H, r2H, r3H := 7.0, 12.0, 8.0
	ngayW := 60.0
	boldContentW := contentW - ngayW

	pdf.Rect(x0, y, labelW, r1H, "D")
	pdf.Rect(x0+labelW, y, contentW, r1H, "D")
	pdf.Rect(x0, y+r1H, labelW, r2H, "D")
	pdf.Rect(x0+labelW, y+r1H, contentW, r2H, "D")
	pdf.Rect(x0, y+r1H+r2H, labelW, r3H, "D")
	pdf.Rect(x0+labelW, y+r1H+r2H, boldContentW, r3H, "D")
	pdf.Rect(x0+labelW+boldContentW, y+r1H+r2H, ngayW, r3H, "D")

	pdf.SetFont("Times", "", 10.5)
	pdf.SetXY(x0+2, y+1.5)
	pdf.CellFormat(labelW-2, 5, "Kính gửi:", "", 0, "L", false, 0, "")
	pdf.SetXY(x0+labelW+2, y+1.5)
	pdf.CellFormat(contentW-2, 5, "-  Ban Giám đốc Nhà máy", "", 0, "L", false, 0, "")

	pdf.SetXY(x0+2, y+r1H+1.5)
	pdf.CellFormat(labelW-2, 5, "Copy:", "", 0, "L", false, 0, "")
	pdf.SetXY(x0+labelW+2, y+r1H+1.5)
	pdf.MultiCell(contentW-4, 5.5, "-  Ban VHSX, KHSX, KTSX (để p/h).\n-  Lưu BDSC.", "", "L", false)

	pdf.SetXY(x0+2, y+r1H+r2H+2)
	pdf.CellFormat(labelW-2, 5, "Nội dung", "", 0, "L", false, 0, "")
	pdf.SetFont("Times", "B", 10.5)
	pdf.SetXY(x0+labelW+2, y+r1H+r2H+2)
	pdf.CellFormat(boldContentW-4, 5, "Báo cáo ngày công tác bảo dưỡng sửa chữa", "", 0, "L", false, 0, "")
	pdf.SetFont("Times", "", 10.5)
	pdf.SetXY(x0+labelW+boldContentW+2, y+r1H+r2H+2)
	pdf.CellFormat(ngayW-4, 5, "Ngày: "+runAt, "", 0, "L", false, 0, "")

	pdf.SetY(y + r1H + r2H + r3H + 4)
	pdf.Ln(2)
}

// signatureBlock reproduces the "NGƯỜI LẬP / PHÊ DUYỆT" sign-off row at the
// end of the original Word report. reviewedByName/approvedByName are the
// Moderator/Approver who Kiểm tra/Phê duyệt'd this report date (blank if
// not yet done) — printed below the "(Ký, ghi rõ họ tên)" hint so the PDF
// carries a name even though there's no real signature image.
func (b *builder) signatureBlock(reviewedByName, approvedByName string) {
	pdf := b.pdf
	if pdf.GetY() > pageBottom-35 {
		pdf.AddPage()
	}
	pdf.Ln(6)
	half := usableWidth / 2
	pdf.SetFont("Times", "B", 11)
	pdf.CellFormat(half, 6, "NGƯỜI LẬP", "", 0, "C", false, 0, "")
	pdf.CellFormat(half, 6, "PHÊ DUYỆT", "", 1, "C", false, 0, "")
	pdf.SetFont("Times", "I", 9)
	pdf.CellFormat(half, 5, "(Ký, ghi rõ họ tên)", "", 0, "C", false, 0, "")
	pdf.CellFormat(half, 5, "(Ký, ghi rõ họ tên)", "", 1, "C", false, 0, "")
	pdf.Ln(10)
	pdf.SetFont("Times", "B", 11)
	pdf.CellFormat(half, 5, reviewedByName, "", 0, "C", false, 0, "")
	pdf.CellFormat(half, 5, approvedByName, "", 1, "C", false, 0, "")
	pdf.SetFont("Times", "", 11)
}

func (b *builder) title(text string, size float64) {
	b.pdf.SetFont("Times", "B", size)
	b.pdf.CellFormat(usableWidth, 8, text, "", 1, "C", false, 0, "")
	b.pdf.SetFont("Times", "", 11)
}

func (b *builder) heading(text string) {
	b.pdf.Ln(2)
	b.pdf.SetFont("Times", "B", 12)
	b.pdf.CellFormat(usableWidth, 7, text, "", 1, "L", false, 0, "")
	b.pdf.SetFont("Times", "", 11)
}

func (b *builder) subheading(text string) {
	b.pdf.SetFont("Times", "B", 11)
	b.pdf.CellFormat(usableWidth, 6, text, "", 1, "L", false, 0, "")
	b.pdf.SetFont("Times", "", 11)
}

func (b *builder) paragraph(text string) {
	b.pdf.MultiCell(usableWidth, lineHt, text, "", "L", false)
}

// table renders a bordered table with word-wrapped cells and manual row
// height (each row is as tall as its tallest wrapped cell needs).
func (b *builder) table(headers []string, widths []float64, rows [][]string, emptyMsg ...string) {
	pdf := b.pdf
	if len(rows) == 0 {
		msg := "Các thiết bị hoạt động ổn định."
		if len(emptyMsg) > 0 {
			msg = emptyMsg[0]
		}
		pdf.SetFont("Times", "I", 10)
		pdf.CellFormat(usableWidth, 6, msg, "1", 1, "C", false, 0, "")
		pdf.SetFont("Times", "", 11)
		return
	}
	drawHeader := func() {
		pdf.SetFont("Times", "B", 9)
		pdf.SetFillColor(224, 224, 224)
		for i, h := range headers {
			pdf.CellFormat(widths[i], 7, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetFont("Times", "", 9)
	}
	if pdf.GetY()+20 > pageBottom {
		pdf.AddPage()
	}
	drawHeader()

	for _, row := range rows {
		maxLines := 1
		cellLines := make([][]string, len(row))
		for i, cell := range row {
			raw := pdf.SplitLines([]byte(cell), widths[i]-2)
			var lines []string
			for _, l := range raw {
				lines = append(lines, string(l))
			}
			if len(lines) == 0 {
				lines = []string{""}
			}
			cellLines[i] = lines
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		rowH := float64(maxLines)*4.2 + 1.5

		if pdf.GetY()+rowH > pageBottom {
			pdf.AddPage()
			drawHeader()
		}
		x, y := pdf.GetX(), pdf.GetY()
		for i := range row {
			pdf.Rect(x, y, widths[i], rowH, "D")
			pdf.SetXY(x+1, y+0.75)
			pdf.MultiCell(widths[i]-2, 4.2, strings.Join(cellLines[i], "\n"), "", "L", false)
			x += widths[i]
		}
		pdf.SetXY(marginLeft, y+rowH)
	}
	pdf.Ln(2)
}

// groupedHeader is one top-row header cell for tableGrouped. A column with
// no Sub spans both header rows (a plain rowspan-2 header). A column with
// 2+ Sub entries spans len(Sub) consecutive width columns in the top row
// (colspan) and shows one sub-label per column in the second row — e.g.
// "Thời gian dừng (h)" over "PX (h)" / "TB (h)".
type groupedHeader struct {
	Label string
	Sub   []string
}

// tableGrouped is `table` with a 2-row header supporting merged/grouped
// column headers (rowspan for ungrouped columns, colspan+sub-labels for
// grouped ones). Data rows reuse the exact same word-wrap/page-break logic
// as `table`; `widths` is still one entry per *final* flat column (a grouped
// header's Sub length must match how many width entries it covers).
func (b *builder) tableGrouped(groups []groupedHeader, widths []float64, rows [][]string, emptyMsg ...string) {
	pdf := b.pdf
	if len(rows) == 0 {
		msg := "Các thiết bị hoạt động ổn định."
		if len(emptyMsg) > 0 {
			msg = emptyMsg[0]
		}
		pdf.SetFont("Times", "I", 10)
		pdf.CellFormat(usableWidth, 6, msg, "1", 1, "C", false, 0, "")
		pdf.SetFont("Times", "", 11)
		return
	}
	const headerH = 7.0
	drawHeader := func() {
		pdf.SetFont("Times", "B", 9)
		pdf.SetFillColor(224, 224, 224)
		x0, y0 := pdf.GetX(), pdf.GetY()
		x, wi := x0, 0
		for _, g := range groups {
			if len(g.Sub) < 2 {
				w := widths[wi]
				pdf.Rect(x, y0, w, headerH*2, "DF")
				pdf.SetXY(x, y0+headerH-2.5)
				pdf.CellFormat(w, 5, g.Label, "", 0, "C", false, 0, "")
				x += w
				wi++
				continue
			}
			span := len(g.Sub)
			gw := 0.0
			for k := 0; k < span; k++ {
				gw += widths[wi+k]
			}
			pdf.Rect(x, y0, gw, headerH, "DF")
			pdf.SetXY(x, y0+1)
			pdf.CellFormat(gw, 5, g.Label, "", 0, "C", false, 0, "")
			sx := x
			for k := 0; k < span; k++ {
				pdf.Rect(sx, y0+headerH, widths[wi+k], headerH, "DF")
				pdf.SetXY(sx, y0+headerH+1)
				pdf.CellFormat(widths[wi+k], 5, g.Sub[k], "", 0, "C", false, 0, "")
				sx += widths[wi+k]
			}
			x += gw
			wi += span
		}
		pdf.SetXY(x0, y0+headerH*2)
		pdf.SetFont("Times", "", 9)
	}
	if pdf.GetY()+20+headerH > pageBottom {
		pdf.AddPage()
	}
	drawHeader()

	for _, row := range rows {
		maxLines := 1
		cellLines := make([][]string, len(row))
		for i, cell := range row {
			raw := pdf.SplitLines([]byte(cell), widths[i]-2)
			var lines []string
			for _, l := range raw {
				lines = append(lines, string(l))
			}
			if len(lines) == 0 {
				lines = []string{""}
			}
			cellLines[i] = lines
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		rowH := float64(maxLines)*4.2 + 1.5

		if pdf.GetY()+rowH > pageBottom {
			pdf.AddPage()
			drawHeader()
		}
		x, y := pdf.GetX(), pdf.GetY()
		for i := range row {
			pdf.Rect(x, y, widths[i], rowH, "D")
			pdf.SetXY(x+1, y+0.75)
			pdf.MultiCell(widths[i]-2, 4.2, strings.Join(cellLines[i], "\n"), "", "L", false)
			x += widths[i]
		}
		pdf.SetXY(marginLeft, y+rowH)
	}
	pdf.Ln(2)
}

func fmtFloat(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

// fmtFloatOrZero is fmtFloat with "0" instead of blank for a missing value —
// used only for the KPI table, where an un-entered figure should read as 0
// rather than an empty cell.
func fmtFloatOrZero(f *float64) string {
	if f == nil {
		return "0"
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

func fmtDate(d models.NullDate) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("02/01/2006")
}

// ---- section renderers ----

// kpiCategories are the 5 fixed KPI rows the report always shows, matching
// the web KPI tab's KPI_CATEGORIES exactly — the PDF renders all 5 every
// day regardless of what was actually saved, defaulting a missing figure to
// 0 and a missing Đánh giá to "Đạt" rather than leaving a blank/missing row.
// GroupSize is set only on the first row of a Nhóm group (how many rows
// that group's TT/Nhóm cells should span); the rows after it are 0.
var kpiCategories = []struct {
	TT        int
	ChiTieu   string
	Nhom      string
	DVT       string
	GroupSize int
}{
	{1, "Sự cố", "Sự cố", "Vụ", 3},
	{1, "Near Miss", "Sự cố", "Vụ", 0},
	{1, "Technical event", "Sự cố", "Vụ", 0},
	{2, "Rework", "Chất lượng", "WO", 1},
	{3, "Dừng ngoài kế hoạch", "Hỏng thiết bị", "Vụ", 1},
}

// sectionKPI reproduces the exact merged-cell layout of the Word KPI table:
// TT/Nhóm merged vertically per group, and a "Hôm nay" header spanning the
// KH/TH sub-columns — drawn by hand since neither `table` nor `tableGrouped`
// merge cells across multiple *data* rows.
func (b *builder) sectionKPI(rows []models.KPIEntry) {
	b.heading("1. CÁC CHỈ SỐ KPI")
	pdf := b.pdf
	byChiTieu := map[string]models.KPIEntry{}
	for _, k := range rows {
		byChiTieu[k.ChiTieu] = k
	}

	// TT, Nhóm, Chỉ tiêu, ĐVT, KH, TH, Lũy kế tháng, Đánh giá — sums to usableWidth.
	widths := []float64{10, 25, 38, 14, 20, 20, 28, 35}
	const headerH = 7.0
	const rowH = 6.5

	drawHeader := func() {
		pdf.SetFont("Times", "B", 9)
		pdf.SetFillColor(224, 224, 224)
		x0, y0 := pdf.GetX(), pdf.GetY()
		x := x0
		for i, lbl := range []string{"TT", "Nhóm", "Chỉ tiêu", "ĐVT"} {
			pdf.Rect(x, y0, widths[i], headerH*2, "DF")
			pdf.SetXY(x, y0+headerH-2.5)
			pdf.CellFormat(widths[i], 5, lbl, "", 0, "C", false, 0, "")
			x += widths[i]
		}
		gw := widths[4] + widths[5]
		pdf.Rect(x, y0, gw, headerH, "DF")
		pdf.SetXY(x, y0+1)
		pdf.CellFormat(gw, 5, "Hôm nay", "", 0, "C", false, 0, "")
		pdf.Rect(x, y0+headerH, widths[4], headerH, "DF")
		pdf.SetXY(x, y0+headerH+1)
		pdf.CellFormat(widths[4], 5, "KH", "", 0, "C", false, 0, "")
		pdf.Rect(x+widths[4], y0+headerH, widths[5], headerH, "DF")
		pdf.SetXY(x+widths[4], y0+headerH+1)
		pdf.CellFormat(widths[5], 5, "TH", "", 0, "C", false, 0, "")
		x += gw
		for i, lbl := range []string{"Lũy kế tháng", "Đánh giá"} {
			w := widths[6+i]
			pdf.Rect(x, y0, w, headerH*2, "DF")
			pdf.SetXY(x, y0+headerH-2.5)
			pdf.CellFormat(w, 5, lbl, "", 0, "C", false, 0, "")
			x += w
		}
		pdf.SetXY(x0, y0+headerH*2)
		pdf.SetFont("Times", "", 9)
	}

	if pdf.GetY()+20+headerH > pageBottom {
		pdf.AddPage()
	}
	drawHeader()

	i := 0
	for i < len(kpiCategories) {
		cat := kpiCategories[i]
		groupSize := cat.GroupSize
		if groupSize == 0 {
			groupSize = 1
		}
		groupH := float64(groupSize) * rowH
		if pdf.GetY()+groupH > pageBottom {
			pdf.AddPage()
			drawHeader()
		}
		x, y := pdf.GetX(), pdf.GetY()

		pdf.Rect(x, y, widths[0], groupH, "D")
		pdf.SetXY(x, y+groupH/2-2.5)
		pdf.CellFormat(widths[0], 5, strconv.Itoa(cat.TT), "", 0, "C", false, 0, "")
		pdf.Rect(x+widths[0], y, widths[1], groupH, "D")
		pdf.SetXY(x+widths[0], y+groupH/2-2.5)
		pdf.CellFormat(widths[1], 5, cat.Nhom, "", 0, "C", false, 0, "")

		for gi := 0; gi < groupSize; gi++ {
			c := kpiCategories[i+gi]
			k, ok := byChiTieu[c.ChiTieu]
			dvt := c.DVT
			if ok && k.DVT != "" {
				dvt = k.DVT
			}
			danhGia := ""
			if ok {
				danhGia = k.DanhGia
			}
			ry := y + float64(gi)*rowH
			cx := x + widths[0] + widths[1]
			vals := []string{c.ChiTieu, dvt, fmtFloatOrZero(k.KeHoach), fmtFloatOrZero(k.ThucHien), fmtFloatOrZero(k.LuyKeThang), danhGia}
			for vi, val := range vals {
				w := widths[2+vi]
				pdf.Rect(cx, ry, w, rowH, "D")
				pdf.SetXY(cx, ry+rowH/2-2.5)
				pdf.CellFormat(w, 5, val, "", 0, "C", false, 0, "")
				cx += w
			}
		}

		pdf.SetXY(x, y+groupH)
		i += groupSize
	}
	pdf.Ln(2)
}

func (b *builder) sectionSuCo(rows []models.SuCo) {
	b.heading("2. SỰ CỐ, VẤN ĐỀ KỸ THUẬT ẢNH HƯỞNG VHSX")
	groups := []groupedHeader{
		{Label: "TT"}, {Label: "Thiết bị"}, {Label: "DM"}, {Label: "Tình trạng"},
		{Label: "Thời gian dừng (h)", Sub: []string{"PX (h)", "TB (h)"}},
		{Label: "Phương án"}, {Label: "Đơn vị"}, {Label: "Thời hạn"},
	}
	widths := []float64{8, 26, 16, 40, 14, 14, 38, 20, 14}
	var data [][]string
	for _, s := range rows {
		data = append(data, []string{
			strconv.Itoa(s.TT), s.ThietBi, s.DM, s.TinhTrang,
			fmtFloat(s.ThoiGianDungPX), fmtFloat(s.ThoiGianDungTB), s.PhuongAn, s.DonVi, fmtDate(s.ThoiHan),
		})
	}
	b.tableGrouped(groups, widths, data, "Không có sự cố/vấn đề kỹ thuật ảnh hưởng vận hành.")
}

func (b *builder) sectionCBM(rows []models.CBMRow) {
	b.subheading("3.1. Giám sát trực tuyến (online monitoring)")
	headers := []string{"TT", "Thời gian", "Tagname", "Thiết bị", "Mô tả", "Giá trị cảnh báo", "Mức cảnh báo"}
	widths := []float64{8, 20, 26, 32, 50, 27, 27}
	var data [][]string
	for i, c := range rows {
		data = append(data, []string{strconv.Itoa(i + 1), c.DataTime.Format("15:04 02/01"), c.Tagname, c.Equipment, c.Description, c.AlarmValue, c.AlarmLevel})
	}
	b.table(headers, widths, data)
}

func (b *builder) sectionGiamSat(rows []models.GiamSat) {
	b.subheading("3.2. Chẩn đoán bằng tool chuyên dụng")
	headers := []string{"TT", "Thiết bị", "DM", "Tình trạng", "Rủi ro", "Phương án", "Đơn vị", "Thời hạn"}
	widths := []float64{8, 24, 12, 40, 30, 42, 20, 14}
	var data [][]string
	for _, s := range rows {
		data = append(data, []string{strconv.Itoa(s.TT), s.ThietBi, s.DM, s.TinhTrang, s.RuiRo, s.PhuongAn, s.DonVi, fmtDate(s.ThoiHan)})
	}
	b.table(headers, widths, data)
}

func (b *builder) sectionRSM(rows []models.RSMRow) {
	b.subheading("3.3. Vấn đề khác liên quan tới độ tin cậy và toàn vẹn thiết bị")
	headers := []string{"Phòng", "Thiết bị", "Vấn đề/Ghi nhận", "Phương án xử lý", "Đơn vị", "Hạn xử lý", "Trạng thái"}
	widths := []float64{14, 30, 45, 55, 20, 16, 10}
	var data [][]string
	for _, r := range rows {
		data = append(data, []string{r.Depart, r.TagName, r.Issue, r.MitigationPlan, r.ActionBy, fmtRSMDate(r.DueDate), r.Status})
	}
	b.table(headers, widths, data)
}

// fmtRSMDate reformats an RSM row's YYYY-MM-DD due date (already
// normalized by internal/rsm) into the same DD/MM/YYYY style every other
// date in this report uses.
func fmtRSMDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("02/01/2006")
}

func (b *builder) sectionCongViec(rows []models.CongViec) {
	b.heading("4. KẾT QUẢ THỰC HIỆN CÔNG VIỆC BDSC QUAN TRỌNG")
	headers := []string{"TT", "Thiết bị", "DM", "Nội dung", "Kế hoạch", "Tiến độ (%)", "Hôm nay", "Ngày mai"}
	widths := []float64{8, 18, 12, 44, 25, 18, 43, 22}
	var data [][]string
	for _, c := range rows {
		data = append(data, []string{strconv.Itoa(c.TT), c.ThietBi, c.DM, c.NoiDung, c.KeHoach, fmtFloat(c.TienDo), c.HomNay, c.NgayMai})
	}
	b.table(headers, widths, data)
}

func (b *builder) sectionDeXuat(suco []models.SuCo, giamsat []models.GiamSat, congviec []models.CongViec) {
	b.heading("5. VẤN ĐỀ CẦN ĐỀ XUẤT")
	headers := []string{"TT", "Thiết bị", "Đề xuất"}
	widths := []float64{10, 45, 135}
	var data [][]string
	for _, it := range CombineDeXuat(suco, giamsat, congviec) {
		data = append(data, []string{strconv.Itoa(it.TT), it.ThietBi, it.Note})
	}
	b.table(headers, widths, data, "Không có đề xuất.")
}

func (b *builder) sectionLuuY(suco []models.SuCo, giamsat []models.GiamSat, congviec []models.CongViec) {
	b.heading("6. LƯU Ý QUAN TRỌNG")
	headers := []string{"TT", "Thiết bị", "Lưu ý"}
	widths := []float64{10, 45, 135}
	var data [][]string
	for _, it := range CombineLuuY(suco, giamsat, congviec) {
		data = append(data, []string{strconv.Itoa(it.TT), it.ThietBi, it.Note})
	}
	b.table(headers, widths, data)
}

// sectionImages renders the same 3-column table (TT | Nội dung | Hình ảnh)
// as the original Word report — one bordered row per photo, with the actual
// image fitted (aspect-preserved) inside its cell, rather than a loose grid.
func (b *builder) sectionImages(rows []models.CongViec) {
	type photo struct {
		path    string
		thietBi string
	}
	var photos []photo
	for _, c := range rows {
		for _, a := range c.Attachments {
			full := filepath.Join(b.uploadsDir, filepath.FromSlash(a.FilePath))
			if _, err := os.Stat(full); err != nil {
				continue
			}
			photos = append(photos, photo{path: full, thietBi: c.ThietBi})
		}
	}
	// Nếu mục 6 đã đi quá nửa trang, sang trang mới cho mục 7 đỡ bị vỡ
	// header (không đủ chỗ cho ảnh đầu tiên); còn dưới nửa trang thì nối
	// tiếp luôn, đỡ lãng phí giấy.
	if b.pdf.GetY() > pageTop+(pageBottom-pageTop)/2 {
		b.pdf.AddPage()
	}
	b.heading("7. MỘT SỐ HÌNH ẢNH THỰC HIỆN CÔNG VIỆC TẠI HIỆN TRƯỜNG")
	pdf := b.pdf
	if len(photos) == 0 {
		pdf.SetFont("Times", "I", 10)
		pdf.CellFormat(usableWidth, 6, "(Không có dữ liệu)", "1", 1, "C", false, 0, "")
		pdf.SetFont("Times", "", 11)
		return
	}

	// Group consecutive photos that share the same Thiết bị so their TT and
	// Nội dung cells can be merged into one spanning cell, matching how the
	// original Word table looked when several photos belonged to the same
	// work item.
	type group struct {
		thietBi string
		photos  []photo
	}
	var groups []group
	for _, p := range photos {
		if n := len(groups); n > 0 && groups[n-1].thietBi == p.thietBi {
			groups[n-1].photos = append(groups[n-1].photos, p)
		} else {
			groups = append(groups, group{thietBi: p.thietBi, photos: []photo{p}})
		}
	}

	colTT, colThiet := 12.0, 38.0
	colImg := usableWidth - colTT - colThiet
	rowH := 55.0

	drawHeader := func() {
		pdf.SetFont("Times", "B", 9)
		pdf.SetFillColor(224, 224, 224)
		pdf.CellFormat(colTT, 7, "TT", "1", 0, "C", true, 0, "")
		pdf.CellFormat(colThiet, 7, "Nội dung", "1", 0, "C", true, 0, "")
		pdf.CellFormat(colImg, 7, "Hình ảnh", "1", 1, "C", true, 0, "")
		pdf.SetFont("Times", "", 9)
	}
	// Dùng chiều cao của cả nhóm ảnh đầu tiên (không phải chỉ 1 hàng) để
	// quyết định có cần sang trang trước khi vẽ header hay không — tránh vẽ
	// header xong rồi mới phát hiện cả nhóm không đủ chỗ, để lại header mồ
	// côi + khoảng trắng như cũ.
	firstGroupH := rowH
	if len(groups) > 0 {
		firstGroupH = float64(len(groups[0].photos)) * rowH
	}
	if pdf.GetY()+7+firstGroupH > pageBottom {
		pdf.AddPage()
	}
	drawHeader()

	for gi, g := range groups {
		groupH := float64(len(g.photos)) * rowH
		if pdf.GetY()+groupH > pageBottom {
			pdf.AddPage()
			drawHeader()
		}
		x, y := pdf.GetX(), pdf.GetY()

		pdf.Rect(x, y, colTT, groupH, "D")
		pdf.Rect(x+colTT, y, colThiet, groupH, "D")
		pdf.SetXY(x, y+groupH/2-2.5)
		pdf.CellFormat(colTT, 5, strconv.Itoa(gi+1), "", 0, "C", false, 0, "")
		thietBiLines := pdf.SplitLines([]byte(g.thietBi), colThiet-3)
		thietBiH := float64(len(thietBiLines)) * 5.0
		pdf.SetXY(x+colTT+1.5, y+(groupH-thietBiH)/2)
		pdf.MultiCell(colThiet-3, 5, g.thietBi, "", "L", false)

		for pi, p := range g.photos {
			py := y + float64(pi)*rowH
			pdf.Rect(x+colTT+colThiet, py, colImg, rowH, "D")

			imgType, data, prepErr := preparePDFImage(p.path)
			if prepErr != nil {
				pdf.SetXY(x+colTT+colThiet, py+rowH/2-2.5)
				pdf.SetFont("Times", "I", 9)
				pdf.CellFormat(colImg, 5, "(Không hiển thị được ảnh này)", "", 0, "C", false, 0, "")
				pdf.SetFont("Times", "", 9)
				continue
			}
			iw, ih := colImg-6, rowH-6
			if info := pdf.RegisterImageOptionsReader(p.path, fpdf.ImageOptions{ImageType: imgType}, bytes.NewReader(data)); info != nil {
				if w0, h0 := info.Extent(); w0 > 0 && h0 > 0 {
					scale := iw / w0
					if ih/h0 < scale {
						scale = ih / h0
					}
					iw, ih = w0*scale, h0*scale
				}
			}
			imgX := x + colTT + colThiet + (colImg-iw)/2
			imgY := py + (rowH-ih)/2
			pdf.ImageOptions(p.path, imgX, imgY, iw, ih, false, fpdf.ImageOptions{ImageType: imgType}, 0, "")
		}

		pdf.SetXY(x, y+groupH)
	}
	pdf.Ln(2)
}

// Generate builds the full daily-report PDF for the window [start, end] and
// writes it to outDir, returning the file path.
func Generate(cfg BuildInput) (string, error) {
	b := newBuilder()
	b.uploadsDir = cfg.UploadsDir

	// The window [start, end] spans yesterday 08:01 -> today 07:59, so the
	// data is really "yesterday's" activity even though the report is
	// generated/sent this morning — the printed date reflects that.
	reportDate := cfg.RunAt.AddDate(0, 0, -1)
	dispStart, dispEnd := window.DisplayBounds(cfg.Start, cfg.End, cfg.ReportHour, cfg.ReportMinute)
	b.letterhead(reportDate.Format("02/01/2006"), dispStart.Format("15:04 02/01"), dispEnd.Format("15:04 02/01"))

	b.sectionKPI(cfg.KPI)
	b.sectionSuCo(cfg.SuCo)
	b.heading("3. KẾT QUẢ GIÁM SÁT, CHẨN ĐOÁN THIẾT BỊ")
	b.sectionCBM(cfg.CBM)
	b.pdf.Ln(2)
	b.sectionGiamSat(cfg.GiamSat)
	b.pdf.Ln(2)
	b.sectionRSM(cfg.RSM)
	b.sectionCongViec(cfg.CongViec)
	b.sectionDeXuat(cfg.SuCo, cfg.GiamSat, cfg.CongViec)
	b.sectionLuuY(cfg.SuCo, cfg.GiamSat, cfg.CongViec)
	b.sectionImages(cfg.CongViec)
	b.signatureBlock(cfg.ReviewedByName, cfg.ApprovedByName)

	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("BaoCaoNgayBDSC_%s.pdf", reportDate.Format("2006-01-02"))
	outPath := filepath.Join(cfg.OutDir, fileName)
	if err := b.pdf.OutputFileAndClose(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

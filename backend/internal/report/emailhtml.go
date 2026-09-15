package report

import (
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"bdscdailyreport/internal/models"
)

// RenderEmailHTML builds a self-contained HTML summary of the report for
// the body of the daily email — every section except the "Người lập / Phê
// duyệt" signature block, including the actual work photos (reusing the
// exact same compressed JPEG bytes already produced for the PDF, per
// preparePDFImage). The PDF itself is no longer attached separately; the
// email links straight to its SharePoint-synced copy at pdfLinkURL instead.
//
// Only inline styles and plain <table>/<tr>/<td> are used: Outlook renders
// email HTML with Word's layout engine, which ignores flexbox/grid and
// most modern CSS that the web app's own styling relies on.
func RenderEmailHTML(data BuildInput, pdfLinkURL string) string {
	var b strings.Builder
	reportDate := data.RunAt.AddDate(0, 0, -1)
	linkLabel := "Báo cáo công tác BDSC ngày " + reportDate.Format("02/01/2006")

	b.WriteString(`<div style="font-family:Arial,sans-serif;font-size:14px;color:#222;">`)
	fmt.Fprintf(&b, `<p>Kính gửi: Ban Giám đốc Nhà máy,</p>`)
	fmt.Fprintf(&b, `<p>Ban BDSC xin gửi báo cáo công tác bảo dưỡng sửa chữa ngày %s như bên dưới:</p>`,
		html.EscapeString(reportDate.Format("02/01/2006")))
	fmt.Fprintf(&b, `<ul><li><a href="%s">%s</a></li></ul>`, html.EscapeString(pdfLinkURL), html.EscapeString(linkLabel))

	emailKPISection(&b, data.KPI)
	emailGiamSatSection(&b, data.CBM, data.GiamSat, data.Reliability)
	emailSuCoSection(&b, data.SuCo)
	emailCongViecSection(&b, data.CongViec)
	emailNotesSection(&b, "5. VẤN ĐỀ CẦN ĐỀ XUẤT", "Đề xuất", "Không có đề xuất.", CombineDeXuat(data.SuCo, data.GiamSat, data.CongViec))
	emailNotesSection(&b, "6. LƯU Ý QUAN TRỌNG", "Lưu ý", "Các thiết bị hoạt động ổn định.", CombineLuuY(data.SuCo, data.GiamSat, data.CongViec))
	emailImagesSection(&b, data)

	b.WriteString(`</div>`)
	return b.String()
}

// htmlText escapes user-entered text and turns embedded newlines into <br>
// so multi-line fields (Tình trạng, Phương án, Nội dung...) still show as
// multiple lines instead of collapsing to one, matching how the web app's
// own tables preserve line breaks.
func htmlText(s string) string {
	return strings.ReplaceAll(html.EscapeString(s), "\n", "<br>")
}

func emailTableStart() string {
	return `<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse;width:100%;font-size:13px;margin-bottom:4px;">`
}

func emailHeading(b *strings.Builder, text string) {
	fmt.Fprintf(b, `<h3 style="margin:18px 0 6px;">%s</h3>`, html.EscapeString(text))
}

func emailEmptyMsg(b *strings.Builder, msg string) {
	fmt.Fprintf(b, `<p style="font-style:italic;color:#555;">%s</p>`, html.EscapeString(msg))
}

func emailKPISection(b *strings.Builder, rows []models.KPIEntry) {
	byChiTieu := map[string]models.KPIEntry{}
	for _, k := range rows {
		byChiTieu[k.ChiTieu] = k
	}
	emailHeading(b, "1. CÁC CHỈ SỐ KPI")
	b.WriteString(emailTableStart())
	b.WriteString(`<tr style="background:#e0e0e0;"><th rowspan="2">TT</th><th rowspan="2">Nhóm</th><th rowspan="2">Chỉ tiêu</th><th rowspan="2">ĐVT</th><th colspan="2">Hôm nay</th><th rowspan="2">Lũy kế tháng</th><th rowspan="2">Đánh giá</th></tr>`)
	b.WriteString(`<tr style="background:#e0e0e0;"><th>KH</th><th>TH</th></tr>`)

	i := 0
	for i < len(kpiCategories) {
		cat := kpiCategories[i]
		groupSize := cat.GroupSize
		if groupSize == 0 {
			groupSize = 1
		}
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
			b.WriteString(`<tr>`)
			if gi == 0 {
				fmt.Fprintf(b, `<td rowspan="%d" style="text-align:center;">%d</td><td rowspan="%d">%s</td>`,
					groupSize, cat.TT, groupSize, htmlText(cat.Nhom))
			}
			fmt.Fprintf(b, `<td>%s</td><td style="text-align:center;">%s</td><td style="text-align:center;">%s</td><td style="text-align:center;">%s</td><td style="text-align:center;">%s</td><td style="text-align:center;">%s</td>`,
				htmlText(c.ChiTieu), htmlText(dvt), fmtFloatOrZero(k.KeHoach), fmtFloatOrZero(k.ThucHien), fmtFloatOrZero(k.LuyKeThang), htmlText(danhGia))
			b.WriteString(`</tr>`)
		}
		i += groupSize
	}
	b.WriteString(`</table>`)
}

func emailSuCoSection(b *strings.Builder, rows []models.SuCo) {
	emailHeading(b, "3. SỰ CỐ, VẤN ĐỀ KỸ THUẬT ẢNH HƯỞNG VHSX")
	if len(rows) == 0 {
		emailEmptyMsg(b, "Không có sự cố/vấn đề kỹ thuật ảnh hưởng vận hành.")
		return
	}
	b.WriteString(emailTableStart())
	b.WriteString(`<tr style="background:#e0e0e0;"><th rowspan="2">TT</th><th rowspan="2">Thiết bị</th><th rowspan="2">DM</th><th rowspan="2">Tình trạng</th><th colspan="2">Thời gian dừng (h)</th><th rowspan="2">Phương án</th><th rowspan="2">Đơn vị</th><th rowspan="2">Thời hạn</th></tr>`)
	b.WriteString(`<tr style="background:#e0e0e0;"><th>PX (h)</th><th>TB (h)</th></tr>`)
	for _, s := range rows {
		fmt.Fprintf(b, `<tr><td style="text-align:center;">%d</td><td>%s</td><td style="text-align:center;">%s</td><td>%s</td><td style="text-align:center;">%s</td><td style="text-align:center;">%s</td><td>%s</td><td>%s</td><td style="text-align:center;">%s</td></tr>`,
			s.TT, htmlText(s.ThietBi), htmlText(s.DM), htmlText(s.TinhTrang), fmtFloat(s.ThoiGianDungPX), fmtFloat(s.ThoiGianDungTB), htmlText(s.PhuongAn), htmlText(s.DonVi), fmtDate(s.ThoiHan))
	}
	b.WriteString(`</table>`)
}

func emailGiamSatSection(b *strings.Builder, cbm []models.CBMRow, giamsat []models.GiamSat, reliability []models.Reliability) {
	emailHeading(b, "2. KẾT QUẢ GIÁM SÁT, CHẨN ĐOÁN THIẾT BỊ")
	fmt.Fprintf(b, `<p style="margin:6px 0;font-weight:bold;">2.1. Giám sát trực tuyến (online monitoring)</p>`)
	if len(cbm) == 0 {
		emailEmptyMsg(b, "Các thiết bị hoạt động ổn định.")
	} else {
		b.WriteString(emailTableStart())
		b.WriteString(`<tr style="background:#e0e0e0;"><th>TT</th><th>Thời gian</th><th>Tagname</th><th>Thiết bị</th><th>Mô tả</th><th>Giá trị cảnh báo</th><th>Mức cảnh báo</th></tr>`)
		for i, c := range cbm {
			fmt.Fprintf(b, `<tr><td style="text-align:center;">%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				i+1, c.DataTime.Format("15:04 02/01"), htmlText(c.Tagname), htmlText(c.Equipment), htmlText(c.Description), htmlText(c.AlarmValue), htmlText(c.AlarmLevel))
		}
		b.WriteString(`</table>`)
	}
	fmt.Fprintf(b, `<p style="margin:12px 0 6px;font-weight:bold;">2.2. Giám sát offline bằng thiết bị chuyên dụng/ phát hiện tại hiện trường</p>`)
	if len(giamsat) == 0 {
		emailEmptyMsg(b, "Các thiết bị hoạt động ổn định.")
	} else {
		b.WriteString(emailTableStart())
		b.WriteString(`<tr style="background:#e0e0e0;"><th>TT</th><th>Thiết bị</th><th>DM</th><th>Tình trạng</th><th>Rủi ro</th><th>Phương án</th><th>Đơn vị</th><th>Thời hạn</th></tr>`)
		for _, s := range giamsat {
			fmt.Fprintf(b, `<tr><td style="text-align:center;">%d</td><td>%s</td><td style="text-align:center;">%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td style="text-align:center;">%s</td></tr>`,
				s.TT, htmlText(s.ThietBi), htmlText(s.DM), htmlText(s.TinhTrang), htmlText(s.RuiRo), htmlText(s.PhuongAn), htmlText(s.DonVi), fmtDate(s.ThoiHan))
		}
		b.WriteString(`</table>`)
	}
	fmt.Fprintf(b, `<p style="margin:12px 0 6px;font-weight:bold;">2.3. VẤN ĐỀ KỸ THUẬT ẢNH HƯỞNG ĐỘ TIN CẬY VÀ TOÀN VẸN CƠ KHÍ</p>`)
	if len(reliability) == 0 {
		emailEmptyMsg(b, "Các thiết bị hoạt động ổn định.")
		return
	}
	b.WriteString(emailTableStart())
	b.WriteString(`<tr style="background:#e0e0e0;"><th>TT</th><th>Thiết bị</th><th>Vấn đề phát hiện</th><th>Phương án xử lý</th><th>Đơn vị thực hiện</th><th>Thời hạn</th></tr>`)
	for _, r := range reliability {
		fmt.Fprintf(b, `<tr><td style="text-align:center;">%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td style="text-align:center;">%s</td></tr>`,
			r.TT, htmlText(r.ThietBi), htmlText(r.VanDe), htmlText(r.PhuongAn), htmlText(r.DonVi), fmtDate(r.ThoiHan))
	}
	b.WriteString(`</table>`)
}

func emailCongViecSection(b *strings.Builder, rows []models.CongViec) {
	emailHeading(b, "4. KẾT QUẢ THỰC HIỆN CÔNG VIỆC BDSC QUAN TRỌNG")
	if len(rows) == 0 {
		emailEmptyMsg(b, "Các thiết bị hoạt động ổn định.")
		return
	}
	b.WriteString(emailTableStart())
	b.WriteString(`<tr style="background:#e0e0e0;"><th>TT</th><th>Thiết bị</th><th>DM</th><th>Nội dung</th><th>Kế hoạch</th><th>Tiến độ (%)</th><th>Hôm nay</th><th>Ngày mai</th></tr>`)
	for _, c := range rows {
		fmt.Fprintf(b, `<tr><td style="text-align:center;">%d</td><td>%s</td><td style="text-align:center;">%s</td><td>%s</td><td>%s</td><td style="text-align:center;">%s</td><td>%s</td><td>%s</td></tr>`,
			c.TT, htmlText(c.ThietBi), htmlText(c.DM), htmlText(c.NoiDung), htmlText(c.KeHoach), fmtFloat(c.TienDo), htmlText(c.HomNay), htmlText(c.NgayMai))
	}
	b.WriteString(`</table>`)
}

func emailNotesSection(b *strings.Builder, heading, colLabel, emptyMsg string, items []NoteItem) {
	emailHeading(b, heading)
	if len(items) == 0 {
		emailEmptyMsg(b, emptyMsg)
		return
	}
	b.WriteString(emailTableStart())
	fmt.Fprintf(b, `<tr style="background:#e0e0e0;"><th>TT</th><th>Thiết bị</th><th>%s</th></tr>`, html.EscapeString(colLabel))
	for _, it := range items {
		fmt.Fprintf(b, `<tr><td style="text-align:center;">%d</td><td>%s</td><td>%s</td></tr>`, it.TT, htmlText(it.ThietBi), htmlText(it.Note))
	}
	b.WriteString(`</table>`)
}

func emailImagesSection(b *strings.Builder, data BuildInput) {
	type photo struct {
		path    string
		thietBi string
	}
	var photos []photo
	for _, c := range data.CongViec {
		for _, a := range c.Attachments {
			full := filepath.Join(data.UploadsDir, filepath.FromSlash(a.FilePath))
			if _, err := os.Stat(full); err != nil {
				continue
			}
			photos = append(photos, photo{path: full, thietBi: c.ThietBi})
		}
	}
	emailHeading(b, "7. MỘT SỐ HÌNH ẢNH THỰC HIỆN CÔNG VIỆC TẠI HIỆN TRƯỜNG")
	if len(photos) == 0 {
		emailEmptyMsg(b, "(Không có dữ liệu)")
		return
	}

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

	b.WriteString(emailTableStart())
	b.WriteString(`<tr style="background:#e0e0e0;"><th>TT</th><th>Nội dung</th><th>Hình ảnh</th></tr>`)
	for gi, g := range groups {
		for pi, p := range g.photos {
			b.WriteString(`<tr>`)
			if pi == 0 {
				fmt.Fprintf(b, `<td rowspan="%d" style="text-align:center;">%d</td><td rowspan="%d">%s</td>`,
					len(g.photos), gi+1, len(g.photos), htmlText(g.thietBi))
			}
			imgType, imgData, err := preparePDFImage(p.path)
			if err != nil {
				b.WriteString(`<td style="text-align:center;color:#888;">(Không hiển thị được ảnh này)</td>`)
			} else {
				mime := "image/jpeg"
				if imgType == "PNG" {
					mime = "image/png"
				}
				fmt.Fprintf(b, `<td style="text-align:center;"><img src="data:%s;base64,%s" style="max-width:350px;max-height:260px;"></td>`,
					mime, base64.StdEncoding.EncodeToString(imgData))
			}
			b.WriteString(`</tr>`)
		}
	}
	b.WriteString(`</table>`)
}

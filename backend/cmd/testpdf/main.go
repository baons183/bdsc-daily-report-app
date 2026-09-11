// Throwaway program to sanity-check PDF rendering (fonts, tables, embedded
// images) without needing a real database. Not part of the served app.
package main

import (
	"fmt"
	"os"
	"time"

	"bdscdailyreport/internal/models"
	"bdscdailyreport/internal/report"
)

func main() {
	f := 12.5
	tienDo := 60.0
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	uploadsDir := "./uploads_test"
	os.MkdirAll(uploadsDir+"/congviec/1", 0o755)
	// copy a real sample image so Image() has something valid to embed
	copyFile(`D:\App\logo-BDSC.jpg`, uploadsDir+"/congviec/1/anh1.jpg")

	input := report.BuildInput{
		RunAt: now, Start: yesterday, End: now,
		KPI: []models.KPIEntry{
			{ChiTieu: "Sự cố", Nhom: "An toàn", DVT: "vụ", KeHoach: &f, ThucHien: &f, LuyKeThang: &f, DanhGia: "Đạt"},
		},
		SuCo: []models.SuCo{
			{TT: 1, ThietBi: "Bơm P-1201A", DM: "ROT", TinhTrang: "Rung cao bất thường ở gối đỡ số 2", ThoiGianDungPX: &f, ThoiGianDungTB: &f, PhuongAn: "Dừng kiểm tra, thay bạc đạn", DonVi: "Xưởng Cơ khí", DeXuat: "Đề nghị đặt hàng bạc đạn dự phòng", LuuY: "Theo dõi sát trong ca đêm"},
		},
		GiamSat: []models.GiamSat{
			{TT: 1, ThietBi: "Máy nén K-401", DM: "ROT", TinhTrang: "Nhiệt độ ổ trục tăng nhẹ", RuiRo: "Trung bình", PhuongAn: "Giám sát thêm 48h", DonVi: "Phòng Kỹ thuật"},
		},
		CongViec: []models.CongViec{
			{TT: 1, ThietBi: "Tháp T-301", DM: "STA", NoiDung: "Vệ sinh, kiểm tra ăn mòn bên trong tháp", KeHoach: "Hoàn thành 25/09", TienDo: &tienDo, HomNay: "Vệ sinh xong ngăn 1-5", NgayMai: "Tiếp tục ngăn 6-10", DeXuat: "N/A", LuuY: "Cẩn thận không gian hạn chế",
				Attachments: []models.Attachment{{FilePath: "congviec/1/anh1.jpg"}},
			},
		},
		CBM: []models.CBMRow{
			{Tagname: "VIB-1201A", Equipment: "Bơm P-1201A", Description: "Độ rung cao", AlarmValue: "8.2 mm/s", AlarmLevel: "Cảnh báo"},
		},
		UploadsDir: uploadsDir,
		OutDir:     "./generated_reports_test",
	}
	path, err := report.Generate(input)
	if err != nil {
		fmt.Println("LOI:", err)
		os.Exit(1)
	}
	fmt.Println("OK, da tao:", path)
}

func copyFile(src, dst string) {
	b, err := os.ReadFile(src)
	if err != nil {
		fmt.Println("khong doc duoc anh mau (bo qua phan anh):", err)
		return
	}
	os.WriteFile(dst, b, 0o644)
}

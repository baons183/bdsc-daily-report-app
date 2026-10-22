package models

import "time"

type User struct {
	Id           int
	EmployeeCode string
	FullName     string
	PasswordHash string
	Roles        string // comma-separated: SuCo,GiamSat,CongViec,KPI,Admin
}

type SuCo struct {
	Id             int       `json:"id"`
	TT             int       `json:"tt"`
	ThietBi        string    `json:"thietBi"`
	DM             string    `json:"dm"`
	LoaiSuCo       string    `json:"loaiSuCo"`
	TinhTrang      string    `json:"tinhTrang"`
	ThoiGianDungPX *float64  `json:"thoiGianDungPX"`
	ThoiGianDungTB *float64  `json:"thoiGianDungTB"`
	PhuongAn       string    `json:"phuongAn"`
	DonVi          string    `json:"donVi"`
	ThoiHan        NullDate  `json:"thoiHan"`
	DeXuat         string    `json:"deXuat"`
	LuuY           string    `json:"luuY"`
	NgayBaoCao     time.Time `json:"ngayBaoCao"`
	CreatedBy      string    `json:"createdBy"`
	TrangThai      string    `json:"trangThai"`
	// ReportDate is the printed report date (YYYY-MM-DD) NgayBaoCao falls
	// under (window.ReportDateForEntry) — filled in only by ListSuCo, for
	// the tab's "Ngày báo cáo" column.
	ReportDate string `json:"reportDate,omitempty"`
}

type GiamSat struct {
	Id         int       `json:"id"`
	TT         int       `json:"tt"`
	ThietBi    string    `json:"thietBi"`
	DM         string    `json:"dm"`
	TinhTrang  string    `json:"tinhTrang"`
	RuiRo      string    `json:"ruiRo"`
	PhuongAn   string    `json:"phuongAn"`
	DonVi      string    `json:"donVi"`
	ThoiHan    NullDate  `json:"thoiHan"`
	DeXuat     string    `json:"deXuat"`
	LuuY       string    `json:"luuY"`
	NgayBaoCao time.Time `json:"ngayBaoCao"`
	CreatedBy  string    `json:"createdBy"`
	TrangThai  string    `json:"trangThai"`
}

type Attachment struct {
	Id          int    `json:"id"`
	CongViecId  int    `json:"congViecId"`
	FileName    string `json:"fileName"`
	FilePath    string `json:"filePath"`
	ContentType string `json:"contentType"`
}

type CongViec struct {
	Id          int          `json:"id"`
	TT          int          `json:"tt"`
	ThietBi     string       `json:"thietBi"`
	DM          string       `json:"dm"`
	NoiDung     string       `json:"noiDung"`
	KeHoach     string       `json:"keHoach"`
	TienDo      *float64     `json:"tienDo"`
	HomNay      string       `json:"homNay"`
	NgayMai     string       `json:"ngayMai"`
	DeXuat      string       `json:"deXuat"`
	LuuY        string       `json:"luuY"`
	NgayBaoCao  time.Time    `json:"ngayBaoCao"`
	CreatedBy   string       `json:"createdBy"`
	Attachments []Attachment `json:"attachments"`
	TrangThai   string       `json:"trangThai"`
}

type KPIEntry struct {
	Id         int       `json:"id"`
	ChiTieu    string    `json:"chiTieu"`
	Nhom       string    `json:"nhom"`
	DVT        string    `json:"dvt"`
	KeHoach    *float64  `json:"keHoach"`
	ThucHien   *float64  `json:"thucHien"`
	LuyKeThang *float64  `json:"luyKeThang"`
	DanhGia    string    `json:"danhGia"`
	NgayBaoCao time.Time `json:"ngayBaoCao"`
	CreatedBy  string    `json:"createdBy"`
}

// CBMRow mirrors a row read from the daily-synced CBM Excel file (produced
// by the external online-monitoring system, unrelated to this app's own
// data entry).
type CBMRow struct {
	Tagname     string    `json:"tagname"`
	DataTime    time.Time `json:"dataTime"`
	Equipment   string    `json:"equipment"`
	Description string    `json:"description"`
	AlarmValue  string    `json:"alarmValue"`
	AlarmLevel  string    `json:"alarmLevel"`
}

// CBMRowKey identifies one deduped CBM alarm row (the same
// Tagname+Description+DataTime triple dedupeByTagnameEarliest collapses
// repeats down to) for the "đưa vào báo cáo" opt-in set (dbo.CBMInclusions)
// — a user ticking a row in the Giám sát trực tuyến tab records against
// this key, and report generation keeps only rows matching a recorded key.
// Shared by internal/db (storage) and internal/report (filtering) so both
// sides build the exact same string without importing each other.
func CBMRowKey(tagname, description string, dataTime time.Time) string {
	return tagname + "|" + description + "|" + dataTime.Format("2006-01-02 15:04:05")
}

// Reliability is one row of report section "3.3. Vấn đề khác liên quan
// tới độ tin cậy và toàn vẹn thiết bị" — a mix of rows a user typed in by
// hand and rows the system auto-imported from BSR's daily RSM Night Shift
// Report PDF (section IV.2, filtered to Ban BDSC's own departments; see
// internal/rsm and report.SyncReliabilityFromRSM). Once a row exists here
// — auto-imported or not — it behaves exactly like Sự cố/Giám sát/Công
// việc: editable and deletable like any other manually-entered row, and
// scoped to the same report window (NgayBaoCao) as everything else.
type Reliability struct {
	Id         int       `json:"id"`
	TT         int       `json:"tt"`
	ThietBi    string    `json:"thietBi"`
	VanDe      string    `json:"vanDe"`
	PhuongAn   string    `json:"phuongAn"`
	DonVi      string    `json:"donVi"`
	ThoiHan    NullDate  `json:"thoiHan"`
	NgayBaoCao time.Time `json:"ngayBaoCao"`
	CreatedBy  string    `json:"createdBy"`
	// Included is only meaningful for auto-imported (RSM PDF) rows — see
	// report.SyncReliabilityFromRSM — where deleting isn't possible (the
	// next sync would just reinsert it); a manually typed row keeps its own
	// Xoá button and this stays true for it always.
	Included  bool   `json:"included"`
	TrangThai string `json:"trangThai"`
}

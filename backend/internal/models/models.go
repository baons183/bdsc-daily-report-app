package models

import (
	"strings"
	"time"
)

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

// CBMExclusionKey identifies one deduped CBM alarm row (the same
// Tagname+Description+DataTime triple dedupeByTagnameEarliest collapses
// repeats down to) for exclusion purposes — a user unticking "đưa vào báo
// cáo" in the Giám sát trực tuyến tab records against this key, and report
// generation filters any row matching a recorded key back out. Shared by
// internal/db (storage) and internal/report (filtering) so both sides
// build the exact same string without importing each other.
func CBMExclusionKey(tagname, description string, dataTime time.Time) string {
	return tagname + "|" + description + "|" + dataTime.Format("2006-01-02 15:04:05")
}

// RSMRow is one row of section IV.2 (MECHANICAL INTEGRITY/EQUIPMENT
// RELIABILITY) of BSR's daily RSM Night Shift Report PDF, filtered down to
// Ban BDSC's own departments — rendered as report section "3.3. Vấn đề
// khác liên quan tới độ tin cậy và toàn vẹn thiết bị".
type RSMRow struct {
	Depart         string `json:"depart"`
	TagName        string `json:"tagName"`
	Issue          string `json:"issue"`
	MitigationPlan string `json:"mitigationPlan"`
	ActionBy       string `json:"actionBy"`
	DueDate        string `json:"dueDate"` // YYYY-MM-DD, or "" if the PDF's date didn't parse
	Status         string `json:"status"`
	// Included is only meaningful on the review/toggle listing (handlers.
	// GetRSMReview) — always true on report-facing data (report.
	// FetchRSMSection already dropped anything excluded).
	Included bool `json:"included"`
}

// RSMIssueKey identifies one IV.2 issue by its (Tag name, Issue) pair —
// the same problem is typically reported day after day until resolved,
// with only its Mitigation plan/Status narrative changing, so keying on
// these 2 fields (rather than including the report date) lets a user's
// "đưa vào báo cáo" tick/untick for one issue carry forward automatically
// across every day it keeps appearing, and keeps working unchanged
// whenever the data source switches from this PDF to a future data lake
// feed, since it depends only on the issue's own content.
func RSMIssueKey(tagName, issue string) string {
	return strings.TrimSpace(tagName) + "|" + strings.TrimSpace(issue)
}

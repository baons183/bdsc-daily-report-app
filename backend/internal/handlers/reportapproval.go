package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"bdscdailyreport/internal/emailer"
)

type reportApprovalResp struct {
	ReportDate     string `json:"reportDate"`
	Status         string `json:"status"` // draft | reviewed | approved
	ReviewedBy     string `json:"reviewedBy"`
	ReviewedByName string `json:"reviewedByName"`
	ReviewedAt     string `json:"reviewedAt"`
	ApprovedBy     string `json:"approvedBy"`
	ApprovedByName string `json:"approvedByName"`
	ApprovedAt     string `json:"approvedAt"`
}

func fetchReportApproval(e *Env, r *http.Request, reportDate string) (reportApprovalResp, error) {
	out := reportApprovalResp{ReportDate: reportDate, Status: "draft"}
	var reviewedBy, reviewedByName, approvedBy, approvedByName sql.NullString
	var reviewedAt, approvedAt sql.NullTime
	row := e.DB.QueryRowContext(r.Context(), `
SELECT Status, ISNULL(ReviewedBy,''), ISNULL(ReviewedByName,''), ReviewedAt, ISNULL(ApprovedBy,''), ISNULL(ApprovedByName,''), ApprovedAt
FROM dbo.ReportApprovals WHERE ReportDate = ?`, reportDate)
	err := row.Scan(&out.Status, &reviewedBy, &reviewedByName, &reviewedAt, &approvedBy, &approvedByName, &approvedAt)
	if err != nil {
		// no row yet => still draft, everything else blank
		return out, nil
	}
	out.ReviewedBy = reviewedBy.String
	out.ReviewedByName = reviewedByName.String
	if reviewedAt.Valid {
		out.ReviewedAt = reviewedAt.Time.Format("02/01/2006 15:04")
	}
	out.ApprovedBy = approvedBy.String
	out.ApprovedByName = approvedByName.String
	if approvedAt.Valid {
		out.ApprovedAt = approvedAt.Time.Format("02/01/2006 15:04")
	}
	return out, nil
}

// GetReportApproval returns the review/approval status of the given report
// date ("?date=YYYY-MM-DD") — any authenticated user can view it (needed
// to render the Kiểm tra/Phê duyệt buttons and the 2 signature boxes on
// the Thống kê tab).
func (e *Env) GetReportApproval(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		writeErr(w, 400, "thieu tham so date (YYYY-MM-DD)")
		return
	}
	out, err := fetchReportApproval(e, r, date)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, out)
}

type reportApprovalReq struct {
	Date string `json:"date"`
}

// ReviewReport is the Moderator's "Kiểm tra" action: only allowed while
// the report is still in "draft", locks it (checkNotLocked starts
// rejecting Create/Update/Delete on that date's SuCo/GiamSat/CongViec),
// stamps the caller's name into "NGƯỜI LẬP", and emails the Approvers.
func (e *Env) ReviewReport(w http.ResponseWriter, r *http.Request) {
	var in reportApprovalReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Date == "" {
		writeErr(w, 400, "thieu date")
		return
	}
	cur, err := fetchReportApproval(e, r, in.Date)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if cur.Status != "draft" {
		writeErr(w, 409, "bao cao nay da duoc kiem tra roi")
		return
	}
	user := currentUser(r)
	code, name := "", ""
	if user != nil {
		code, name = user.EmployeeCode, user.FullName
	}
	if err := upsertReview(e, r, in.Date, code, name); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	reportDate, _ := time.Parse("2006-01-02", in.Date)
	if err := emailer.SendApprovalReminder(e.EmailFlowApprovalURL, reportDate, name); err != nil {
		// Da luu trang thai thanh cong - loi gui email khong nen chan
		// nguoi dung, chi ghi nhan trong response de ho biet.
		writeJSON(w, 200, map[string]string{"ok": "true", "emailError": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ApproveReport is 1-of-2 Approvers' "Phê duyệt" action: only allowed
// while the report is "reviewed", stamps the caller's name into "PHÊ
// DUYỆT" and finalizes the report.
func (e *Env) ApproveReport(w http.ResponseWriter, r *http.Request) {
	var in reportApprovalReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Date == "" {
		writeErr(w, 400, "thieu date")
		return
	}
	cur, err := fetchReportApproval(e, r, in.Date)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if cur.Status != "reviewed" {
		writeErr(w, 409, "bao cao chua duoc kiem tra, chua the phe duyet")
		return
	}
	user := currentUser(r)
	code, name := "", ""
	if user != nil {
		code, name = user.EmployeeCode, user.FullName
	}
	if err := upsertApprove(e, r, in.Date, code, name); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// RejectReport is an Approver sending a "reviewed" report back to "draft"
// — clears the reviewer's name too, so the Moderator has to Kiểm tra lại
// from scratch after whatever gets fixed.
func (e *Env) RejectReport(w http.ResponseWriter, r *http.Request) {
	var in reportApprovalReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Date == "" {
		writeErr(w, 400, "thieu date")
		return
	}
	cur, err := fetchReportApproval(e, r, in.Date)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if cur.Status != "reviewed" {
		writeErr(w, 409, "bao cao khong o trang thai cho phe duyet")
		return
	}
	if _, err := e.DB.ExecContext(r.Context(), `
UPDATE dbo.ReportApprovals SET Status='draft', ReviewedBy=NULL, ReviewedByName=NULL, ReviewedAt=NULL, UpdatedAt=GETDATE()
WHERE ReportDate = ?`, in.Date); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func upsertReview(e *Env, r *http.Request, date, code, name string) error {
	_, err := e.DB.ExecContext(r.Context(), `
IF EXISTS (SELECT 1 FROM dbo.ReportApprovals WHERE ReportDate = ?)
  UPDATE dbo.ReportApprovals SET Status='reviewed', ReviewedBy=?, ReviewedByName=?, ReviewedAt=GETDATE(), UpdatedAt=GETDATE() WHERE ReportDate = ?
ELSE
  INSERT INTO dbo.ReportApprovals (ReportDate, Status, ReviewedBy, ReviewedByName, ReviewedAt) VALUES (?, 'reviewed', ?, ?, GETDATE())`,
		date, code, name, date, date, code, name)
	return err
}

func upsertApprove(e *Env, r *http.Request, date, code, name string) error {
	_, err := e.DB.ExecContext(r.Context(), `
UPDATE dbo.ReportApprovals SET Status='approved', ApprovedBy=?, ApprovedByName=?, ApprovedAt=GETDATE(), UpdatedAt=GETDATE()
WHERE ReportDate = ?`, code, name, date)
	return err
}

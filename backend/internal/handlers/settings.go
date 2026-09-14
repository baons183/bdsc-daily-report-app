package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"bdscdailyreport/internal/db"
)

const settingKeyCBMExcelPath = "CBM_EXCEL_PATH"

// effectiveCBMExcelPath resolves the CBM Excel path an Admin may have set
// from the web UI, falling back to the .env value (e.CBMExcelPath) if
// nobody has ever changed it — every place that reads the CBM Excel file
// should call this instead of using e.CBMExcelPath directly.
func (e *Env) effectiveCBMExcelPath() string {
	return db.GetSetting(e.DB, settingKeyCBMExcelPath, e.CBMExcelPath)
}

type cbmExcelPathResp struct {
	CBMExcelPath string `json:"cbmExcelPath"`
}

// GetCBMExcelPathSetting returns the currently effective CBM Excel path
// (DB override if set, otherwise the .env default).
func (e *Env) GetCBMExcelPathSetting(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, cbmExcelPathResp{CBMExcelPath: e.effectiveCBMExcelPath()})
}

type setCBMExcelPathReq struct {
	CBMExcelPath string `json:"cbmExcelPath"`
}

// SetCBMExcelPathSetting lets an Admin change the CBM Excel path without
// editing .env or restarting the app.
func (e *Env) SetCBMExcelPathSetting(w http.ResponseWriter, r *http.Request) {
	var in setCBMExcelPathReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	user := currentUser(r)
	updatedBy := ""
	if user != nil {
		updatedBy = user.EmployeeCode
	}
	if err := db.SetSetting(e.DB, settingKeyCBMExcelPath, in.CBMExcelPath, updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

const settingKeySendAllDays = "SEND_ALL_DAYS"

// EffectiveSendAllDays reports whether the daily report should run every
// day of the week (including Sat/Sun) instead of the default "skip
// weekends unless a specific date is added as an exception" behavior.
func (e *Env) EffectiveSendAllDays() bool {
	return db.GetSetting(e.DB, settingKeySendAllDays, "false") == "true"
}

type sendAllDaysResp struct {
	SendAllDays bool `json:"sendAllDays"`
}

func (e *Env) GetSendAllDays(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, sendAllDaysResp{SendAllDays: e.EffectiveSendAllDays()})
}

type setSendAllDaysReq struct {
	SendAllDays bool `json:"sendAllDays"`
}

// SetSendAllDays lets an Admin switch between "skip Thứ 7/CN by default,
// unless a specific date is added as an exception" (false, the original
// behavior) and "run every day of the week" (true, exceptions become
// moot). Takes effect within a minute — scheduleDailyReport polls it.
func (e *Env) SetSendAllDays(w http.ResponseWriter, r *http.Request) {
	var in setSendAllDaysReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	user := currentUser(r)
	updatedBy := ""
	if user != nil {
		updatedBy = user.EmployeeCode
	}
	val := "false"
	if in.SendAllDays {
		val = "true"
	}
	if err := db.SetSetting(e.DB, settingKeySendAllDays, val, updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

const (
	settingKeyKiemTraHour    = "KIEMTRA_DEADLINE_HOUR"
	settingKeyKiemTraMinute  = "KIEMTRA_DEADLINE_MINUTE"
	settingKeyPheDuyetHour   = "PHEDUYET_DEADLINE_HOUR"
	settingKeyPheDuyetMinute = "PHEDUYET_DEADLINE_MINUTE"
)

// EffectiveApprovalSchedule returns the Admin-configured Kiểm tra/Phê duyệt
// deadline times, defaulting to 09:00 (Kiểm tra) and 10:00 (Phê duyệt) —
// both intentionally after the default 08:00 report-generation time, giving
// the Moderator/Approver a window to act before the auto-fallback fires.
func (e *Env) EffectiveApprovalSchedule() (kiemTraHour, kiemTraMinute, pheDuyetHour, pheDuyetMinute int) {
	kiemTraHour = atoiOr(db.GetSetting(e.DB, settingKeyKiemTraHour, "9"), 9)
	kiemTraMinute = atoiOr(db.GetSetting(e.DB, settingKeyKiemTraMinute, "0"), 0)
	pheDuyetHour = atoiOr(db.GetSetting(e.DB, settingKeyPheDuyetHour, "10"), 10)
	pheDuyetMinute = atoiOr(db.GetSetting(e.DB, settingKeyPheDuyetMinute, "0"), 0)
	return
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

type approvalScheduleResp struct {
	KiemTraHour    int `json:"kiemTraHour"`
	KiemTraMinute  int `json:"kiemTraMinute"`
	PheDuyetHour   int `json:"pheDuyetHour"`
	PheDuyetMinute int `json:"pheDuyetMinute"`
}

// GetApprovalSchedule returns the current auto-Kiểm tra / auto-Phê duyệt
// deadline times an Admin has configured (see scheduleApprovalDeadlines in
// main.go for what happens when a deadline passes unattended).
func (e *Env) GetApprovalSchedule(w http.ResponseWriter, r *http.Request) {
	kh, km, ph, pm := e.EffectiveApprovalSchedule()
	writeJSON(w, 200, approvalScheduleResp{KiemTraHour: kh, KiemTraMinute: km, PheDuyetHour: ph, PheDuyetMinute: pm})
}

type setApprovalScheduleReq struct {
	KiemTraHour    int `json:"kiemTraHour"`
	KiemTraMinute  int `json:"kiemTraMinute"`
	PheDuyetHour   int `json:"pheDuyetHour"`
	PheDuyetMinute int `json:"pheDuyetMinute"`
}

// SetApprovalSchedule lets an Admin change the 2 independent deadlines
// without editing .env or restarting the app — scheduleApprovalDeadlines
// polls these settings every 20s, so a change takes effect almost
// immediately.
func (e *Env) SetApprovalSchedule(w http.ResponseWriter, r *http.Request) {
	var in setApprovalScheduleReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, "du lieu khong hop le: "+err.Error())
		return
	}
	if in.KiemTraHour < 0 || in.KiemTraHour > 23 || in.KiemTraMinute < 0 || in.KiemTraMinute > 59 ||
		in.PheDuyetHour < 0 || in.PheDuyetHour > 23 || in.PheDuyetMinute < 0 || in.PheDuyetMinute > 59 {
		writeErr(w, 400, "gio/phut khong hop le")
		return
	}
	user := currentUser(r)
	updatedBy := ""
	if user != nil {
		updatedBy = user.EmployeeCode
	}
	if err := db.SetSetting(e.DB, settingKeyKiemTraHour, strconv.Itoa(in.KiemTraHour), updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := db.SetSetting(e.DB, settingKeyKiemTraMinute, strconv.Itoa(in.KiemTraMinute), updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := db.SetSetting(e.DB, settingKeyPheDuyetHour, strconv.Itoa(in.PheDuyetHour), updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := db.SetSetting(e.DB, settingKeyPheDuyetMinute, strconv.Itoa(in.PheDuyetMinute), updatedBy); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

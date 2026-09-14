package handlers

import (
	"encoding/json"
	"net/http"

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

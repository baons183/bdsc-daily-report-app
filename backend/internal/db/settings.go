package db

import "database/sql"

// GetSetting reads one dbo.AppSettings value, falling back to `fallback`
// (normally the .env value) if the key has never been set from the web UI.
func GetSetting(conn *sql.DB, key, fallback string) string {
	var v string
	if err := conn.QueryRow(`SELECT SettingValue FROM dbo.AppSettings WHERE SettingKey = ?`, key).Scan(&v); err != nil {
		return fallback
	}
	return v
}

// SetSetting upserts one dbo.AppSettings value.
func SetSetting(conn *sql.DB, key, value, updatedBy string) error {
	_, err := conn.Exec(`
IF EXISTS (SELECT 1 FROM dbo.AppSettings WHERE SettingKey = ?)
  UPDATE dbo.AppSettings SET SettingValue = ?, UpdatedBy = ?, UpdatedAt = GETDATE() WHERE SettingKey = ?
ELSE
  INSERT INTO dbo.AppSettings (SettingKey, SettingValue, UpdatedBy) VALUES (?, ?, ?)`,
		key, value, updatedBy, key, key, value, updatedBy)
	return err
}

// ReportTime reads the Admin-configured daily-report send time from
// dbo.ReportSchedule, falling back to (defaultHour, defaultMinute) if the
// row is missing or unreadable. Shared by main.go's scheduler and every
// handler/report function that needs "what time does the reporting day
// start" for window.EntryWindow/ReportWindow — so changing the send time
// in the Admin panel reshapes the actual report window, not just its
// on-screen label.
func ReportTime(conn *sql.DB, defaultHour, defaultMinute int) (hour, minute int) {
	if err := conn.QueryRow(`SELECT ReportHour, ReportMinute FROM dbo.ReportSchedule WHERE Id = 1`).Scan(&hour, &minute); err != nil {
		return defaultHour, defaultMinute
	}
	return hour, minute
}

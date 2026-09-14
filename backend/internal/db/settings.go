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

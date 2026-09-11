package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// NullDate is an optional calendar date (no time-of-day component) — sent
// over JSON as "YYYY-MM-DD" (matching an HTML <input type="date">, e.g.
// "2026-09-13") or null/empty, and stored underneath as a SQL DATE. Go's
// default time.Time JSON (un)marshaling only accepts RFC3339
// ("2006-01-02T15:04:05Z07:00"), which rejects a plain date string — this
// type exists specifically to accept that plain form instead.
type NullDate struct {
	Time  time.Time
	Valid bool
}

func (d *NullDate) UnmarshalJSON(data []byte) error {
	var s *string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == nil || *s == "" {
		d.Valid = false
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return fmt.Errorf("ngay khong hop le %q (can dang YYYY-MM-DD): %w", *s, err)
	}
	d.Time = t
	d.Valid = true
	return nil
}

func (d NullDate) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.Format("2006-01-02"))
}

func (d *NullDate) Scan(value interface{}) error {
	if value == nil {
		d.Valid = false
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("khong doc duoc NullDate tu gia tri kieu %T", value)
	}
	d.Time = t
	d.Valid = true
	return nil
}

func (d NullDate) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Time, nil
}

package report

import (
	"log"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"bdscdailyreport/internal/models"
)

// cbmTimeLayouts covers the formats the external monitoring system's export
// has been observed to use for Data_Time (plain "yyyy-MM-ddTHH:mm:ss", no
// timezone suffix — treated as local time, matching how every other
// timestamp in this app is local).
var cbmTimeLayouts = []string{
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"01/02/2006 15:04:05",
}

func parseCBMTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range cbmTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ReadCBMExcel reads the daily-synced monitoring export and keeps only rows
// whose Data_Time falls inside [start, end] — the file accumulates every
// alarm ever logged (not reset daily), so without this filter every day's
// report would repeat old alarms indefinitely. Columns are matched by header
// name (case-insensitive) so small header differences don't break the
// report. Returns an empty slice (not an error) if the path is blank or
// unreadable, since this section is best-effort/optional.
func ReadCBMExcel(path string, start, end time.Time) []models.CBMRow {
	if path == "" {
		return nil
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		log.Printf("khong doc duoc file CBM excel (%s): %v", path, err)
		return nil
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	rowsRaw, err := f.GetRows(sheet)
	if err != nil || len(rowsRaw) < 2 {
		return nil
	}
	header := rowsRaw[0]
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToUpper(strings.TrimSpace(h))] = i
	}
	find := func(names ...string) int {
		for _, n := range names {
			if i, ok := col[strings.ToUpper(n)]; ok {
				return i
			}
		}
		return -1
	}
	iTag := find("TAGNAME", "TAG NAME", "TAG")
	iTime := find("DATA_TIME", "DATA TIME", "THOI GIAN", "TIME")
	iEquip := find("EQUIPMENT", "THIET BI")
	iDesc := find("DESCRIPTION", "MO TA")
	iVal := find("ALARM_VALUE", "ALARM VALUE", "GIA TRI")
	iLevel := find("ALARM_LEVEL", "ALARM LEVEL", "MUC CANH BAO")

	get := func(r []string, i int) string {
		if i < 0 || i >= len(r) {
			return ""
		}
		return strings.TrimSpace(r[i])
	}

	var out []models.CBMRow
	for _, r := range rowsRaw[1:] {
		if len(r) == 0 || strings.TrimSpace(strings.Join(r, "")) == "" {
			continue
		}
		t, ok := parseCBMTime(get(r, iTime))
		if !ok || t.Before(start) || t.After(end) {
			continue
		}
		out = append(out, models.CBMRow{
			Tagname:     get(r, iTag),
			DataTime:    t,
			Equipment:   get(r, iEquip),
			Description: get(r, iDesc),
			AlarmValue:  get(r, iVal),
			AlarmLevel:  get(r, iLevel),
		})
	}
	return dedupeByTagnameEarliest(out)
}

// dedupeByTagnameEarliest collapses repeated alarms sharing the same
// Tagname AND Description (the monitoring system can log the exact same
// alarm many times within one window) down to a single row — the earliest
// one within the report window, i.e. when the alarm first started rather
// than when it was last still firing. A single tag with genuinely
// different alarm descriptions (different measurement points/alarm types)
// is kept as separate rows.
func dedupeByTagnameEarliest(rows []models.CBMRow) []models.CBMRow {
	earliest := map[string]models.CBMRow{}
	order := []string{}
	keyOf := func(r models.CBMRow) string { return r.Tagname + "|" + r.Description }
	for _, row := range rows {
		key := keyOf(row)
		if existing, ok := earliest[key]; !ok || row.DataTime.Before(existing.DataTime) {
			if !ok {
				order = append(order, key)
			}
			earliest[key] = row
		}
	}
	out := make([]models.CBMRow, 0, len(order))
	for _, key := range order {
		out = append(out, earliest[key])
	}
	return out
}

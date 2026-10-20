// Package rsm reads BSR's daily "RSM Night Shift Report" PDF (an automated
// export from an external shift-handover system, unrelated to this app's
// own data entry) and pulls out section "IV.2 MECHANICAL INTEGRITY/
// EQUIPMENT RELIABILITY", filtered down to Ban BDSC's own departments, for
// rendering as report section "3.3. Vấn đề khác liên quan tới độ tin cậy
// và toàn vẹn thiết bị" (see internal/report/rsm_section.go). PDF layout
// parsing is inherently heuristic (tuned to this exact export template)
// and could silently misparse if BSR ever changes it — this is a stopgap
// until the data is available from a proper data lake feed instead.
package rsm

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// Row is one line of the IV.2 table, in the PDF's own column order.
type Row struct {
	Depart         string `json:"depart"`
	TagName        string `json:"tagName"`
	Issue          string `json:"issue"`
	MitigationPlan string `json:"mitigationPlan"`
	ActionBy       string `json:"actionBy"`
	DueDate        string `json:"dueDate"` // DD/MM/YYYY as printed, or "" if unparsed
	Status         string `json:"status"`
}

// FindPDF looks in folder for the file naming convention BSR's export tool
// uses — "RSM-Night-Shift-Report-on-DD-MM-YYYY.pdf" — for reportDate. The
// file's own date label matches this app's own "printed report date"
// convention (a report generated the morning of D+1 is labeled D), since
// the night shift it covers runs ~19:00 on D through the early morning of
// D+1 — so the same reportDate string this app already uses elsewhere
// resolves the right file.
func FindPDF(folder string, reportDate time.Time) (string, bool) {
	if folder == "" {
		return "", false
	}
	name := fmt.Sprintf("RSM-Night-Shift-Report-on-%s.pdf", reportDate.Format("02-01-2006"))
	path := filepath.Join(folder, name)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

// word is one reconstructed table-cell fragment: consecutive characters
// the PDF emitted at the exact same pen position, which in this export
// template lines up with one column's worth of text per visual line.
type word struct {
	Y, X float64
	S    string
}

// ParsePDF extracts section IV.2's table (MECHANICAL INTEGRITY/EQUIPMENT
// RELIABILITY) as rows. Best-effort: a PDF this app can't open, or where
// the IV.2/IV.3 markers can't be found, returns an empty slice rather
// than an error — this feature is optional convenience, never something
// that should block the rest of the app.
func ParsePDF(path string) ([]Row, error) {
	return parseSection(path, "IV.2", "IV.3")
}

// ParseNewTEIC extracts section IV.3's table (NEW TE/IC IN SHIFT — new
// Technical Event/Incident logged during the shift) as rows, using the
// exact same column layout as IV.2. This section is very often empty in
// practice (no new TE/IC that shift) — an empty slice is the normal case,
// not an error.
func ParseNewTEIC(path string) ([]Row, error) {
	return parseSection(path, "IV.3", "V. Recommendation")
}

// parseSection extracts the table between startMarker and endMarker
// (matched as literal text prefixes) — shared by ParsePDF (IV.2) and
// ParseNewTEIC (IV.3), which use the exact same column layout.
func parseSection(path, startMarker, endMarker string) ([]Row, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("khong mo duoc file PDF: %w", err)
	}
	defer f.Close()

	var rows []Row
	collecting := false
	done := false
	bounds := defaultColumnBounds
	for pageNum := 1; pageNum <= r.NumPage() && !done; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		var windowWords []word
		for _, w := range wordsFromPage(page) {
			text := cleanText(w.S)
			if isPageChrome(text) {
				continue
			}
			if !collecting {
				if strings.HasPrefix(text, startMarker) {
					collecting = true
				}
				continue
			}
			if strings.HasPrefix(text, endMarker) {
				done = true
				break
			}
			windowWords = append(windowWords, w)
		}
		// Recalibrate from this page's own header row if present (usually
		// only the first page of the table has one) — keeps working even
		// if BSR rescales the export again, instead of silently bucketing
		// every column from wherever it drifted into the last one (see
		// calibrateColumnBounds's comment on the 07/10/2026 incident).
		if b := calibrateColumnBounds(windowWords); b != nil {
			bounds = b
		}
		rows = append(rows, rowsFromWords(windowWords, bounds)...)
	}
	return rows, nil
}

// isPageChrome reports whether a fragment is the export tool's own
// repeated per-page header ("RSM Shift Handover" / "Snapshot at: ..." /
// "Last Modified: ...") rather than table content — it's printed at the
// top of every page, including ones in the middle of a section that spans
// a page break, so it must be filtered out before it bleeds into
// whichever row happens to still be open at that point.
func isPageChrome(text string) bool {
	return text == "RSM Shift Handover" ||
		strings.HasPrefix(text, "Snapshot at:") ||
		strings.HasPrefix(text, "Last Modified:")
}

// cleanText trims a fragment and normalizes the non-breaking spaces this
// export occasionally emits (e.g. "of RDL. ") to plain spaces.
func cleanText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, " ", " "))
}

// wordsFromPage groups a page's individual character fragments (the
// ledongthuc/pdf library reports one Text entry per glyph, but every
// glyph belonging to one PDF text-showing operation shares the exact same
// starting X/Y in this export) back into per-cell strings, ordered top to
// bottom then left to right — the same order the table reads in.
func wordsFromPage(page pdf.Page) []word {
	texts := page.Content().Text
	sort.SliceStable(texts, func(i, j int) bool {
		if texts[i].Y != texts[j].Y {
			return texts[i].Y > texts[j].Y
		}
		return texts[i].X < texts[j].X
	})
	var words []word
	for _, t := range texts {
		if n := len(words); n > 0 && words[n-1].Y == t.Y && words[n-1].X == t.X {
			words[n-1].S += t.S
			continue
		}
		words = append(words, word{t.Y, t.X, t.S})
	}
	return words
}

// defaultColumnBounds are IV.2's 8 columns' boundaries (No., Depart., Tag
// name, Issue/Observation, Mitigation plan, Action by, Due Date, Action
// status) as last observed in BSR's export — used only as a fallback for a
// page whose own header row can't be found (calibrateColumnBounds returns
// nil), since BSR has already changed this export's scale/margins at least
// once before (see calibrateColumnBounds's comment) and a hardcoded table
// alone silently misparses every row the moment it drifts again.
var defaultColumnBounds = []float64{70, 110, 205, 300, 450, 495, 560}

// columnOf buckets an X position into one of the 8 columns using bounds (7
// boundary values separating 8 columns, increasing) — roughly halfway
// between each pair of adjacent columns so minor per-line offsets within a
// wrapped cell still land in the right bucket.
func columnOf(x float64, bounds []float64) int {
	for i, b := range bounds {
		if x < b {
			return i
		}
	}
	return len(bounds)
}

// calibrateColumnBounds derives this page's own column boundaries from
// wherever its header row's 8 labels actually sit, instead of trusting a
// hardcoded pixel table tuned to one past export — BSR has already
// reflowed this report at least once (around 07/10/2026, every column
// from "Action by" onward shifted ~30% right, which silently made
// FilterByDepartment drop every single row for ~2 weeks since none of
// their real department codes landed in the Action by bucket anymore).
// Returns nil if the header row isn't present on this page/word set (most
// pages of a multi-page table only have it once) — the caller keeps
// whatever bounds it last calibrated, or defaultColumnBounds if none yet.
func calibrateColumnBounds(words []word) []float64 {
	// "Action status" wraps onto 2 lines ("Action" / "status") — keep
	// whichever of the two is seen first; they sit at virtually the same X.
	labelX := map[string]float64{}
	for _, w := range words {
		text := cleanText(w.S)
		switch text {
		case "No", "Depart.", "Tag name", "Issue/ Observation", "Mitigation plan", "Action by", "Due Date":
			if _, ok := labelX[text]; !ok {
				labelX[text] = w.X
			}
		case "Action", "status", "Action status":
			if _, ok := labelX["Action status"]; !ok {
				labelX["Action status"] = w.X
			}
		}
	}
	order := []string{"No", "Depart.", "Tag name", "Issue/ Observation", "Mitigation plan", "Action by", "Due Date", "Action status"}
	xs := make([]float64, len(order))
	for i, label := range order {
		x, ok := labelX[label]
		if !ok {
			return nil
		}
		xs[i] = x
	}
	bounds := make([]float64, len(xs)-1)
	for i := 0; i < len(bounds); i++ {
		bounds[i] = (xs[i] + xs[i+1]) / 2
	}
	return bounds
}

var rowNumberRe = regexp.MustCompile(`^\d+$`)

// rowsFromWords reconstructs one page's worth of table rows from its
// already-filtered, already-ordered word list.
//
// The row-number column is vertically CENTERED in its row's tallest cell
// rather than pinned to the first line, so a naive "everything after this
// marker belongs to this row" scan misattributes a multi-line cell's
// earlier lines to the previous row. Instead: every "No." column marker's
// Y gives that row's approximate vertical center, and every other word
// (any column) is assigned to whichever row-center Y is numerically
// closest to it — correctly re-merging a cell's lines regardless of
// whether they sit above or below its own row's number.
// headerLabels are the column-heading strings themselves ("No",
// "Depart.", ..., and "Action"/"status" — the "Action status" heading
// wraps onto 2 lines) — matched literally and dropped wherever they
// appear, rather than by position, since the heading occasionally wraps
// to a Y that falls inside what would otherwise look like row 1's own
// band.
var headerLabels = map[string]bool{
	"No": true, "Depart.": true, "Tag name": true,
	"Issue/ Observation": true, "Mitigation plan": true,
	"Action by": true, "Due Date": true, "Action": true, "status": true,
	"Action status": true,
}

func rowsFromWords(words []word, bounds []float64) []Row {
	data := make([]word, 0, len(words))
	for _, w := range words {
		if !headerLabels[cleanText(w.S)] {
			data = append(data, w)
		}
	}

	var markerYs []float64
	for _, w := range data {
		if columnOf(w.X, bounds) == 0 && rowNumberRe.MatchString(cleanText(w.S)) {
			markerYs = append(markerYs, w.Y)
		}
	}
	if len(markerYs) == 0 {
		return nil
	}

	rows := make([]Row, len(markerYs))
	for _, w := range data {
		col := columnOf(w.X, bounds)
		if col == 0 {
			continue // the row-number itself carries no field data
		}
		text := cleanText(w.S)
		if text == "" {
			continue
		}
		appendCol(&rows[nearestMarker(markerYs, w.Y)], col, text)
	}
	return rows
}

func nearestMarker(markerYs []float64, y float64) int {
	best := 0
	bestDist := math.Abs(markerYs[0] - y)
	for i := 1; i < len(markerYs); i++ {
		if d := math.Abs(markerYs[i] - y); d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

func appendCol(r *Row, col int, text string) {
	join := func(existing string) string {
		if existing == "" {
			return text
		}
		return existing + " " + text
	}
	switch col {
	case 1:
		r.Depart = join(r.Depart)
	case 2:
		r.TagName = join(r.TagName)
	case 3:
		r.Issue = join(r.Issue)
	case 4:
		r.MitigationPlan = join(r.MitigationPlan)
	case 5:
		r.ActionBy = join(r.ActionBy)
	case 6:
		r.DueDate = join(r.DueDate)
	case 7:
		r.Status = join(r.Status)
	}
}

// ParseDueDate converts the PDF's printed "DD/MM/YYYY" due date into a
// time.Time, returning zero + false if it doesn't parse.
func ParseDueDate(s string) (time.Time, bool) {
	t, err := time.ParseInLocation("02/01/2006", strings.TrimSpace(s), time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// DepartmentKeywords are Ban BDSC's own department/technical-group codes
// as they appear in the RSM report's "Action by" column — an IV.2 row is
// kept only if at least one "/"-separated token in Action by matches one
// of these (case-insensitive), everything else (AU, PP, OMS, PRD, RTD,
// RPD, ...) belongs to other departments and is dropped. Includes both
// the diacritic and plain-ASCII spelling of the same code (TĐH/TDH,
// HTĐ/HTD) since the PDF export isn't consistent about which it uses.
var DepartmentKeywords = []string{
	"MNT", "CI", "C&I", "ROT", "STA", "MWS", "INT", "ELE",
	"TĐH", "TDH", "HTĐ", "HTD", "CKQ", "TBQ", "CKT", "TBT", "GCCK",
}

// FilterByDepartment keeps only rows whose Action by column contains at
// least one token (split on "/" and ",") matching a keyword — matched as
// whole tokens rather than raw substrings so a keyword like "INT" can't
// accidentally match inside an unrelated longer code.
func FilterByDepartment(rows []Row, keywords []string) []Row {
	keySet := make(map[string]bool, len(keywords))
	for _, k := range keywords {
		keySet[strings.ToUpper(k)] = true
	}
	splitter := func(r rune) bool { return r == '/' || r == ',' }
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		for _, tok := range strings.FieldsFunc(row.ActionBy, splitter) {
			if keySet[strings.ToUpper(strings.TrimSpace(tok))] {
				out = append(out, row)
				break
			}
		}
	}
	return out
}

// KeepDepartmentTokens returns only the "/"-separated tokens of actionBy
// that match a keyword (same matching rule as FilterByDepartment), joined
// back with "/" — e.g. "OMS/RPD/RTD/MNT" with the default keyword list
// becomes just "MNT". Used to populate a Reliability row's "Đơn vị thực
// hiện" with only Ban BDSC's own department(s), dropping whichever other
// departments the PDF's "Action by" column also lists.
func KeepDepartmentTokens(actionBy string, keywords []string) string {
	keySet := make(map[string]bool, len(keywords))
	for _, k := range keywords {
		keySet[strings.ToUpper(k)] = true
	}
	splitter := func(r rune) bool { return r == '/' || r == ',' }
	var kept []string
	for _, tok := range strings.FieldsFunc(actionBy, splitter) {
		trimmed := strings.TrimSpace(tok)
		if keySet[strings.ToUpper(trimmed)] {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, "/")
}

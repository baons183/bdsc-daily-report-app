package report

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TagKey reduces a Thiết bị / tag-name string to the set of tag-like tokens
// it mentions, so the many ways BSR's RSM export writes the same equipment
// from one day to the next collapse onto one key:
//
//	"C-301 RECYCLE GAS COMPRESSOR"                  → "c-301"
//	"RDL TK-6001F"                                  → "tk-6001f"
//	"The fire detectors 053-DF-105 and 053-DF-106"  → "053-df-105 053-df-106"
//	"053-DF-105/053-DF-106"                         → "053-df-105 053-df-106"
//
// A token counts as tag-like when it contains a digit; tokens are split on
// whitespace plus the separators BSR uses to list several tags at once
// ("/", ",", ";", "&", "+"), trimmed of surrounding punctuation, lowercased,
// deduped and sorted. A string with no tag-like token at all falls back to
// its whole lowercased text, so free-text entries only match themselves.
func TagKey(s string) string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("/,;&+", r)
	})
	seen := map[string]bool{}
	var tags []string
	for _, f := range fields {
		f = strings.TrimFunc(f, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if f == "" || seen[f] || !strings.ContainsFunc(f, unicode.IsDigit) {
			continue
		}
		seen[f] = true
		tags = append(tags, f)
	}
	if len(tags) == 0 {
		return strings.Join(strings.Fields(strings.ToLower(s)), " ")
	}
	sort.Strings(tags)
	return strings.Join(tags, " ")
}

// SameTag reports whether a and b name the same equipment (see TagKey).
func SameTag(a, b string) bool {
	return TagKey(a) == TagKey(b)
}

// ShorterTag picks which of two same-tag names to display: the shorter
// one, which is the bare tag BSR's later exports settle on ("C-301" over
// "C-301 RECYCLE GAS COMPRESSOR"). Ties keep a.
func ShorterTag(a, b string) string {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if utf8.RuneCountInString(b) < utf8.RuneCountInString(a) {
		return b
	}
	return a
}

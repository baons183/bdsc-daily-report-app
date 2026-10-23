package report

import (
	"database/sql"
	"testing"
)

func TestSameTag(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"C-301", "C-301 RECYCLE GAS COMPRESSOR", true},
		{"TK-6001F", "RDL TK-6001F", true},
		{"P-5101B", "Offspec Propylene pumps P-5101B", true},
		{"053-DF-105/053-DF-106", "The fire detectors 053-DF-105 and 053-DF-106", true},
		{"M-5116B", "Mixer (M-5116B)", true},
		{"TK-6001F", "TK-6001F2", false},
		{"TK-5108A", "TK-5108B", false},
		{"053-DF-105", "053-DF-105/053-DF-106", false},
		{"Fire water pump", "Fire water pump P-972", false},
		{"The fire detectors 053- DF-105 and 053-DF-106", "053-DF-105/053-DF-106", true},
		{"040-AIT-151 - online analyser", "040-AIT-151", true},
	}
	for _, c := range cases {
		if got := SameTag(c.a, c.b); got != c.want {
			t.Errorf("SameTag(%q, %q) = %v, want %v (keys %q / %q)", c.a, c.b, got, c.want, TagKey(c.a), TagKey(c.b))
		}
	}
}

func TestShorterTag(t *testing.T) {
	if got := ShorterTag("C-301 RECYCLE GAS COMPRESSOR", "C-301"); got != "C-301" {
		t.Errorf("got %q", got)
	}
	if got := ShorterTag("TK-6001F", "RDL TK-6001F"); got != "TK-6001F" {
		t.Errorf("got %q", got)
	}
}

func TestReliabilityKeys(t *testing.T) {
	if got := reliabilityKeys("TK-6001F", sql.NullString{}); len(got) != 1 || got[0] != "tk-6001f" {
		t.Errorf("no RsmTag: got %v", got)
	}
	if got := reliabilityKeys("TK-6001F", sql.NullString{String: "RDL TK-6001F", Valid: true}); len(got) != 1 {
		t.Errorf("same-tag RsmTag should not add a key: got %v", got)
	}
	got := reliabilityKeys("Bồn TK-6001", sql.NullString{String: "RDL TK-6001F", Valid: true})
	if len(got) != 2 || got[0] != "tk-6001" || got[1] != "tk-6001f" {
		t.Errorf("renamed row should answer to both tags: got %v", got)
	}
}

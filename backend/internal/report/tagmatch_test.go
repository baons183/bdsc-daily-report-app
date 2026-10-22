package report

import "testing"

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

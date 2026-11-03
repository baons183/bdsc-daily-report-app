package report

import (
	"testing"

	"bdscdailyreport/internal/models"
)

func TestWithAllKPICategoriesFillsMissingDays(t *testing.T) {
	got := WithAllKPICategories(nil)
	if len(got) != len(kpiCategories) {
		t.Fatalf("empty day: got %d rows, want %d", len(got), len(kpiCategories))
	}
	for i, c := range kpiCategories {
		if got[i].ChiTieu != c.ChiTieu || got[i].KeHoach == nil || *got[i].KeHoach != 0 {
			t.Errorf("row %d = %+v, want blank %q", i, got[i], c.ChiTieu)
		}
	}

	kh := 5.0
	saved := []models.KPIEntry{{ChiTieu: "Rework", KeHoach: &kh}}
	got = WithAllKPICategories(saved)
	if len(got) != len(kpiCategories) || got[0].ChiTieu != "Rework" || *got[0].KeHoach != 5 {
		t.Errorf("saved row must be kept as-is and only missing ones added, got %+v", got)
	}
}

package handlers

import (
	"testing"
	"time"

	"bdscdailyreport/internal/models"
)

func TestDedupeWeeklyReliabilityKeepsShortestTagAndLatestRow(t *testing.T) {
	older := time.Date(2026, 10, 19, 0, 0, 0, 0, time.Local)
	newer := older.AddDate(0, 0, 1)
	rows := []models.Reliability{
		{ThietBi: "RDL TK-6001F", VanDe: "old", NgayBaoCao: older},
		{ThietBi: "TK-6001F", VanDe: "latest", NgayBaoCao: newer},
		{ThietBi: "TK-6001F2", VanDe: "different tag", NgayBaoCao: newer},
		{ThietBi: "The fire detectors 053-DF-105 and 053-DF-106", VanDe: "fd latest", NgayBaoCao: newer},
		{ThietBi: "053-DF-105/053-DF-106", VanDe: "fd old", NgayBaoCao: older},
	}

	got := dedupeWeeklyReliability(rows)

	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	if got[0].ThietBi != "TK-6001F" || got[0].VanDe != "latest" {
		t.Errorf("expected shortest tag with latest row, got tag %q and issue %q", got[0].ThietBi, got[0].VanDe)
	}
	if got[1].ThietBi != "TK-6001F2" {
		t.Errorf("expected distinct tag to remain, got %q", got[1].ThietBi)
	}
	if got[2].ThietBi != "053-DF-105/053-DF-106" || got[2].VanDe != "fd latest" {
		t.Errorf("expected fire detectors merged under shortest tag with latest row, got tag %q and issue %q", got[2].ThietBi, got[2].VanDe)
	}
	if got[0].TT != 1 || got[1].TT != 2 || got[2].TT != 3 {
		t.Errorf("expected sequential row numbers 1..3, got %d, %d, %d", got[0].TT, got[1].TT, got[2].TT)
	}
}

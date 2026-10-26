package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLunarYearCache(t *testing.T) {
	s := &Service{root: t.TempDir()}
	got, err := s.GetLunarYear(2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Days) != 365 || got.Days["2026-02-17"].Day != "初一" || got.Days["2026-02-17"].Month != "正月" || got.Days["2026-10-23"].Term != "霜降" {
		t.Fatalf("invalid calendar: %+v %+v", got.Days["2026-02-17"], got.Days["2026-10-23"])
	}
	path := filepath.Join(s.root, "calendar", "lunar-2026.json")
	info, _ := os.Stat(path)
	again, err := (&Service{root: s.root}).GetLunarYear(2026)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("cache roundtrip failed", err)
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(info.ModTime()) {
		t.Fatal("cache rewritten")
	}
	leap, err := s.GetLunarYear(2024)
	if err != nil || len(leap.Days) != 366 {
		t.Fatal("leap year", err)
	}
	delete(got.Days, "2026-02-17")
	if err = writeJSON(path, got); err != nil {
		t.Fatal(err)
	}
	fixed, err := s.GetLunarYear(2026)
	if err != nil || len(fixed.Days) != 365 {
		t.Fatal("partial cache not repaired")
	}
	if _, err = s.GetLunarYear(2101); err == nil {
		t.Fatal("range not checked")
	}
}

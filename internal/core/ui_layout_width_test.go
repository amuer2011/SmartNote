package core

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestUIPreferencesInspectorWidthPersistsAcrossRestart(t *testing.T) {
	for _, width := range []int{120, 481, 660, 1024, 4096} {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "data")
			s, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetUIPreferences(UIPreferences{FontSize: 14, Layout: LayoutPreferences{InspectorWidth: width}}); err != nil {
				t.Fatalf("inspector width %d rejected: %v", width, err)
			}
			if got := s.Settings().UI.Layout.InspectorWidth; got != width {
				t.Fatalf("inspector width stored as %d, want %d", got, width)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if got := s.Settings().UI.Layout.InspectorWidth; got != width {
				t.Fatalf("inspector width after restart is %d, want %d", got, width)
			}
		})
	}
}

func TestUIPreferencesInspectorWidthDefaultsAndRejectsOutOfRange(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetUIPreferences(UIPreferences{FontSize: 14}); err != nil {
		t.Fatal(err)
	}
	if got := s.Settings().UI.Layout.InspectorWidth; got != 340 {
		t.Fatalf("default inspector width is %d, want 340", got)
	}
	if err := s.SetUIPreferences(UIPreferences{FontSize: 14, Layout: LayoutPreferences{InspectorWidth: 660}}); err != nil {
		t.Fatalf("inspector width 660 rejected: %v", err)
	}
	for _, width := range []int{119, -1} {
		if err := s.SetUIPreferences(UIPreferences{FontSize: 14, Layout: LayoutPreferences{InspectorWidth: width}}); err == nil {
			t.Fatalf("inspector width %d accepted", width)
		}
		if got := s.Settings().UI.Layout.InspectorWidth; got != 660 {
			t.Fatalf("rejected inspector width %d changed stored width to %d", width, got)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.Settings().UI.Layout.InspectorWidth; got != 660 {
		t.Fatalf("inspector width after restart is %d, want 660", got)
	}
}

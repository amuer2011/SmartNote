package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDirectoryFallsBackWhenPreferredIsUnavailable(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(root, "fallback")
	selected, err := selectDataDirectory([]string{filepath.Join(blocked, "SmartNote"), fallback})
	if err != nil || selected != fallback {
		t.Fatalf("selected %q, error %v", selected, err)
	}
	original, _ := os.ReadFile(blocked)
	if string(original) != "original" {
		t.Fatal("modified existing file")
	}
}

func TestDataDirectoryKeepsExistingProfile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"workspaces":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := selectDataDirectory([]string{root, filepath.Join(t.TempDir(), "fallback")})
	if err != nil || selected != root {
		t.Fatalf("existing profile bypassed: %q %v", selected, err)
	}
}

func TestDataDirectoryExplicitOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "my-data")
	t.Setenv("SMARTNOTE_DATA_DIR", override)
	candidates := dataDirectoryCandidates()
	if len(candidates) != 1 || candidates[0] != override {
		t.Fatalf("unexpected candidates: %v", candidates)
	}
}

func TestDataDirectoryFindsPortableRegistryBeforeEmptyDefault(t *testing.T) {
	preferred, portable := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(portable, "settings.json"), []byte(`{"workspaces":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := selectDataDirectory([]string{preferred, portable})
	if err != nil || selected != portable {
		t.Fatalf("portable registry bypassed: %q %v", selected, err)
	}
}

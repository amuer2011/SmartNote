package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceWriteProbe(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "existing.md")
	if err := os.WriteFile(keep, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := probeWorkspaceWrite(root); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("probe left files: %v %v", entries, err)
	}
	b, err := os.ReadFile(keep)
	if err != nil || string(b) != "preserve" {
		t.Fatal("existing note changed")
	}
	if err := probeWorkspaceWrite(keep); err == nil {
		t.Fatal("file accepted as workspace")
	}
}

package core

import (
	"os"
	"path/filepath"
)

// Probe only a unique temporary directory; never touch existing notes.
func probeWorkspaceWrite(root string) error {
	dir, err := os.MkdirTemp(root, ".smartnote-write-check-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "probe")
	if err = atomicWrite(path, []byte("create")); err != nil {
		return err
	}
	if err = atomicWrite(path, []byte("replace")); err != nil {
		return err
	}
	return os.Remove(path)
}

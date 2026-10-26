package core

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Include empty directories, but match the note scanner's hidden/symlink policy.
func workspaceDirectories(root string) ([]string, error) {
	dirs := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() || path == root {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		dirs = append(dirs, filepath.ToSlash(rel))
		return nil
	})
	return dirs, err
}

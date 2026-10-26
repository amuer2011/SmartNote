package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type TrashEntry struct {
	Source    string `json:"source"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified"`
}

func (s *Service) trashRoot(w Workspace, source string) (string, error) {
	switch source {
	case "notes":
		return safePath(s.data(w), "trash")
	case "folders":
		return safePath(w.Path, ".smartnote-trash")
	default:
		return "", errors.New("未知回收来源")
	}
}

func (s *Service) ListTrash(workspaceID string) ([]TrashEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	out := []TrashEntry{}
	for _, source := range []string{"notes", "folders"} {
		root, err := s.trashRoot(w, source)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
			if os.IsNotExist(e) && path == root {
				return nil
			}
			if e != nil {
				return e
			}
			if path == root {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return nil
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			out = append(out, TrashEntry{Source: source, Path: filepath.ToSlash(rel), Name: d.Name(), Directory: d.IsDir(), Size: info.Size(), Modified: info.ModTime().Format(time.RFC3339Nano)})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// On Windows a single leading slash can be rooted without IsAbs being true.
// Reject all rooted inputs and verify the resolved target stays below this root.
func trashPath(root, rel string) (string, error) {
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", errors.New("回收站路径必须是相对路径")
	}
	path, err := safePath(root, rel)
	if err != nil {
		return "", err
	}
	child, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if child == "." || child == ".." || strings.HasPrefix(child, ".."+string(filepath.Separator)) || filepath.IsAbs(child) {
		return "", errors.New("不可操作回收站本身或站外路径")
	}
	return path, nil
}

func (s *Service) ReadTrashFile(workspaceID, source, rel string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return "", err
	}
	root, err := s.trashRoot(w, source)
	if err != nil {
		return "", err
	}
	path, err := trashPath(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 2*1024*1024 {
		return "", errors.New("仅支持预览 2MB 以内的文本文件")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
		return "", errors.New("此文件不支持文本预览")
	}
	return string(b), nil
}

func removeTrashPath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err = checkedDirectory(path); err != nil {
			return err
		}
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func (s *Service) DeleteTrashEntry(workspaceID, source, rel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return err
	}
	root, err := s.trashRoot(w, source)
	if err != nil {
		return err
	}
	// safePath refuses the root itself, traversal, absolute paths and links.
	path, err := trashPath(root, rel)
	if err != nil {
		return err
	}
	return removeTrashPath(path)
}

func (s *Service) EmptyTrash(workspaceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return err
	}
	// Validate both roots before removing anything. Keep the roots themselves fixed.
	targets := []string{}
	for _, source := range []string{"notes", "folders"} {
		root, e := s.trashRoot(w, source)
		if e != nil {
			return e
		}
		entries, e := os.ReadDir(root)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if e = checkedDirectory(root); e != nil {
			return e
		}
		for _, entry := range entries {
			path, e := safePath(root, entry.Name())
			if e != nil {
				return e
			}
			targets = append(targets, path)
		}
	}
	for _, path := range targets {
		if err = removeTrashPath(path); err != nil {
			return err
		}
	}
	return nil
}

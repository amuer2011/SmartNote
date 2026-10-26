package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EntryResult struct {
	Path         string `json:"path"`
	RecoveryPath string `json:"recoveryPath"`
	UpdatedLinks int    `json:"updatedLinks"`
}

type directoryJournal struct {
	State   string       `json:"state"`
	Source  string       `json:"source"`
	Target  string       `json:"target"`
	Changes []moveChange `json:"changes"`
}

func withinEntry(path, directory string) bool {
	return strings.EqualFold(path, directory) || strings.HasPrefix(strings.ToLower(path), strings.ToLower(directory)+"/")
}

func mappedEntry(path, source, target string) string {
	if withinEntry(path, source) {
		return target + path[len(source):]
	}
	return path
}

func entryPath(w Workspace, rel string) (string, string, error) {
	full, err := safePath(w.Path, rel)
	if err != nil {
		return "", "", err
	}
	path, err := filepath.Rel(w.Path, full)
	if err != nil {
		return "", "", err
	}
	path = filepath.ToSlash(path)
	if err := visibleNotePath(path); err != nil {
		return "", "", err
	}
	return full, path, nil
}

// Reject links anywhere inside a directory before operating on the complete tree.
func checkedDirectory(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("目标不是文件夹")
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("文件夹包含符号链接，不能整体操作")
		}
		return nil
	})
}

func (s *Service) CreateDirectory(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	full, _, err := entryPath(w, path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(full); !errors.Is(err, os.ErrNotExist) {
		return errors.New("目标已存在或无法访问")
	}
	return os.MkdirAll(full, 0700)
}

func (s *Service) EntryLocation(path string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return "", err
	}
	if path == "" || path == "." {
		return w.Path, nil
	}
	full, _, err := entryPath(w, path)
	if err != nil {
		return "", err
	}
	_, err = os.Stat(full)
	return full, err
}

// Moving a folder is one filesystem rename. Link edits have checked backups and
// a write-ahead journal, so a failed/interrupted operation can be rolled back.
func (s *Service) MoveDirectory(source, target string) (EntryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := EntryResult{}
	w, err := s.active()
	if err != nil {
		return out, err
	}
	original, source, err := entryPath(w, source)
	if err != nil {
		return out, err
	}
	dest, target, err := entryPath(w, target)
	if err != nil {
		return out, err
	}
	if withinEntry(target, source) {
		return out, errors.New("不能移动到自身或自己的子目录")
	}
	if err = checkedDirectory(original); err != nil {
		return out, err
	}
	if _, err = os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
		return out, errors.New("目标已存在或无法访问，不会合并或覆盖")
	}
	notes, err := s.scan(w)
	if err != nil {
		return out, err
	}
	changed := map[string]string{}
	links := 0
	for _, n := range notes {
		from := mappedEntry(n.Path, source, target)
		edits := []textEdit{}
		for _, ref := range markdownLinks(n.Content) {
			if ref.Wiki {
				linked := resolveNoteLink(n, ref, notes)
				if linked == nil || (!withinEntry(linked.Path, source) && from == n.Path) {
					continue
				}
				fragment := ""
				if i := strings.IndexByte(ref.Target, '#'); i >= 0 {
					fragment = ref.Target[i:]
				}
				value := strings.TrimSuffix(mappedEntry(linked.Path, source, target), ".md") + fragment
				if value != ref.Target {
					edits = append(edits, textEdit{Start: ref.Start, End: ref.End, Value: value})
					links++
				}
				continue
			}
			resolved, suffix, ok := localTarget(n.Path, ref.Target)
			if !ok {
				continue
			}
			mapped := mappedEntry(resolved, source, target)
			if from == n.Path && mapped == resolved {
				continue
			}
			rel, e := filepath.Rel(filepath.Dir(filepath.FromSlash(from)), filepath.FromSlash(mapped))
			if e != nil {
				return out, e
			}
			value := encodedPath(filepath.ToSlash(rel)) + suffix
			if value != ref.Target {
				edits = append(edits, textEdit{Start: ref.Start, End: ref.End, Value: value})
				links++
			}
		}
		text := applyEdits(n.Content, edits)
		if from != n.Path && n.SyntheticID {
			newline := "\n"
			if strings.Contains(text, "\r\n") {
				newline = "\r\n"
			}
			if strings.HasPrefix(text, "---"+newline) {
				text = "---" + newline + "id: " + n.ID + newline + text[len("---"+newline):]
			} else {
				text = "---" + newline + "id: " + n.ID + newline + "---" + newline + newline + text
			}
		}
		if from != n.Path && parseNote(from, []byte(text), time.Now()).ID != n.ID {
			return out, errors.New("无法保留笔记 ID，请先修复 Front Matter")
		}
		if text != n.Content {
			changed[n.Path] = text
		}
	}
	recovery := filepath.Join(s.data(w), "directory-operations", uid())
	j := directoryJournal{State: "prepared", Source: source, Target: target, Changes: []moveChange{}}
	for _, n := range notes {
		if text, ok := changed[n.Path]; ok {
			backup := fmt.Sprintf("%d.md", len(j.Changes))
			if err := atomicWrite(filepath.Join(recovery, backup), []byte(n.Content)); err != nil {
				return out, err
			}
			j.Changes = append(j.Changes, moveChange{Path: n.Path, BeforeHash: n.Hash, AfterHash: digest([]byte(text)), Backup: backup})
		}
	}
	journalPath := filepath.Join(recovery, "journal.json")
	if err = writeJSON(journalPath, j); err != nil {
		return out, err
	}
	fail := func(cause error) (EntryResult, error) {
		if e := s.rollbackDirectory(w, recovery, &j); e != nil {
			return out, fmt.Errorf("%v；需手动恢复：%v；副本：%s", cause, e, recovery)
		}
		return out, fmt.Errorf("%v；操作已恢复，副本：%s", cause, recovery)
	}
	// Check every note, including unchanged ones, before moving the tree.
	for _, n := range notes {
		p, e := safePath(w.Path, n.Path)
		if e != nil {
			return fail(e)
		}
		b, e := os.ReadFile(p)
		if e != nil || digest(b) != n.Hash {
			return fail(errors.New("CONFLICT: 文件发生外部修改"))
		}
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return fail(err)
	}
	if err = os.Rename(original, dest); err != nil {
		return fail(err)
	}
	for _, c := range j.Changes {
		p, e := safePath(w.Path, mappedEntry(c.Path, source, target))
		if e != nil {
			return fail(e)
		}
		b, e := os.ReadFile(p)
		if e != nil || digest(b) != c.BeforeHash {
			return fail(errors.New("CONFLICT: 更新链接前检测到外部修改"))
		}
		if e = atomicWrite(p, []byte(changed[c.Path])); e != nil {
			return fail(e)
		}
	}
	j.State = "committed"
	if err = writeJSON(journalPath, j); err != nil {
		return fail(err)
	}
	return EntryResult{Path: target, RecoveryPath: recovery, UpdatedLinks: links}, nil
}

func (s *Service) rollbackDirectory(w Workspace, recovery string, j *directoryJournal) error {
	source, err := safePath(w.Path, j.Source)
	if err != nil {
		return err
	}
	target, err := safePath(w.Path, j.Target)
	if err != nil {
		return err
	}
	_, se := os.Lstat(source)
	_, te := os.Lstat(target)
	moved := errors.Is(se, os.ErrNotExist) && te == nil
	if se == nil && te == nil {
		return errors.New("源和目标同时存在，保留文件，请手动恢复")
	}
	if !moved {
		if se != nil {
			return fmt.Errorf("源文件夹无法访问: %w", se)
		}
		if !errors.Is(te, os.ErrNotExist) {
			return te
		}
	}
	for _, c := range j.Changes {
		path := c.Path
		if moved {
			path = mappedEntry(path, j.Source, j.Target)
		}
		p, e := safePath(w.Path, path)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if digest(b) == c.BeforeHash {
			continue
		}
		if digest(b) != c.AfterHash {
			return fmt.Errorf("保留外部修改：%s", path)
		}
		backup, e := safePath(recovery, c.Backup)
		if e != nil {
			return e
		}
		before, e := os.ReadFile(backup)
		if e != nil || digest(before) != c.BeforeHash {
			return errors.New("恢复副本校验失败")
		}
		if e = atomicWrite(p, before); e != nil {
			return e
		}
	}
	if moved {
		if err = checkedDirectory(target); err != nil {
			return err
		}
		if err = os.Rename(target, source); err != nil {
			return err
		}
	}
	j.State = "rolled-back"
	return writeJSON(filepath.Join(recovery, "journal.json"), j)
}

func (s *Service) recoverDirectories() error {
	for _, w := range s.settings.Workspaces {
		root := filepath.Join(s.data(w), "directory-operations")
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			recovery, err := safePath(root, entry.Name())
			if err != nil {
				return err
			}
			var j directoryJournal
			if err = readJSON(filepath.Join(recovery, "journal.json"), &j); err != nil {
				return err
			}
			if j.State == "prepared" {
				if err = s.rollbackDirectory(w, recovery, &j); err != nil {
					return fmt.Errorf("文件夹移动未完成，恢复目录 %s: %w", recovery, err)
				}
			}
		}
	}
	return nil
}

// Trash stays on the workspace's volume so deletion is an atomic rename and
// preserves empty folders, non-Markdown files and hidden children together.
func (s *Service) DeleteDirectory(path string) (EntryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return EntryResult{}, err
	}
	full, path, err := entryPath(w, path)
	if err != nil {
		return EntryResult{}, err
	}
	if err = checkedDirectory(full); err != nil {
		return EntryResult{}, err
	}
	trashRoot := filepath.Join(w.Path, ".smartnote-trash")
	trash, err := safePath(w.Path, filepath.Join(".smartnote-trash", uid(), filepath.FromSlash(path)))
	if err != nil {
		return EntryResult{}, err
	}
	if err = os.MkdirAll(filepath.Dir(trash), 0700); err != nil {
		return EntryResult{}, err
	}
	if err = writeJSON(filepath.Join(filepath.Dir(trash), ".recovery.json"), map[string]string{"original": path, "deletedAt": time.Now().Format(time.RFC3339)}); err != nil {
		return EntryResult{}, err
	}
	if err = os.Rename(full, trash); err != nil {
		return EntryResult{}, err
	}
	return EntryResult{Path: path, RecoveryPath: trashRoot}, nil
}

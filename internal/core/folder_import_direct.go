package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Called with s.mu held. All validation and asset preparation has completed
// before any user Markdown is changed. Keep recovery files if rollback conflicts.
func (s *Service) commitImportedFolder(w Workspace, notes []Note, originals map[string]string, stage string) (keepRecovery bool, err error) {
	type change struct {
		Path, BeforeHash, AfterHash, Backup string
		Modified                            time.Time
	}
	changes := []change{}
	for _, note := range notes {
		path, e := safePath(w.Path, note.Path)
		if e != nil {
			return false, e
		}
		before, e := os.ReadFile(path)
		if e != nil {
			return false, e
		}
		if digest(before) != originals[note.Path] {
			return false, fmt.Errorf("CONFLICT：导入期间文件已修改：%s", note.Path)
		}
		if digest(before) == note.Hash {
			continue
		}
		info, e := os.Stat(path)
		if e != nil {
			return false, e
		}
		backup := filepath.Join(stage, "originals", filepath.FromSlash(note.Path))
		if e = atomicWrite(backup, before); e != nil {
			return false, e
		}
		changes = append(changes, change{path, digest(before), note.Hash, backup, info.ModTime()})
	}
	if err = writeJSON(filepath.Join(stage, "recovery.json"), changes); err != nil {
		return false, err
	}
	applied := 0
	defer func() {
		if err == nil {
			return
		}
		for i := applied - 1; i >= 0; i-- {
			c := changes[i]
			current, e := os.ReadFile(c.Path)
			if e == nil && digest(current) == c.BeforeHash {
				continue
			}
			if e == nil && digest(current) != c.AfterHash {
				e = errors.New("文件又被外部修改，未覆盖")
			}
			if e == nil {
				var before []byte
				before, e = os.ReadFile(c.Backup)
				if e == nil {
					e = atomicWrite(c.Path, before)
				}
				if e == nil {
					e = os.Chtimes(c.Path, c.Modified, c.Modified)
				}
			}
			if e != nil {
				keepRecovery = true
				err = errors.Join(err, fmt.Errorf("恢复 %s：%w；原文备份：%s", c.Path, e, c.Backup))
			}
		}
	}()
	for _, c := range changes {
		current, e := os.ReadFile(c.Path)
		if e != nil {
			return false, e
		}
		if digest(current) != c.BeforeHash {
			return false, fmt.Errorf("CONFLICT：导入期间文件已修改：%s", c.Path)
		}
		rel, e := filepath.Rel(w.Path, c.Path)
		if e != nil {
			return false, e
		}
		prepared, e := os.ReadFile(filepath.Join(stage, "notes", rel))
		if e != nil {
			return false, e
		}
		if err = atomicWrite(c.Path, prepared); err != nil {
			return false, err
		}
		applied++
		if err = os.Chtimes(c.Path, c.Modified, c.Modified); err != nil {
			return false, err
		}
	}
	previous := s.settings
	s.settings.Workspaces = append(s.settings.Workspaces, w)
	s.settings.ActiveID = w.ID
	if err = s.persist(); err != nil {
		s.settings = previous
		return false, err
	}
	return false, nil
}

// Reopening an already registered directory keeps its metadata and identifiers.
func (s *Service) reopenImportedFolder(w Workspace, organize bool) (FolderImportResult, error) {
	out := FolderImportResult{Workspace: w, Warnings: []string{}}
	notes, err := s.scan(w)
	if err != nil {
		return out, err
	}
	attachments := []Attachment{}
	if err = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &attachments); err != nil {
		return out, err
	}
	config, err := s.semanticConfig(w)
	if err != nil {
		return out, err
	}
	path := filepath.Join(s.data(w), "semantic-archive.json")
	before, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return out, readErr
	}
	config.Automatic, config.AutomaticVersion, config.Revision = organize, 1, uid()
	if err = writeJSON(path, config); err != nil {
		return out, err
	}
	previous := s.settings
	s.settings.ActiveID = w.ID
	if err = s.persist(); err != nil {
		s.settings = previous
		if readErr == nil {
			err = errors.Join(err, atomicWrite(path, before))
		} else {
			err = errors.Join(err, os.Remove(path))
		}
		return out, err
	}
	out.Notes, out.Attachments = len(notes), len(attachments)
	return out, nil
}

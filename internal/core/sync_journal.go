package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type syncChange struct {
	Key        string `json:"key"`
	BeforeHash string `json:"beforeHash"`
	AfterHash  string `json:"afterHash"`
}
type syncJournal struct {
	State   string         `json:"state"`
	Changes []syncChange   `json:"changes"`
	Before  syncLocalState `json:"before"`
}

func fileDigest(path string) (string, error) {
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	return digest(b), nil
}

// Caller holds s.mu. Backups and the journal are flushed before changing files;
// incomplete transactions roll back at once or on next startup.
func (s *Service) applySyncFiles(w Workspace, before map[string][]byte, writes map[string][]byte, deletes map[string]bool, old, next syncLocalState) (err error) {
	if len(writes) == 0 && len(deletes) == 0 {
		return writeJSON(s.syncPath(w, "state.json"), next)
	}
	operation := s.syncPath(w, "operations/"+uid())
	j := syncJournal{State: "prepared", Before: old, Changes: []syncChange{}}
	keys := map[string]bool{}
	for key := range writes {
		keys[key] = true
	}
	for key := range deletes {
		keys[key] = true
	}
	for key := range keys {
		ch := syncChange{Key: key}
		if b, ok := before[key]; ok {
			ch.BeforeHash = digest(b)
			if e := atomicWrite(filepath.Join(operation, "before", filepath.FromSlash(key)), b); e != nil {
				return e
			}
		}
		if b, ok := writes[key]; ok {
			ch.AfterHash = digest(b)
		}
		j.Changes = append(j.Changes, ch)
	}
	if err = writeJSON(filepath.Join(operation, "journal.json"), j); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, s.rollbackSync(w, operation, j))
		}
	}()
	for _, ch := range j.Changes {
		path, e := s.syncFilePath(w, ch.Key)
		if e != nil {
			return e
		}
		current, e := fileDigest(path)
		if e != nil {
			return e
		}
		if current != ch.BeforeHash {
			return errors.New("同步提交期间文件被外部修改，已停止覆盖")
		}
		if b, ok := writes[ch.Key]; ok {
			e = atomicWrite(path, b)
		} else {
			e = os.Remove(path)
			if errors.Is(e, os.ErrNotExist) {
				e = nil
			}
		}
		if e != nil {
			return e
		}
	}
	if err = writeJSON(s.syncPath(w, "state.json"), next); err != nil {
		return err
	}
	j.State = "committed"
	if err = writeJSON(filepath.Join(operation, "journal.json"), j); err != nil {
		return err
	}
	committed = true
	return nil
}
func (s *Service) rollbackSync(w Workspace, operation string, j syncJournal) error {
	for _, ch := range j.Changes {
		path, e := s.syncFilePath(w, ch.Key)
		if e != nil {
			return e
		}
		current, e := fileDigest(path)
		if e != nil {
			return e
		}
		if current != ch.BeforeHash && current != ch.AfterHash {
			return fmt.Errorf("同步恢复发现后续外部修改，文件及备份已保留：%s（%s）", ch.Key, operation)
		}
	}
	for _, ch := range j.Changes {
		path, e := s.syncFilePath(w, ch.Key)
		if e != nil {
			return e
		}
		if ch.BeforeHash == "" {
			e = os.Remove(path)
			if errors.Is(e, os.ErrNotExist) {
				e = nil
			}
		} else {
			var b []byte
			b, e = os.ReadFile(filepath.Join(operation, "before", filepath.FromSlash(ch.Key)))
			if e == nil {
				if digest(b) != ch.BeforeHash {
					return errors.New("同步恢复副本校验失败")
				}
				e = atomicWrite(path, b)
			}
		}
		if e != nil {
			return e
		}
	}
	if e := writeJSON(s.syncPath(w, "state.json"), j.Before); e != nil {
		return e
	}
	j.State = "rolled-back"
	return writeJSON(filepath.Join(operation, "journal.json"), j)
}
func (s *Service) recoverSyncs() error {
	for _, w := range s.settings.Workspaces {
		root := s.syncPath(w, "operations")
		dirs, e := os.ReadDir(root)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return e
		}
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			operation, e := safePath(root, d.Name())
			if e != nil {
				return e
			}
			var j syncJournal
			if e = readJSON(filepath.Join(operation, "journal.json"), &j); e != nil {
				return e
			}
			if j.State != "prepared" {
				continue
			}
			if e = s.rollbackSync(w, operation, j); e != nil {
				return e
			}
		}
	}
	return nil
}

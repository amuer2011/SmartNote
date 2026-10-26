package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type moveChange struct {
	Path       string `json:"path"`
	BeforeHash string `json:"beforeHash"`
	AfterHash  string `json:"afterHash"`
	Backup     string `json:"backup"`
}
type moveJournal struct {
	State         string       `json:"state"`
	WorkspaceID   string       `json:"workspaceId"`
	Source        string       `json:"source"`
	Target        string       `json:"target"`
	TargetHash    string       `json:"targetHash"`
	TargetCreated bool         `json:"targetCreated"`
	Changes       []moveChange `json:"changes"`
}
type MoveResult struct {
	Note         Note   `json:"note"`
	UpdatedLinks int    `json:"updatedLinks"`
	RecoveryPath string `json:"recoveryPath"`
}

func (s *Service) MoveNote(id, hash, target string) (MoveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moveNoteLocked(id, hash, target, nil)
}

// moveNoteLocked commits an optional edited source together with the rename and link updates.
func (s *Service) moveNoteLocked(id, hash, target string, content *string) (MoveResult, error) {
	out := MoveResult{}
	w, err := s.active()
	if err != nil {
		return out, err
	}
	notes, err := s.scan(w)
	if err != nil {
		return out, err
	}
	var source *Note
	for i := range notes {
		if notes[i].ID == id {
			source = &notes[i]
			break
		}
	}
	if source == nil {
		return out, errors.New("笔记不存在")
	}
	if source.Hash != hash {
		return out, errors.New("CONFLICT: 笔记已被外部修改，请刷新")
	}
	dest, err := safePath(w.Path, target)
	if err != nil {
		return out, err
	}
	if !strings.EqualFold(filepath.Ext(dest), ".md") {
		return out, errors.New("目标必须为 .md 文件")
	}
	rel, err := filepath.Rel(w.Path, dest)
	if err != nil {
		return out, err
	}
	target = filepath.ToSlash(rel)
	if err = visibleNotePath(target); err != nil {
		return out, err
	}
	if strings.EqualFold(source.Path, target) {
		return out, errors.New("目标与源路径相同；暂不支持仅修改路径大小写")
	}
	if _, err = os.Lstat(dest); err == nil {
		return out, errors.New("目标已存在，移动不会覆盖文件")
	} else if !errors.Is(err, os.ErrNotExist) {
		return out, err
	}
	changed := map[string]string{}
	links := 0
	for _, n := range notes {
		from := n.Path
		if n.ID == id {
			from = target
		}
		if n.ID == id && content != nil {
			n.Content = *content
		}
		edits := []textEdit{}
		for _, ref := range markdownLinks(n.Content) {
			linked := resolveNoteLink(n, ref, notes)
			if ref.Wiki {
				if linked == nil || (linked.ID != id && n.ID != id) {
					continue
				}
				fragment := ""
				if index := strings.IndexByte(ref.Target, '#'); index >= 0 {
					fragment = ref.Target[index:]
				}
				linkedPath := linked.Path
				if linked.ID == id {
					linkedPath = target
				}
				value := strings.TrimSuffix(linkedPath, ".md") + fragment
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
			if linked != nil && linked.ID == id {
				resolved = target
			}
			if n.ID != id && (linked == nil || linked.ID != id) {
				continue
			}
			relative, e := filepath.Rel(filepath.Dir(filepath.FromSlash(from)), filepath.FromSlash(resolved))
			if e != nil {
				return out, e
			}
			value := encodedPath(filepath.ToSlash(relative)) + suffix
			if value != ref.Target {
				edits = append(edits, textEdit{Start: ref.Start, End: ref.End, Value: value})
				links++
			}
		}
		text := applyEdits(n.Content, edits)
		if n.ID == id && n.SyntheticID {
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
		if n.ID == id && parseNote(target, []byte(text), time.Now()).ID != id {
			return out, errors.New("笔记 Front Matter 无法保留稳定 ID，请先修复 YAML 格式")
		}
		if text != n.Content || n.ID == id {
			changed[n.Path] = text
		}
	}
	recovery := filepath.Join(s.data(w), "operations", uid())
	journal := moveJournal{State: "prepared", WorkspaceID: w.ID, Source: source.Path, Target: target, TargetHash: digest([]byte(changed[source.Path])), Changes: []moveChange{}}
	for _, n := range notes {
		if text, ok := changed[n.Path]; ok {
			backup := fmt.Sprintf("%d.md", len(journal.Changes))
			if err = atomicWrite(filepath.Join(recovery, backup), []byte(n.Content)); err != nil {
				return out, err
			}
			journal.Changes = append(journal.Changes, moveChange{Path: n.Path, BeforeHash: n.Hash, AfterHash: digest([]byte(text)), Backup: backup})
		}
	}
	journalPath := filepath.Join(recovery, "journal.json")
	if err = writeJSON(journalPath, journal); err != nil {
		return out, err
	}
	fail := func(cause error) (MoveResult, error) {
		if e := s.rollbackMove(w, recovery, &journal); e != nil {
			return out, fmt.Errorf("%v；自动恢复未完成：%v。原文副本：%s", cause, e, recovery)
		}
		return out, fmt.Errorf("%v；已恢复移动前内容。恢复副本：%s", cause, recovery)
	}
	for _, change := range journal.Changes {
		p, e := safePath(w.Path, change.Path)
		if e != nil {
			return fail(e)
		}
		current, e := os.ReadFile(p)
		if e != nil || digest(current) != change.BeforeHash {
			return fail(errors.New("CONFLICT: 移动前检测到外部修改"))
		}
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return fail(err)
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fail(err)
	}
	journal.TargetCreated = true
	if err = writeJSON(journalPath, journal); err != nil {
		f.Close()
		return fail(err)
	}
	if _, err = f.Write([]byte(changed[source.Path])); err != nil {
		f.Close()
		return fail(err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return fail(err)
	}
	if err = f.Close(); err != nil {
		return fail(err)
	}
	for _, change := range journal.Changes {
		if change.Path == source.Path {
			continue
		}
		p, e := safePath(w.Path, change.Path)
		if e != nil {
			return fail(e)
		}
		current, e := os.ReadFile(p)
		if e != nil || digest(current) != change.BeforeHash {
			return fail(errors.New("CONFLICT: 更新链接时检测到外部修改"))
		}
		if err = atomicWrite(p, []byte(changed[change.Path])); err != nil {
			return fail(err)
		}
	}
	original, err := safePath(w.Path, source.Path)
	if err != nil {
		return fail(err)
	}
	current, err := os.ReadFile(original)
	if err != nil || digest(current) != source.Hash {
		return fail(errors.New("CONFLICT: 原文件变化，停止移动"))
	}
	if err = os.Remove(original); err != nil {
		return fail(err)
	}
	journal.State = "committed"
	if err = writeJSON(journalPath, journal); err != nil {
		return fail(err)
	}
	out = MoveResult{Note: parseNote(target, []byte(changed[source.Path]), time.Now()), UpdatedLinks: links, RecoveryPath: recovery}
	return out, nil
}
func writeExclusive(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (s *Service) rollbackMove(w Workspace, recovery string, journal *moveJournal) error {
	if !journal.TargetCreated {
		journal.State = "aborted"
		return writeJSON(filepath.Join(recovery, "journal.json"), journal)
	}
	for _, change := range journal.Changes {
		p, err := safePath(w.Path, change.Path)
		if err != nil {
			return err
		}
		backup, err := safePath(recovery, change.Backup)
		if err != nil {
			return err
		}
		before, err := os.ReadFile(backup)
		if err != nil || digest(before) != change.BeforeHash {
			return errors.New("恢复副本校验失败")
		}
		current, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			if change.Path != journal.Source {
				return fmt.Errorf("关联笔记已被外部移除: %s", change.Path)
			}
			if err = writeExclusive(p, before); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if digest(current) == change.AfterHash {
			if err = atomicWrite(p, before); err != nil {
				return err
			}
		} else if digest(current) != change.BeforeHash {
			return fmt.Errorf("保留外部修改，需手动恢复: %s", change.Path)
		}
	}
	target, err := safePath(w.Path, journal.Target)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(target)
	if err == nil {
		if digest(b) != journal.TargetHash {
			return errors.New("目标内容有变化，已保留双方文件，需手动处理")
		}
		if err = os.Remove(target); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	journal.State = "rolled-back"
	return writeJSON(filepath.Join(recovery, "journal.json"), journal)
}
func (s *Service) recoverMoves() error {
	for _, w := range s.settings.Workspaces {
		root := filepath.Join(s.data(w), "operations")
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
			journalPath := filepath.Join(recovery, "journal.json")
			if _, err = os.Stat(journalPath); errors.Is(err, os.ErrNotExist) {
				continue
			}
			var journal moveJournal
			if err = readJSON(journalPath, &journal); err != nil {
				return err
			}
			if journal.State != "prepared" {
				continue
			}
			if journal.WorkspaceID != w.ID {
				return errors.New("移动恢复记录不属于此工作区")
			}
			if err = s.rollbackMove(w, recovery, &journal); err != nil {
				return fmt.Errorf("未完成的移动需要处理: %s: %w", recovery, err)
			}
		}
	}
	return nil
}

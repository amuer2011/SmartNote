package core

import (
	"errors"
	"path/filepath"
	"time"
)

func needsTaskConfirmation(t Task) bool {
	return t.Inferred && t.IntentDecision == "pending" && !t.Done
}

type TaskIntentDecision struct {
	WorkspaceID    string `json:"workspaceId"`
	TaskID         string `json:"taskId"`
	SourceRevision string `json:"sourceRevision"`
	Until          string `json:"until,omitempty"`
	Accept         bool   `json:"accept"`
	IgnoreNote     bool   `json:"ignoreNote,omitempty"`
}

type TaskIntentRef struct {
	TaskID         string `json:"taskId"`
	SourceRevision string `json:"sourceRevision"`
}

type TaskIntentBatch struct {
	WorkspaceID string          `json:"workspaceId"`
	Tasks       []TaskIntentRef `json:"tasks"`
}

// Ignore only the pending revisions the user saw. Validate the whole selection
// before committing once, so conflicts cannot leave a partially ignored queue.
func (s *Service) IgnoreAllTaskIntents(input TaskIntentBatch) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(input.WorkspaceID)
	if err != nil {
		return 0, err
	}
	selected := map[string]string{}
	for _, item := range input.Tasks {
		if item.TaskID == "" || item.SourceRevision == "" {
			return 0, errors.New("识别结果无效，请刷新后重试")
		}
		if revision, exists := selected[item.TaskID]; exists && revision != item.SourceRevision {
			return 0, errors.New("识别结果存在冲突，请刷新后重试")
		}
		selected[item.TaskID] = item.SourceRevision
	}
	if len(selected) == 0 {
		return 0, nil
	}
	notes, err := s.scan(w)
	if err != nil {
		return 0, err
	}
	meta := map[string]Task{}
	if err = readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &meta); err != nil {
		return 0, err
	}
	count := 0
	for _, note := range notes {
		for _, task := range tasksFor(note) {
			revision, exists := selected[task.ID]
			if !exists || !task.Inferred {
				continue
			}
			if task.SourceRevision != revision {
				return 0, errors.New("笔记内容已变化，请确认最新识别结果后重试；本次未忽略任何笔记")
			}
			if stored, ok := meta[task.ID]; ok {
				task = mergeTask(task, stored)
			}
			if task.Deleted || !needsTaskConfirmation(task) {
				return 0, errors.New("待确认状态已变化，请刷新后重试；本次未忽略任何笔记")
			}
			task.IntentDecision, task.IgnoreNoteAutomation = "rejected", true
			task.Candidate, task.NeedsConfirmation = false, false
			meta[task.ID] = task
			count++
		}
	}
	if count != len(selected) {
		return 0, errors.New("识别结果已失效，请刷新后重试；本次未忽略任何笔记")
	}
	if err = s.commitTaskMetadata(w, meta); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) ResolveTaskIntent(input TaskIntentDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.IgnoreNote && input.Accept {
		return errors.New("不能同时加入待办和忽略笔记")
	}
	w, e := s.archiveWorkspace(input.WorkspaceID)
	if e != nil {
		return e
	}
	notes, e := s.scan(w)
	if e != nil {
		return e
	}
	meta := map[string]Task{}
	if e = readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &meta); e != nil {
		return e
	}
	for _, n := range notes {
		for _, t := range tasksFor(n) {
			if t.ID != input.TaskID || !t.Inferred {
				continue
			}
			if t.SourceRevision != input.SourceRevision {
				return errors.New("笔记内容已变化，请确认最新识别结果")
			}
			if stored, ok := meta[t.ID]; ok {
				t = mergeTask(t, stored)
			}
			if t.Deleted {
				return errors.New("待办已移除")
			}
			if t.Done {
				return errors.New("待办已完成")
			}
			if t.IgnoreNoteAutomation {
				return errors.New("此笔记已忽略，请先从笔记右键菜单恢复识别")
			}
			t.IntentDecision = "rejected"
			t.IgnoreNoteAutomation = input.IgnoreNote
			if input.Accept {
				if input.Until != "" {
					if t.Recurrence == nil {
						return errors.New("只有循环待办可以设置截止日期")
					}
					if _, err := time.Parse("2006-01-02", input.Until); err != nil {
						return errors.New("截止日期格式无效")
					}
					if input.Until < t.Recurrence.Anchor {
						return errors.New("截止日期不能早于循环开始日期")
					}
				}
				if t.Recurrence != nil {
					r := *t.Recurrence
					r.Until = input.Until
					t.Recurrence = &r
				}
				t.IntentDecision = "accepted"
			}
			t.Candidate = false
			t.NeedsConfirmation = false
			meta[t.ID] = t
			return s.commitTaskMetadata(w, meta)
		}
	}
	return errors.New("识别结果已失效，请刷新笔记")
}

// The stable intent record also carries the note-level automation preference.
// Keeping it in task metadata preserves it in native backups and WebDAV sync.
func (s *Service) SetNoteAutomationIgnored(workspaceID, noteID, hash string, ignored bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return err
	}
	n, err := s.archiveNote(w, noteID, hash)
	if err != nil {
		return err
	}
	meta := map[string]Task{}
	if err = readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &meta); err != nil {
		return err
	}
	id := n.ID + "_intent"
	t := meta[id]
	if !ignored && !t.IgnoreNoteAutomation {
		return nil
	}
	for _, fresh := range tasksFor(n) {
		if fresh.Inferred {
			fresh.History = t.History
			t = fresh
			break
		}
	}
	t.ID, t.NoteID = id, n.ID
	t.IgnoreNoteAutomation = ignored
	if ignored {
		t.IntentDecision = "rejected"
	}
	meta[id] = t
	return s.commitTaskMetadata(w, meta)
}

package core

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type TaskArchiveRecord struct {
	NoteContent    string `json:"noteContent,omitempty"`
	ID             string `json:"id"`
	TaskID         string `json:"taskId"`
	NoteID         string `json:"noteId"`
	NoteTitle      string `json:"noteTitle"`
	Title          string `json:"title"`
	NotePath       string `json:"notePath"`
	DueAt          string `json:"dueAt"`
	CompletedAt    string `json:"completedAt"`
	ReopenedAt     string `json:"reopenedAt,omitempty"`
	Imported       bool   `json:"imported,omitempty"`
	SourceRevision string `json:"sourceRevision,omitempty"`
	Line           int    `json:"line"`
	Priority       string `json:"priority"`
}

func noteTaskRevision(n Note) string {
	return digest([]byte(n.Title + "\n" + strings.ReplaceAll(archiveBody(n.Content), "\r\n", "\n")))
}
func taskHistory(t Task, date string, done bool, n Note, imported bool) Task {
	for i := len(t.History) - 1; i >= 0; i-- {
		h := t.History[i]
		if h.SourceRevision == t.SourceRevision && (h.DueAt == date || t.Recurrence == nil) && h.ReopenedAt == "" {
			if !done {
				t.History = append([]TaskArchiveRecord{}, t.History...)
				t.History[i].ReopenedAt = time.Now().Format(time.RFC3339Nano)
			}
			return t
		}
	}
	if !done {
		return t
	}
	at := time.Now().Format(time.RFC3339Nano)
	h := TaskArchiveRecord{ID: uid(), TaskID: t.ID, NoteID: t.NoteID, NoteTitle: t.NoteTitle, Title: t.Title, NotePath: n.Path, NoteContent: n.Content, DueAt: date, CompletedAt: at, Imported: imported, SourceRevision: t.SourceRevision, Line: t.Line, Priority: t.Priority}
	t.History = append(append([]TaskArchiveRecord{}, t.History...), h)
	return t
}

// Recalculate on an explicit edit, preserving immutable completion history.
func (s *Service) recalculateNoteTasks(w Workspace, n Note) error {
	path := filepath.Join(s.data(w), "tasks-metadata.json")
	meta := map[string]Task{}
	if e := readJSON(path, &meta); e != nil {
		return e
	}
	base := n
	base.Created = time.Now()
	for _, t := range tasksFor(base) {
		old, exists := meta[t.ID]
		if t.Inferred {
			t.IgnoreNoteAutomation = old.IgnoreNoteAutomation
			if t.IgnoreNoteAutomation {
				t.IntentDecision = "rejected"
			}
			t.History = old.History
			t.Deleted = old.Deleted
			if old.Priority != "" {
				t.Priority = old.Priority
			}
			meta[t.ID] = t
		} else {
			if exists {
				t = mergeTask(t, old)
			}
			t = taskHistory(t, t.DueAt, t.Done, n, false)
			meta[t.ID] = t
		}
	}
	return s.commitTaskMetadata(w, meta)
}
func archiveRecords(meta map[string]Task) []TaskArchiveRecord {
	out := []TaskArchiveRecord{}
	seen := map[string]bool{}
	for _, t := range meta {
		for _, h := range t.History {
			if h.ID != "" && !seen[h.ID] {
				seen[h.ID] = true
				out = append(out, h)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CompletedAt > out[j].CompletedAt })
	return out
}

// The metadata ledger is committed with task state. Monthly files are repairable
// projections, so a failed file write never loses a completion record.
func (s *Service) projectTaskArchive(w Workspace, records []TaskArchiveRecord) error {
	groups := map[string][]TaskArchiveRecord{}
	for _, r := range records {
		d, e := time.Parse(time.RFC3339Nano, r.CompletedAt)
		if e != nil {
			return errors.New("完成记录时间格式无效")
		}
		key := d.Format("2006/01")
		groups[key] = append(groups[key], r)
	}
	for key, list := range groups {
		path := filepath.Join(s.data(w), "task-archive", filepath.FromSlash(key)+".json")
		var cached []TaskArchiveRecord
		if readJSON(path, &cached) == nil && sameArchive(cached, list) {
			continue
		}
		if e := writeJSON(path, list); e != nil {
			return e
		}
	}
	return s.materializeTaskArchive(w, records)
}
func sameArchive(a, b []TaskArchiveRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func (s *Service) GetTaskArchive(workspaceID string) ([]TaskArchiveRecord, error) {
	// Snapshot also migrates pre-archive completions without requiring a note edit.
	snap, e := s.Snapshot()
	if e != nil {
		return nil, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.active()
	if e != nil {
		return nil, e
	}
	if w.ID != workspaceID || snap.Settings.ActiveID != workspaceID {
		return nil, errors.New("工作区已切换")
	}
	meta := map[string]Task{}
	if e = readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &meta); e != nil {
		return nil, e
	}
	out := archiveRecords(meta)
	return out, s.projectTaskArchive(w, out)
}

// Commit the authoritative ledger first; visible files can be repaired on refresh.
func (s *Service) commitTaskMetadata(w Workspace, meta map[string]Task) error {
	if e := writeJSON(filepath.Join(s.data(w), "tasks-metadata.json"), meta); e != nil {
		return e
	}
	return s.projectTaskArchive(w, archiveRecords(meta))
}

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompletedArchiveSurvivesEditsAndDeletion(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("明天提醒我充值话费", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	id := tasksFor(n)[0].ID
	if e = s.SetTask(id, true, false); e != nil {
		t.Fatal(e)
	}
	records, e := s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 1 {
		t.Fatalf("%+v %v", records, e)
	}
	original := records[0]
	if original.Imported || original.Title != n.Title {
		t.Fatalf("%+v", original)
	}
	at, _ := time.Parse(time.RFC3339Nano, original.CompletedAt)
	path := filepath.Join(s.data(w), "task-archive", at.Format("2006"), at.Format("01")+".json")
	if _, e = os.Stat(path); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	for _, note := range snap.Notes {
		if note.ID == n.ID {
			n = note
		}
	}
	n, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: strings.ReplaceAll(n.Content, "明天提醒我充值话费", "180天后提醒我充值话费")})
	if e != nil {
		t.Fatal(e)
	}
	snap, e = s.Snapshot()
	if e != nil || len(snap.Tasks) != 0 {
		t.Fatal("archive edit recreated task", e)
	}
	if e = s.DeleteNote(n.ID, n.Hash); e != nil {
		t.Fatal(e)
	}
	records, e = s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 1 || records[0] != original {
		t.Fatalf("history changed: %+v %v", records, e)
	}
	snap, _ = s.Snapshot()
	if len(snap.Tasks) != 0 {
		t.Fatal("deleted source task remains active")
	}
	if e = os.WriteFile(path, []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.GetTaskArchive(w.ID); e != nil {
		t.Fatal(e)
	}
	var repaired []TaskArchiveRecord
	if e = readJSON(path, &repaired); e != nil || len(repaired) != 1 {
		t.Fatal("monthly projection not repaired", e)
	}
}
func TestEditResetsManualDeadlineAndCompletedCycle(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("每周五提醒我检查", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	tsk := tasksFor(n)[0]
	if e = s.SetTaskOccurrence(w.ID, tsk.ID, tsk.DueAt, true); e != nil {
		t.Fatal(e)
	}
	if e = s.UpdateTaskDetails(TaskDetails{ID: tsk.ID, DueAt: "2030-01-01", Priority: "high"}); e != nil {
		t.Fatal(e)
	}
	n, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n每2天提醒我检查"})
	if e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	task := snap.Tasks[0]
	if task.Recurrence == nil || task.Recurrence.Interval != 2 || task.Recurrence.Unit != "day" || task.DueAt != time.Now().Format("2006-01-02") || len(task.CompletedDates) != 0 || task.Priority != "high" {
		t.Fatalf("%+v", task)
	}
	records, e := s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 1 {
		t.Fatalf("%+v %v", records, e)
	}
}
func TestArchiveCompletionUndoAndMigration(t *testing.T) {
	s, w := fixture(t)
	n, _ := s.CreateNote("今天提醒我检查", "Inbox")
	n, e := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n- [ ] 保留的其他事项"})
	if e != nil {
		t.Fatal(e)
	}
	id := n.ID + "_intent"
	if e := s.SetTask(id, true, false); e != nil {
		t.Fatal(e)
	}
	if e := s.SetTask(id, true, false); e != nil {
		t.Fatal(e)
	}
	records, e := s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 1 {
		t.Fatalf("duplicate completion: %+v %v", records, e)
	}
	if e = s.SetTask(id, false, false); e != nil {
		t.Fatal(e)
	}
	records, e = s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 1 || records[0].ReopenedAt == "" {
		t.Fatal("undo not recorded", e)
	}
	if e = s.SetTask(id, true, false); e != nil {
		t.Fatal(e)
	}
	records, e = s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 2 {
		t.Fatal("second completion not retained", e)
	}
	old := Task{ID: id, Done: true, DueAt: "2025-05-01", Priority: "normal"}
	if e = writeJSON(filepath.Join(s.data(w), "tasks-metadata.json"), map[string]Task{id: old}); e != nil {
		t.Fatal(e)
	}
	records, e = s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 1 || !records[0].Imported {
		t.Fatalf("migration %+v %v", records, e)
	}
}
func TestExplicitCheckboxEditorCompletionArchived(t *testing.T) {
	s, w := fixture(t)
	n, _ := s.CreateNote("检查记录", "Inbox")
	n, e := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n- [x] 检查数据库\n- [ ] 其他未完成事项"})
	if e != nil {
		t.Fatal(e)
	}
	rows, e := s.GetTaskArchive(w.ID)
	if e != nil || len(rows) != 1 || rows[0].Imported {
		t.Fatalf("%+v %v", rows, e)
	}
	_, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: strings.Replace(n.Content, "[x]", "[ ]", 1)})
	if e != nil {
		t.Fatal(e)
	}
	rows, e = s.GetTaskArchive(w.ID)
	if e != nil || rows[0].ReopenedAt == "" {
		t.Fatalf("editor undo: %+v %v", rows, e)
	}
}

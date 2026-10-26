package core

import (
	"path/filepath"
	"testing"
)

func TestIgnoreAllPendingTaskIntents(t *testing.T) {
	s, w := fixture(t)
	first := mustNote(t, s, "明天提醒我充值话费")
	second := mustNote(t, s, "每周提醒我检查数据库")
	accepted := mustNote(t, s, "今天提醒我开会")
	acceptedTask := tasksFor(accepted)[0]
	if err := s.ResolveTaskIntent(TaskIntentDecision{WorkspaceID: w.ID, TaskID: acceptedTask.ID, SourceRevision: acceptedTask.SourceRevision, Accept: true}); err != nil {
		t.Fatal(err)
	}
	manual := mustNote(t, s, "会议记录")
	if _, err := s.SaveNote(SaveInput{Path: manual.Path, Hash: manual.Hash, Content: manual.Content + "\n- [ ] 准备材料\n"}); err != nil {
		t.Fatal(err)
	}
	input := TaskIntentBatch{WorkspaceID: w.ID}
	for _, n := range []Note{first, second} {
		task := tasksFor(n)[0]
		input.Tasks = append(input.Tasks, TaskIntentRef{TaskID: task.ID, SourceRevision: task.SourceRevision})
	}
	// Validation of every requested revision happens before any state is written.
	stale := input
	stale.Tasks = append([]TaskIntentRef{}, input.Tasks...)
	stale.Tasks[1].SourceRevision = "outdated"
	if _, err := s.IgnoreAllTaskIntents(stale); err == nil {
		t.Fatal("stale batch accepted")
	}
	changedState := input
	changedState.Tasks = append(append([]TaskIntentRef{}, input.Tasks...), TaskIntentRef{TaskID: acceptedTask.ID, SourceRevision: acceptedTask.SourceRevision})
	if _, err := s.IgnoreAllTaskIntents(changedState); err == nil {
		t.Fatal("accepted task was included in ignore batch")
	}
	missing := input
	missing.Tasks = append(append([]TaskIntentRef{}, input.Tasks...), TaskIntentRef{TaskID: "missing", SourceRevision: "missing"})
	if _, err := s.IgnoreAllTaskIntents(missing); err == nil {
		t.Fatal("missing task accepted")
	}
	before, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range before.Notes {
		if n.AutomationIgnored {
			t.Fatal("failed batch partially ignored notes")
		}
	}
	// Duplicate references must not cause duplicate processing.
	input.Tasks = append(input.Tasks, input.Tasks[0])
	if count, err := s.IgnoreAllTaskIntents(input); err != nil || count != 2 {
		t.Fatalf("ignore batch: %d %v", count, err)
	}
	if err := s.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	after, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Tasks) != 2 {
		t.Fatalf("existing accepted or Markdown tasks changed: %+v", after.Tasks)
	}
	for _, task := range after.Tasks {
		if task.NeedsConfirmation {
			t.Fatal("ignored result still pending")
		}
	}
	for _, n := range after.Notes {
		want := n.ID == first.ID || n.ID == second.ID
		if n.AutomationIgnored != want {
			t.Fatalf("wrong ignore scope: %+v", n)
		}
	}
	for _, n := range []Note{first, second, accepted} {
		if got := workspaceFile(t, w.Path, n.Path); got != n.Content {
			t.Fatal("ignore rewrote Markdown", n.Path)
		}
	}
	first, err = s.SaveNote(SaveInput{Path: first.Path, Hash: first.Hash, Content: first.Content + "\n下周提醒我充值话费"})
	if err != nil {
		t.Fatal(err)
	}
	if moved, err := s.AutoArchive(w.ID, first.ID, first.Hash); err != nil || moved != nil {
		t.Fatal("batch ignored note classified", err)
	}
	if err := s.SetNoteAutomationIgnored(w.ID, first.ID, first.Hash, false); err != nil {
		t.Fatal(err)
	}
	after, err = s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	pending := 0
	for _, task := range after.Tasks {
		if task.NeedsConfirmation {
			pending++
		}
	}
	if pending != 1 {
		t.Fatal("individual restore did not restore one note", pending)
	}
}

func TestIgnoreAllTaskIntentsWorkspaceAndEmptySelection(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "明天提醒我开会")
	task := tasksFor(n)[0]
	if count, err := s.IgnoreAllTaskIntents(TaskIntentBatch{WorkspaceID: w.ID}); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	other, err := s.AddWorkspace("其他", filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatal(err)
	}
	mustNote(t, s, "明天提醒我检查数据库")
	if _, err := s.IgnoreAllTaskIntents(TaskIntentBatch{WorkspaceID: w.ID, Tasks: []TaskIntentRef{{TaskID: task.ID, SourceRevision: task.SourceRevision}}}); err == nil {
		t.Fatal("old workspace batch accepted")
	}
	snapshot, err := s.Snapshot()
	if err != nil || snapshot.Settings.ActiveID != other.ID || len(snapshot.Tasks) != 1 || !snapshot.Tasks[0].NeedsConfirmation || snapshot.Notes[0].AutomationIgnored {
		t.Fatal("another workspace was changed", err)
	}
}

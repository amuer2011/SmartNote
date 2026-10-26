package core

import (
	"path/filepath"
	"testing"
)

func TestIgnoreNoteSurvivesEditMoveBackupAndResume(t *testing.T) {
	s, w, _ := semanticFixture(t, [][]float64{{1, 0}, {1, 0}, {0, 1}})
	n := mustNote(t, s, "明天提醒我检查数据库")
	task := tasksFor(n)[0]
	decision := TaskIntentDecision{WorkspaceID: w.ID, TaskID: task.ID, SourceRevision: task.SourceRevision, IgnoreNote: true}
	stale := decision
	stale.SourceRevision = "stale"
	if s.ResolveTaskIntent(stale) == nil {
		t.Fatal("stale ignore accepted")
	}
	if err := s.ResolveTaskIntent(decision); err != nil {
		t.Fatal(err)
	}
	if got := workspaceFile(t, w.Path, n.Path); got != n.Content {
		t.Fatal("ignore modified note")
	}
	assertIgnored := func() {
		t.Helper()
		snapshot, err := s.Snapshot()
		if err != nil || len(snapshot.Tasks) != 0 || len(snapshot.Notes) != 1 || !snapshot.Notes[0].AutomationIgnored {
			t.Fatal("ignore lost", snapshot.Tasks, err)
		}
		if result, err := s.AutoArchive(w.ID, n.ID, n.Hash); err != nil || result != nil {
			t.Fatal("ignored note auto-classified", result, err)
		}
	}
	assertIgnored()
	var err error
	n, err = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n每周提醒我维护数据库"})
	if err != nil {
		t.Fatal(err)
	}
	assertIgnored()
	moved, err := s.MoveNote(n.ID, n.Hash, "其他目录/提醒.md")
	if err != nil {
		t.Fatal(err)
	}
	n = moved.Note
	assertIgnored()
	backup := filepath.Join(t.TempDir(), "ignored.zip")
	if err = s.Export(backup, true); err != nil {
		t.Fatal(err)
	}
	restored, err := s.Import(backup, filepath.Join(t.TempDir(), "restored"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil || len(snap.Tasks) != 0 || !snap.Notes[0].AutomationIgnored {
		t.Fatal("backup lost ignore", err)
	}
	if err = s.SetNoteAutomationIgnored(w.ID, n.ID, n.Hash, false); err == nil {
		t.Fatal("wrong workspace allowed")
	}
	if err = s.SetNoteAutomationIgnored(restored.ID, n.ID, "stale", false); err == nil {
		t.Fatal("stale restore allowed")
	}
	if err = s.SetNoteAutomationIgnored(restored.ID, n.ID, n.Hash, false); err != nil {
		t.Fatal(err)
	}
	snap, err = s.Snapshot()
	if err != nil || len(snap.Tasks) != 1 || !snap.Tasks[0].NeedsConfirmation || snap.Notes[0].AutomationIgnored {
		t.Fatal("restore failed", snap.Tasks, err)
	}
	if result, err := s.AutoArchive(restored.ID, n.ID, n.Hash); err != nil || result == nil {
		t.Fatal("classification not restored", result, err)
	}
}

func TestIgnoreDuringInferencePreventsAutomaticMove(t *testing.T) {
	s, w, _ := semanticFixture(t, [][]float64{{1, 0}, {1, 0}, {0, 1}})
	n := mustNote(t, s, "明天提醒我维护数据库")
	task := tasksFor(n)[0]
	s.embedTexts = func(texts []string) ([][]float64, error) {
		if err := s.ResolveTaskIntent(TaskIntentDecision{WorkspaceID: w.ID, TaskID: task.ID, SourceRevision: task.SourceRevision, IgnoreNote: true}); err != nil {
			t.Fatal(err)
		}
		return [][]float64{{1, 0}, {1, 0}, {0, 1}}, nil
	}
	if result, err := s.AutoArchive(w.ID, n.ID, n.Hash); err != nil || result != nil {
		t.Fatal("in-flight inference moved ignored note", result, err)
	}
	if got := workspaceFile(t, w.Path, n.Path); got != n.Content {
		t.Fatal("in-flight ignore changed note")
	}
}

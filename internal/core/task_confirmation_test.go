package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestInferredTaskRequiresConfirmationAndRejectsStaleDecisions(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("今天提醒我开会", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	if len(snap.Tasks) != 1 || !snap.Tasks[0].NeedsConfirmation || !snap.Tasks[0].Candidate {
		t.Fatalf("automatic task escaped confirmation: %+v", snap.Tasks)
	}
	task := snap.Tasks[0]
	data := reminderStore{Config: ReminderConfig{Enabled: true, Time: "00:00", LeadDays: 0}}
	if reminders := dueReminders(snap.Tasks, data, time.Now()); len(reminders) != 0 {
		t.Fatal("unconfirmed task triggered a reminder")
	}
	decision := TaskIntentDecision{WorkspaceID: w.ID, TaskID: task.ID, SourceRevision: task.SourceRevision, Accept: true}
	stale := decision
	stale.SourceRevision = "stale"
	if e = s.ResolveTaskIntent(stale); e == nil {
		t.Fatal("stale confirmation accepted")
	}
	if e = s.ResolveTaskIntent(decision); e != nil {
		t.Fatal(e)
	}
	snap, e = s.Snapshot()
	if e != nil || len(snap.Tasks) != 1 || snap.Tasks[0].NeedsConfirmation || snap.Tasks[0].Candidate {
		t.Fatalf("confirmation not applied: %+v %v", snap.Tasks, e)
	}
	if workspaceFile(t, w.Path, n.Path) != n.Content {
		t.Fatal("confirmation changed original note")
	}
	// Re-reading persisted metadata must not require confirmation again.
	if e = s.OpenWorkspace(w.ID); e != nil {
		t.Fatal(e)
	}
	snap, e = s.Snapshot()
	if e != nil || snap.Tasks[0].NeedsConfirmation {
		t.Fatal("confirmation not persistent", e)
	}
	n, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n明天提醒我开会"})
	if e != nil {
		t.Fatal(e)
	}
	snap, e = s.Snapshot()
	if e != nil || !snap.Tasks[0].NeedsConfirmation {
		t.Fatal("edited task did not request confirmation", e)
	}
	if e = s.ResolveTaskIntent(decision); e == nil {
		t.Fatal("old revision accepted after editing")
	}
	decision.SourceRevision = snap.Tasks[0].SourceRevision
	decision.Accept = false
	if e = s.ResolveTaskIntent(decision); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		snap, e = s.Snapshot()
		if e != nil || len(snap.Tasks) != 0 {
			t.Fatal("ignored task reappeared", e)
		}
	}
	n, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n每周提醒我开会"})
	if e != nil {
		t.Fatal(e)
	}
	snap, e = s.Snapshot()
	if e != nil || len(snap.Tasks) != 1 || !snap.Tasks[0].NeedsConfirmation {
		t.Fatal("edited rejected task was not recognized again", e)
	}
}

func TestLegacyTaskConfirmationMigrationAndWorkspaceSafety(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("每周提醒我检查", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	legacy := tasksFor(n)[0]
	legacy.IntentDecision = ""
	if e = writeJSON(filepath.Join(s.data(w), "tasks-metadata.json"), map[string]Task{legacy.ID: legacy}); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot()
	if e != nil || snap.Tasks[0].NeedsConfirmation || snap.Tasks[0].Candidate {
		t.Fatal("existing confirmed task was reset", e)
	}
	if _, e = s.AddWorkspace("其他", filepath.Join(t.TempDir(), "other")); e != nil {
		t.Fatal(e)
	}
	if e = s.ResolveTaskIntent(TaskIntentDecision{WorkspaceID: w.ID, TaskID: legacy.ID, SourceRevision: legacy.SourceRevision, Accept: false}); e == nil {
		t.Fatal("stale workspace changed")
	}
}

func TestRecurringConfirmationUntil(t *testing.T) {
	s, w := fixture(t)
	_, err := s.CreateNote("每周提醒我开会", "Inbox")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil || len(snap.Tasks) != 1 {
		t.Fatal(err, snap.Tasks)
	}
	task := snap.Tasks[0]
	if task.Recurrence == nil {
		t.Fatal("missing recurrence")
	}
	decision := TaskIntentDecision{WorkspaceID: w.ID, TaskID: task.ID, SourceRevision: task.SourceRevision, Accept: true, Until: "bad-date"}
	if s.ResolveTaskIntent(decision) == nil {
		t.Fatal("invalid date accepted")
	}
	decision.Until = "1900-01-01"
	if s.ResolveTaskIntent(decision) == nil {
		t.Fatal("end before start accepted")
	}
	decision.Until = task.Recurrence.Anchor
	if err = s.ResolveTaskIntent(decision); err != nil {
		t.Fatal(err)
	}
	snap, err = s.Snapshot()
	if err != nil || snap.Tasks[0].Recurrence.Until != decision.Until {
		t.Fatal("until not persisted", err)
	}
	task = snap.Tasks[0]
	if occurrence(task.Recurrence, 0) != decision.Until || occurrence(task.Recurrence, 1) != "" {
		t.Fatal("inclusive until not respected")
	}
	if openOccurrence(task, occurrence(&Recurrence{Unit: "week", Interval: 1, Anchor: decision.Until}, 1)) {
		t.Fatal("out of range occurrence allowed")
	}
	completed, err := completeOccurrence(task, decision.Until, true)
	if err != nil || !completed.Done {
		t.Fatal("final completion failed", err)
	}
	reopened, err := completeOccurrence(completed, decision.Until, false)
	if err != nil || reopened.Done {
		t.Fatal("reopen failed", err)
	}
	unlimited := *task.Recurrence
	unlimited.Until = ""
	if occurrence(&unlimited, 100) == "" {
		t.Fatal("empty until must be unlimited")
	}
}

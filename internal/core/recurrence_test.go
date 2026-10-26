package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRelativeAndRecurringIntent(t *testing.T) {
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.Local)
	cases := map[string]string{"180天后提醒我充值手机话费": base.AddDate(0, 0, 180).Format("2006-01-02"), "2周后提醒我续费": "2026-10-30", "每周一提醒我开会": "2026-10-19", "每月15号提醒我还款": "2026-11-15", "每年10月21日提醒我体检": "2026-10-21", "每3天提醒我检查": "2026-10-16", "每两周提醒我整理": "2026-10-16", "每2年提醒我体检": "2026-10-16"}
	for title, want := range cases {
		n := parseNote("test.md", []byte("# "+title), base)
		n.Created = base
		tasks := tasksFor(n)
		if len(tasks) != 1 || tasks[0].DueAt != want {
			t.Fatalf("%s %+v want %s", title, tasks, want)
		}
	}
}
func TestRecurringMonthAndLeapAnchors(t *testing.T) {
	r := &Recurrence{Unit: "month", Interval: 1, Anchor: "2026-01-31", Day: 31}
	for i, want := range []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"} {
		if got := occurrence(r, i); got != want {
			t.Fatalf("%s != %s", got, want)
		}
	}
	r = &Recurrence{Unit: "year", Interval: 1, Anchor: "2024-02-29", Day: 29}
	if occurrence(r, 1) != "2025-02-28" || occurrence(r, 4) != "2028-02-29" {
		t.Fatal("leap drift")
	}
}
func TestRecurringCompletionRemovalAndRestore(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("每周五提醒我整理资料", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	source := workspaceFile(t, w.Path, n.Path)
	task := tasksFor(n)[0]
	first := task.DueAt
	if e = s.SetTaskOccurrence(w.ID, task.ID, first, true); e != nil {
		t.Fatal(e)
	}
	if e = s.SetTaskOccurrence(w.ID, task.ID, first, true); e != nil {
		t.Fatal(e)
	} // Duplicate completion must not skip a second occurrence.
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	next := snap.Tasks[0]
	if next.Done || next.DueAt != occurrence(task.Recurrence, 1) || len(next.CompletedDates) != 1 {
		t.Fatalf("%+v", next)
	}
	if e = s.UpdateTaskDetails(TaskDetails{ID: task.ID, DueAt: next.DueAt, Priority: "high"}); e != nil {
		t.Fatal(e)
	}
	snap, _ = s.Snapshot()
	if len(snap.Tasks[0].CompletedDates) != 1 {
		t.Fatal("priority edit erased history")
	}
	if e = s.SetRecurringEnabled(w.ID, []string{task.ID}, false); e != nil {
		t.Fatal(e)
	}
	snap, _ = s.Snapshot()
	if len(snap.Tasks) != 0 {
		t.Fatal("removed series reappeared")
	}
	if e = s.SetRecurringEnabled("wrong", []string{task.ID}, true); e == nil {
		t.Fatal("workspace switch accepted")
	}
	if e = s.SetRecurringEnabled(w.ID, []string{task.ID, "missing"}, true); e == nil {
		t.Fatal("invalid batch accepted")
	}
	snap, _ = s.Snapshot()
	if len(snap.Tasks) != 0 {
		t.Fatal("invalid batch partially applied")
	}
	if e = s.SetRecurringEnabled(w.ID, []string{task.ID}, true); e != nil {
		t.Fatal(e)
	}
	if e = s.SetTaskOccurrence(w.ID, task.ID, first, false); e != nil {
		t.Fatal(e)
	}
	snap, _ = s.Snapshot()
	if snap.Tasks[0].DueAt != first || len(snap.Tasks[0].CompletedDates) != 0 {
		t.Fatal("undo completion failed")
	}
	if workspaceFile(t, w.Path, n.Path) != source {
		t.Fatal("recurrence changed source")
	}
	meta := map[string]Task{}
	if e = readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &meta); e != nil || meta[task.ID].Recurrence == nil {
		t.Fatal("rule not persisted", e)
	}
}

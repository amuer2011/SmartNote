package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDueRemindersTimeLeadDismissAndSnooze(t *testing.T) {
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, time.FixedZone("local", 8*3600))
	data := defaultReminders()
	tasks := []Task{{ID: "overdue", DueAt: "2026-10-18"}, {ID: "today", DueAt: "2026-10-19"}, {ID: "future", DueAt: "2026-10-20"}, {ID: "done", DueAt: "2026-10-19", Done: true}, {ID: "candidate", DueAt: "2026-10-19", Candidate: true}, {ID: "invalid", DueAt: "2026-13-01"}}
	if got := dueReminders(tasks, data, now.Add(-time.Minute)); len(got) != 0 {
		t.Fatal("reminded before configured time")
	}
	if got := dueReminders(tasks, data, now); len(got) != 2 || got[0].ID != "overdue" {
		t.Fatalf("wrong due tasks %+v", got)
	}
	data.Config.LeadDays = 1
	if len(dueReminders(tasks, data, now)) != 3 {
		t.Fatal("advance reminder missing")
	}
	data.Receipts[reminderKey(tasks[1])] = reminderReceipt{Day: "2026-10-19"}
	if len(dueReminders(tasks, data, now)) != 2 {
		t.Fatal("dismiss not applied")
	}
	data.Receipts[reminderKey(tasks[0])] = reminderReceipt{Until: now.Add(15 * time.Minute).Format(time.RFC3339)}
	if len(dueReminders(tasks, data, now)) != 1 {
		t.Fatal("snooze not applied")
	}
	if len(dueReminders(tasks, data, now.Add(16*time.Minute))) != 2 {
		t.Fatal("snooze never resumed")
	}
	if len(dueReminders(tasks, data, now.AddDate(0, 0, 1))) != 3 {
		t.Fatal("dismiss persisted beyond today")
	}
	changed := []Task{{ID: "today", DueAt: "2026-10-18"}}
	if len(dueReminders(changed, data, now)) != 1 {
		t.Fatal("deadline change suppressed")
	}
	data.Config.Enabled = false
	if len(dueReminders(tasks, data, now)) != 0 {
		t.Fatal("disabled reminder fired")
	}
}
func TestRemindersPersistAndRestoreWithBackup(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "提醒测试")
	today := time.Now().Format("2006-01-02")
	_, err := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n- [ ] 测试任务 " + today})
	if err != nil {
		t.Fatal(err)
	}
	config, err := s.GetReminderConfig(w.ID)
	if err != nil || !config.Enabled || config.Time != "09:00" {
		t.Fatal(config, err)
	}
	original := config
	config.Time = "00:00"
	config, err = s.SaveReminderConfig(w.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveReminderConfig(w.ID, original); err == nil {
		t.Fatal("stale config overwritten")
	}
	due, err := s.DueReminders(w.ID)
	if err != nil || len(due) != 1 {
		t.Fatal(due, err)
	}
	task := due[0]
	input := ReminderAction{WorkspaceID: w.ID, TaskID: task.ID, DueAt: task.DueAt, Action: "dismiss"}
	if err = s.ActOnReminder(input); err != nil {
		t.Fatal(err)
	}
	if due, err = s.DueReminders(w.ID); err != nil || len(due) != 0 {
		t.Fatal(due, err)
	}
	reopened, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if due, err = reopened.DueReminders(w.ID); err != nil || len(due) != 0 {
		t.Fatal("restart lost dismiss", due, err)
	}
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if err = s.Export(archive, true); err != nil {
		t.Fatal(err)
	}
	imported, err := s.Import(archive, filepath.Join(t.TempDir(), "restore"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetReminderConfig(imported.ID)
	if err != nil || got.Time != "00:00" || got.Revision != config.Revision {
		t.Fatal("backup lost config", got, err)
	}
	if due, err = s.DueReminders(imported.ID); err != nil || len(due) != 0 {
		t.Fatal("backup lost dismiss", due, err)
	}
	if err = s.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err = s.UpdateTaskDetails(TaskDetails{ID: task.ID, DueAt: yesterday, Priority: "normal"}); err != nil {
		t.Fatal(err)
	}
	if due, err = s.DueReminders(w.ID); err != nil || len(due) != 1 {
		t.Fatal("changed deadline not reminded", due, err)
	}
	input.DueAt = yesterday
	input.Action = "snooze"
	input.Minutes = 15
	if err = s.ActOnReminder(input); err != nil {
		t.Fatal(err)
	}
	if due, err = s.DueReminders(w.ID); err != nil || len(due) != 0 {
		t.Fatal("snooze failed", due, err)
	}
	if err = s.SetTask(task.ID, true, false); err != nil {
		t.Fatal(err)
	}
	if due, err = s.DueReminders(w.ID); err != nil || len(due) != 0 {
		t.Fatal("completed task reminded", due, err)
	}
	if err = s.ActOnReminder(input); err == nil {
		t.Fatal("acted on completed task")
	}
}
func TestReminderValidation(t *testing.T) {
	for _, c := range []ReminderConfig{{Time: "9:00"}, {Time: "25:00"}, {Time: "09:00", LeadDays: 8}, {Time: "09:00", LeadDays: -1}} {
		if validateReminderConfig(c) == nil {
			t.Fatal("invalid config accepted", c)
		}
	}
	s, w := fixture(t)
	if _, err := s.DueReminders("another-workspace"); err == nil {
		t.Fatal("workspace leak")
	}
	if err := s.ActOnReminder(ReminderAction{WorkspaceID: w.ID, Action: "snooze", Minutes: 0}); err == nil {
		t.Fatal("bad snooze accepted")
	}
}

func TestInferDueCalendarAndWeekdays(t *testing.T) {
	base := time.Date(2026, 10, 16, 10, 0, 0, 0, time.FixedZone("local", 8*3600))
	n := Note{Created: base}
	for title, want := range map[string]string{"周一提交": "2026-10-12", "本周五提交": "2026-10-16", "下周一提交": "2026-10-19", "下下星期天检查": "2026-11-01", "2026/11/2 开会": "2026-11-02", "2026年11月2日开会": "2026-11-02", "2026年2月30日开会": "", "明天检查": "2026-10-17"} {
		if got := inferDue(title, n); got != want {
			t.Errorf("%s: %s != %s", title, got, want)
		}
	}
	if got := inferDue("下周一提交", Note{Updated: base}); got != "2026-10-19" {
		t.Fatal(got)
	}
}

func TestReminderSnoozeAcrossMidnight(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 10, 0, 0, time.FixedZone("local", 8*3600))
	task := Task{ID: "late", DueAt: "2026-10-19"}
	data := defaultReminders()
	data.Receipts[reminderKey(task)] = reminderReceipt{Until: now.Format(time.RFC3339)}
	if len(dueReminders([]Task{task}, data, now.Add(-time.Minute))) != 0 {
		t.Fatal("snooze returned early")
	}
	if len(dueReminders([]Task{task}, data, now)) != 1 {
		t.Fatal("midnight snooze deferred until morning")
	}
	if len(dueReminders([]Task{task}, data, now.AddDate(0, 0, 1))) != 0 {
		t.Fatal("old snooze bypassed next day's daily time")
	}
}

func TestRecurringReminderUsesLatestOccurrence(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.Local)
	tsk := Task{ID: "series", DueAt: "2026-10-02", Recurrence: &Recurrence{Unit: "week", Interval: 1, Anchor: "2026-10-02"}}
	data := reminderStore{Config: ReminderConfig{Enabled: true, Time: "09:00"}, Receipts: map[string]reminderReceipt{}}
	got := dueReminders([]Task{tsk}, data, now)
	if len(got) != 1 || got[0].DueAt != "2026-10-16" {
		t.Fatalf("%+v", got)
	}
	tsk.CompletedDates = []string{"2026-10-16"}
	got = dueReminders([]Task{tsk}, data, now)
	if len(got) != 1 || got[0].DueAt != "2026-10-09" {
		t.Fatalf("%+v", got)
	}
}

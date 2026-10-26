package core

import (
	"strings"
	"testing"
	"time"
)

func TestInferredNoteTaskMovesSourceOnCompletion(t *testing.T) {
	s, w := fixture(t)
	n, err := s.CreateNote("下周五提醒我VPN延期", "Inbox")
	if err != nil {
		t.Fatal(err)
	}
	task := tasksFor(n)[0]
	if !task.Inferred || task.Candidate || strings.Contains(n.Content, "- [ ]") {
		t.Fatal("invalid note task")
	}
	if err = s.UpdateTaskDetails(TaskDetails{ID: task.ID, DueAt: "2026-10-24", Priority: "high"}); err != nil {
		t.Fatal(err)
	}
	if err = s.SetTask(task.ID, true, false); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Notes) != 1 || snap.Notes[0].ID != n.ID || !snap.Notes[0].TaskArchived || len(snap.Tasks) != 0 {
		t.Fatalf("archive move failed: %+v", snap)
	}
	if archiveBody(snap.Notes[0].Content) != archiveBody(n.Content) {
		t.Fatal("archive move changed body")
	}
	records, err := s.GetTaskArchive(w.ID)
	if err != nil || len(records) != 1 || records[0].Priority != "high" || records[0].DueAt != "2026-10-24" {
		t.Fatal("completion details lost", err)
	}
}
func TestInferredNoteTaskSafety(t *testing.T) {
	for _, body := range []string{"如果下周五提醒我VPN延期", "不用提醒我VPN延期", "不要提醒我VPN延期", "提醒我VPN延期？", "```\n提醒我VPN延期\n```", "普通笔记"} {
		n := parseNote("test.md", []byte(body), time.Now())
		for _, task := range tasksFor(n) {
			if task.Inferred {
				t.Fatalf("unexpected inference: %s", body)
			}
		}
	}
	n := parseNote("test.md", []byte("# VPN延期\n\n下周五提醒我VPN延期"), time.Date(2026, 10, 16, 10, 0, 0, 0, time.Local))
	tasks := tasksFor(n)
	if len(tasks) != 1 || tasks[0].DueAt != "2026-10-23" || tasks[0].Title != "VPN延期" {
		t.Fatalf("%+v", tasks)
	}
}

func TestPriorReminderCheckboxIsNotDuplicated(t *testing.T) {
	n := parseNote("test.md", []byte("# 下周五提醒我VPN延期\n\n- [x] 下周五提醒我VPN延期"), time.Now())
	tasks := tasksFor(n)
	if len(tasks) != 1 || !tasks[0].Inferred || !tasks[0].Done {
		t.Fatalf("%+v", tasks)
	}
}

func TestDateTitleNoteTask(t *testing.T) {
	base := time.Date(2026, 10, 16, 10, 0, 0, 0, time.Local)
	for _, title := range []string{"10月21日我要出差", "10月21号参加会议", "2026年10月21日办理续费", "2026-10-21提交报告"} {
		n := parseNote("test.md", []byte("# "+title), base)
		n.Created = base
		tasks := tasksFor(n)
		if len(tasks) != 1 || !tasks[0].Inferred || tasks[0].DueAt != "2026-10-21" {
			t.Fatalf("%s: %+v", title, tasks)
		}
	}
	n := parseNote("test.md", []byte("# 普通记录\n\n10月21日我要出差"), base)
	if tasks := tasksFor(n); len(tasks) != 1 || tasks[0].DueAt != "2026-10-21" {
		t.Fatalf("body intent: %+v", tasks)
	}
}

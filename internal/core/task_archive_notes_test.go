package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskArchiveNotesVisibleEditableAndExcluded(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("今天提醒我检查数据库", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	n, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n独立的正文快照"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SetTask(tasksFor(n)[0].ID, true, false); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	var archived Note
	for _, note := range snap.Notes {
		if note.TaskArchived {
			archived = note
		}
	}
	prefix := taskArchiveDirectory + "/" + time.Now().Format("2006/01") + "/"
	if !strings.HasPrefix(archived.Path, prefix) || !strings.Contains(archived.Content, "独立的正文快照") {
		t.Fatalf("missing visible archive: %+v", archived)
	}
	if len(snap.Tasks) != 0 {
		t.Fatalf("archive generated tasks: %+v", snap.Tasks)
	}
	if _, err := os.Stat(filepath.Join(w.Path, filepath.FromSlash(n.Path))); !os.IsNotExist(err) {
		t.Fatal("source note still exists", err)
	}
	if archived.ID != n.ID {
		t.Fatal("archive changed note identity")
	}
	// Editing even unchecked tasks and recurrence requests in an archive is safe.
	archived, e = s.SaveNote(SaveInput{Path: archived.Path, Hash: archived.Hash, Content: archived.Content + "\n- [ ] 每天提醒我充值\n明天提醒我开会\n自定义归档批注"})
	if e != nil {
		t.Fatal(e)
	}
	if len(tasksFor(archived)) != 0 {
		t.Fatal("archive edit scheduled a task")
	}
	if r, e := s.AutoArchive(w.ID, archived.ID, archived.Hash); e != nil || r != nil {
		t.Fatalf("archive reclassified: %+v %v", r, e)
	}
	moved, e := s.MoveNote(archived.ID, archived.Hash, "其他/归档记录.md")
	if e != nil {
		t.Fatal(e)
	}
	if len(tasksFor(moved.Note)) != 0 {
		t.Fatal("moved archive scheduled a task")
	}
	if r, e := s.AutoArchive(w.ID, moved.Note.ID, moved.Note.Hash); e != nil || r != nil {
		t.Fatalf("moved archive reclassified: %+v %v", r, e)
	}
	for i := 0; i < 2; i++ {
		snap, e = s.Snapshot()
		if e != nil {
			t.Fatal(e)
		}
		if len(snap.Tasks) != 0 || len(snap.Notes) != 1 {
			t.Fatalf("duplicate or scheduled archive: %+v", snap)
		}
	}
	if workspaceFile(t, w.Path, moved.Note.Path) != moved.Note.Content {
		t.Fatal("archive edit overwritten")
	}
	plain, e := s.CreateNote("每周提醒我开会", taskArchiveDirectory+"/自定义")
	if e != nil {
		t.Fatal(e)
	}
	plain, e = s.SaveNote(SaveInput{Path: plain.Path, Hash: plain.Hash, Content: plain.Content + "\n- [ ] 明天检查"})
	if e != nil || len(tasksFor(plain)) != 0 {
		t.Fatalf("unmarked archive-folder note scheduled tasks: %v", e)
	}
}

func TestTaskArchiveLegacyAndRecurringSnapshots(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("每周提醒我充值话费", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	task := tasksFor(n)[0]
	for _, date := range []string{task.DueAt, occurrence(task.Recurrence, 1)} {
		if e = s.SetTaskOccurrence(w.ID, task.ID, date, true); e != nil {
			t.Fatal(e)
		}
	}
	records, e := s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 2 {
		t.Fatalf("%+v %v", records, e)
	}
	snap, e := s.Snapshot()
	if e != nil || len(snap.Notes) != 3 {
		t.Fatalf("recurring completion files missing: %d %v", len(snap.Notes), e)
	}
	// Test upgrading a v0.9.9 ledger with a completion but no source body.
	legacy := TaskArchiveRecord{ID: "legacy", TaskID: "gone", NoteID: "deleted", Title: "明天提醒我检查", NotePath: "Inbox/已删除.md", CompletedAt: "2025-03-05T12:00:00+08:00", Imported: true}
	if e = writeJSON(filepath.Join(s.data(w), "tasks-metadata.json"), map[string]Task{"gone": {History: []TaskArchiveRecord{legacy}}}); e != nil {
		t.Fatal(e)
	}
	snap, e = s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(w.Path, taskArchiveDirectory, "2025", "03", archiveNoteFilename(legacy))
	b, e := os.ReadFile(path)
	if e != nil || !strings.Contains(string(b), "来源笔记已不存在") {
		t.Fatalf("legacy snapshot not created: %s %v", b, e)
	}
	// Snapshot must include the new folder in the same response.
	found := false
	for _, dir := range snap.Directories {
		if dir == taskArchiveDirectory+"/2025/03" {
			found = true
		}
	}
	if !found {
		t.Fatal("new archive directory not visible on first refresh")
	}
}

func TestCompletedNoteWaitsForAllTasksAndPreservesConflictingFiles(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("两个事项", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	n, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n- [ ] 第一项\n- [ ] 第二项"})
	if e != nil {
		t.Fatal(e)
	}
	tasks := tasksFor(n)
	ref, e := s.CreateNote("引用", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	ref, e = s.SaveNote(SaveInput{Path: ref.Path, Hash: ref.Hash, Content: ref.Content + "\n[原笔记](两个事项.md)"})
	if e != nil {
		t.Fatal(e)
	}
	dir := taskArchiveDirectory + "/" + time.Now().Format("2006/01")
	conflict, e := s.CreateNote("两个事项", dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SetTask(tasks[0].ID, true, false); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(w.Path, filepath.FromSlash(n.Path))); e != nil {
		t.Fatal("partly completed note moved", e)
	}
	if e = s.SetTask(tasks[1].ID, true, false); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(w.Path, filepath.FromSlash(n.Path))); !os.IsNotExist(e) {
		t.Fatal("source still exists", e)
	}
	if workspaceFile(t, w.Path, conflict.Path) != conflict.Content {
		t.Fatal("archive collision overwrote another note")
	}
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	var moved Note
	for _, note := range snap.Notes {
		if note.ID == n.ID {
			moved = note
		}
		if note.ID == ref.ID {
			ref = note
		}
	}
	if moved.ID == "" || moved.Path == conflict.Path || !moved.TaskArchived {
		t.Fatal("original note not archived safely")
	}
	links := markdownLinks(ref.Content)
	found := false
	for _, link := range links {
		if resolved := resolveNoteLink(ref, link, snap.Notes); resolved != nil && resolved.ID == n.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("reference to original note not updated")
	}
}

func TestFinishedRecurringNoteMovesOnlyAfterAllOccurrences(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("每天提醒我检查设备", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot()
	if e != nil || len(snap.Tasks) != 1 {
		t.Fatal(e, snap.Tasks)
	}
	task := snap.Tasks[0]
	anchor := task.Recurrence.Anchor
	end := occurrence(task.Recurrence, 1)
	if e = s.ResolveTaskIntent(TaskIntentDecision{WorkspaceID: w.ID, TaskID: task.ID, SourceRevision: task.SourceRevision, Accept: true, Until: end}); e != nil {
		t.Fatal(e)
	}
	// Completing the last date first must not archive outstanding earlier dates.
	if e = s.SetTaskOccurrence(w.ID, task.ID, end, true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(w.Path, filepath.FromSlash(n.Path))); e != nil {
		t.Fatal("unfinished series moved", e)
	}
	if e = s.SetTaskOccurrence(w.ID, task.ID, anchor, true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(w.Path, filepath.FromSlash(n.Path))); !os.IsNotExist(e) {
		t.Fatal("finished source still exists", e)
	}
	snap, e = s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	var moved Note
	for _, note := range snap.Notes {
		if note.ID == n.ID {
			moved = note
		}
	}
	if !moved.TaskArchived || !strings.HasPrefix(moved.Path, taskArchiveDirectory+"/"+time.Now().Format("2006/01")+"/") {
		t.Fatal("missing original archive", moved.Path)
	}
	if len(snap.Tasks) != 0 {
		t.Fatal("archived series generated tasks", snap.Tasks)
	}
	if !strings.Contains(moved.Content, "每天提醒我检查设备") {
		t.Fatal("source content lost")
	}
	// Refreshes preserve the move and the per-occurrence completion records.
	records, e := s.GetTaskArchive(w.ID)
	if e != nil || len(records) != 2 {
		t.Fatal("history lost", e, len(records))
	}
}

func TestUnlimitedRecurringNoteKeepsSource(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("每周提醒我检查设备", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	task := snap.Tasks[0]
	if e = s.ResolveTaskIntent(TaskIntentDecision{WorkspaceID: w.ID, TaskID: task.ID, SourceRevision: task.SourceRevision, Accept: true}); e != nil {
		t.Fatal(e)
	}
	if e = s.SetTaskOccurrence(w.ID, task.ID, task.Recurrence.Anchor, true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(w.Path, filepath.FromSlash(n.Path))); e != nil {
		t.Fatal("unlimited source moved", e)
	}
}

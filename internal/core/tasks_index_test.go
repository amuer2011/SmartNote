package core

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskDetailsSurviveCompletionAndBackup(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "任务元数据")
	n, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "- [ ] 完成迁移 2026-10-20\n明天需要整理资料。\n"})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	explicit, candidate := snap.Tasks[0], snap.Tasks[1]
	if explicit.DueAt != "2026-10-20" || candidate.DueAt != n.Created.AddDate(0, 0, 1).Format("2006-01-02") {
		t.Fatalf("date inference %+v", snap.Tasks)
	}
	if err = s.UpdateTaskDetails(TaskDetails{ID: explicit.ID, DueAt: "2026-10-18", Priority: "high"}); err != nil {
		t.Fatal(err)
	}
	if err = s.SetTask(explicit.ID, true, false); err != nil {
		t.Fatal(err)
	}
	if err = s.SetTask(candidate.ID, false, true); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateTaskDetails(TaskDetails{ID: candidate.ID, DueAt: "", Priority: "low"}); err != nil {
		t.Fatal(err)
	}
	if err = s.SetTask(candidate.ID, true, false); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		snap, e := s.Snapshot()
		if e != nil {
			t.Fatal(e)
		}
		archived := false
		for _, note := range snap.Notes {
			if note.ID == n.ID {
				archived = note.TaskArchived
			}
		}
		if len(snap.Tasks) != 0 || !archived {
			t.Fatal("completed note not moved")
		}
		active, _ := s.active()
		metadata := map[string]Task{}
		if e := readJSON(filepath.Join(s.data(active), "tasks-metadata.json"), &metadata); e != nil {
			t.Fatal(e)
		}
		a, b := metadata[explicit.ID], metadata[candidate.ID]
		if !a.Done || a.Priority != "high" || a.DueAt != "2026-10-18" || !b.Done || b.Candidate || b.Priority != "low" || b.DueAt != "" {
			t.Fatalf("lost task details %+v", snap.Tasks)
		}
	}
	check()
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if err = s.Export(archive, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Import(archive, filepath.Join(t.TempDir(), "restored")); err != nil {
		t.Fatal(err)
	}
	check()
	if err = s.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateTaskDetails(TaskDetails{ID: explicit.ID, DueAt: "invalid", Priority: "high"}); err == nil {
		t.Fatal("invalid date accepted")
	}
}
func TestIncrementalIndexSkipsUnchangedAndRemovesDeleted(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "中文检索")
	if _, err := s.Search("中文"); err != nil {
		t.Fatal(err)
	}
	var before, after int64
	if err := s.db.QueryRow("SELECT total_changes()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search("检索"); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT total_changes()").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("unchanged search rewrote index: %d -> %d", before, after)
	}
	n, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "新的关键词"})
	if err != nil {
		t.Fatal(err)
	}
	results, err := s.Search("新的关键词")
	if err != nil || len(results) != 1 {
		t.Fatalf("incremental update: %v", err)
	}
	if err = s.DeleteNote(n.ID, n.Hash); err != nil {
		t.Fatal(err)
	}
	results, err = s.Search("新的关键词")
	if err != nil || len(results) != 0 {
		t.Fatal("deleted note still searchable")
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM notes_fts WHERE workspace=?", w.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("deleted FTS row retained")
	}
}
func TestImportReferenceStyleSpacedAndWikiImages(t *testing.T) {
	s, _ := fixture(t)
	root := t.TempDir()
	archive := filepath.Join(root, "input.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	items := map[string]string{"notes/readme.md": "# 导入\n![one](<../assets/a b(1).png> \"title\")\n![two][img]\n[img]: ../assets/a%20b(1).png\n![[../assets/a b(1).png|图片别名]]\n`![示例](missing.png)`\n", "assets/a b(1).png": "same-image", "unused.txt": "unreferenced"}
	for name, content := range items {
		entry, e := writer.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = entry.Write([]byte(content)); e != nil {
			t.Fatal(e)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, err = s.Import(archive, filepath.Join(root, "restored")); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Attachments) != 2 || strings.Count(snapshot.Notes[0].Content, "smartnote-asset://") != 3 || !strings.Contains(snapshot.Notes[0].Content, "`![示例](missing.png)`") {
		t.Fatalf("incorrect migration: %+v", snapshot)
	}
}

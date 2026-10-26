package core

import (
	"archive/zip"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Service, Workspace) {
	t.Helper()
	root := t.TempDir()
	s, err := New(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	w, err := s.AddWorkspace("测试", filepath.Join(root, "notes"))
	if err != nil {
		t.Fatal(err)
	}
	return s, w
}
func mustNote(t *testing.T, s *Service, title string) Note {
	t.Helper()
	n, err := s.CreateNote(title, "Inbox")
	if err != nil {
		t.Fatal(err)
	}
	return n
}
func TestConflictAndDeletePreserveData(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "数据库排查")
	path := filepath.Join(w.Path, filepath.FromSlash(n.Path))
	external := n.Content + "外部修改\n"
	if err := os.WriteFile(path, []byte(external), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: "覆盖"}); err == nil || !strings.Contains(err.Error(), "CONFLICT") {
		t.Fatalf("expected conflict, got %v", err)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != external {
		t.Fatal("external content was lost")
	}
	if err := s.DeleteNote(n.ID, n.Hash); err == nil {
		t.Fatal("stale deletion should fail")
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteNote(n.ID, snap.Notes[0].Hash); err != nil {
		t.Fatal(err)
	}
	preserved := false
	filepath.WalkDir(filepath.Join(s.data(w), "trash"), func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			b, _ := os.ReadFile(p)
			preserved = string(b) == external
		}
		return e
	})
	if !preserved {
		t.Fatal("deleted note was not recoverable")
	}
}
func TestTasksRoundTripAndNoteIntentPersistence(t *testing.T) {
	s, _ := fixture(t)
	n := mustNote(t, s, "任务")
	text := n.Content + "- [ ] 检查数据库\n明天需要整理资料。\n```md\n- [ ] 示例任务\n```\n"
	n, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: text})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Tasks) != 2 {
		t.Fatalf("got tasks: %+v", snapshot.Tasks)
	}
	explicit, candidate := snapshot.Tasks[0], snapshot.Tasks[1]
	if !candidate.Candidate || !candidate.Inferred || !candidate.NeedsConfirmation {
		t.Fatal("recognized note intent must wait for confirmation")
	}
	if err = s.SetTask(explicit.ID, true, false); err != nil {
		t.Fatal(err)
	}
	if err = s.SetTask(candidate.ID, false, true); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	sourceContent := ""
	for _, note := range snapshot.Notes {
		if note.ID == n.ID {
			sourceContent = note.Content
		}
	}
	if !strings.Contains(sourceContent, "- [x] 检查数据库 <!-- task:"+explicit.ID+" -->") {
		t.Fatalf("checkbox not updated: %s", sourceContent)
	}
	if !snapshot.Tasks[0].Done || snapshot.Tasks[1].Candidate {
		t.Fatal("task state not persisted")
	}
	if err = s.SetTask(explicit.ID, false, false); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Tasks[0].Done || snapshot.Tasks[0].ID != explicit.ID {
		t.Fatal("task ID or state changed")
	}
}
func TestNativeAndStandardBackupRoundTrip(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "带附件笔记")
	a, err := s.ImportAttachment("测试.txt", base64.StdEncoding.EncodeToString([]byte("附件原文")), false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "[文件](smartnote-asset://" + a.ID + ")\n明天需要整理资料。\n"})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetTask(snap.Tasks[0].ID, true, true); err != nil {
		t.Fatal(err)
	}
	for _, native := range []bool{true, false} {
		if err = s.OpenWorkspace(w.ID); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		archive := filepath.Join(root, "backup.zip")
		if err = s.Export(archive, native); err != nil {
			t.Fatal(err)
		}
		restored, err := s.Import(archive, filepath.Join(root, "restored"))
		if err != nil {
			t.Fatal(err)
		}
		if restored.ID == w.ID {
			t.Fatal("restore must have independent workspace ID")
		}
		snapshot, err := s.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Notes) != 1 || len(snapshot.Attachments) != 1 {
			t.Fatalf("incomplete restore: %+v", snapshot)
		}
		sourceCount, archiveCount := 0, 0
		for _, note := range snapshot.Notes {
			if note.TaskArchived {
				archiveCount++
			}
			if note.ID == n.ID {
				sourceCount++
			}
			if !strings.Contains(note.Content, "smartnote-asset://"+snapshot.Attachments[0].ID) {
				t.Fatal("attachment reference not restored")
			}
		}
		if sourceCount != 1 || archiveCount != 1 {
			t.Fatal("source or archive missing from backup")
		}
		data, err := s.AttachmentData(snapshot.Attachments[0].ID)
		if err != nil || !strings.Contains(data, base64.StdEncoding.EncodeToString([]byte("附件原文"))) {
			t.Fatalf("attachment mismatch %v", err)
		}
		if native {
			records, e := s.GetTaskArchive(restored.ID)
			if e != nil || len(records) != 1 || records[0].NoteID != n.ID {
				t.Fatal("native backup lost completion metadata", e)
			}
		}
		if len(snapshot.Tasks) != 0 {
			t.Fatal("archived backup scheduled task")
		}
	}
}
func TestUnsafeImportAndPaths(t *testing.T) {
	s, w := fixture(t)
	for _, path := range []string{"../escape.md", "C:/escape.md", "foo/../../escape.md", "foo/CON.md", "foo/test.md.", "foo/file:stream.md"} {
		if _, err := safePath(w.Path, path); err == nil {
			t.Errorf("accepted unsafe path %q", path)
		}
	}
	root := t.TempDir()
	archive := filepath.Join(root, "evil.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	entry, err := z.Create("../escape.md")
	if err != nil {
		t.Fatal(err)
	}
	entry.Write([]byte("escape"))
	z.Close()
	f.Close()
	if _, err = s.Import(archive, filepath.Join(root, "destination")); err == nil {
		t.Fatal("zip slip was accepted")
	}
	if _, err = os.Stat(filepath.Join(root, "escape.md")); !os.IsNotExist(err) {
		t.Fatal("zip escaped staging directory")
	}
}
func TestChineseSearchWorkspaceIsolationAndDedup(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "国产化数据库改造")
	_, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "灰度认证失败排查，显式解密密码。"})
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"密码", "数据库", "认证失败", "灰度 解密"} {
		r, err := s.Search(query)
		if err != nil || len(r) != 1 {
			t.Fatalf("query %q: %v %+v", query, err, r)
		}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("same"))
	a, err := s.ImportAttachment("a.txt", encoded, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.ImportAttachment("b.txt", encoded, false)
	if err != nil || a.ID != b.ID {
		t.Fatal("attachments not deduplicated")
	}
	_, err = s.AddWorkspace("隔离", filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Search("数据库")
	if err != nil || len(r) != 0 {
		t.Fatal("search leaked across workspaces")
	}
	if err = s.RemoveWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(w.Path, n.Path)); err != nil {
		t.Fatal("removing registration deleted user data")
	}
}

func TestEmptyWorkspaceBackup(t *testing.T) {
	s, _ := fixture(t)
	root := t.TempDir()
	archive := filepath.Join(root, "empty.zip")
	if err := s.Export(archive, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(archive, filepath.Join(root, "restored")); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot()
	if err != nil || len(snapshot.Notes) != 0 {
		t.Fatalf("empty workspace restore: %v", err)
	}
}

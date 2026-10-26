package core

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotNote returns the current snapshot state of a note by stable ID.
func snapshotNote(t *testing.T, s *Service, id string) Note {
	t.Helper()
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range snap.Notes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("note %s missing from snapshot: %+v", id, snap.Notes)
	return Note{}
}

// workspaceFile reads a workspace-relative note file straight from disk.
func workspaceFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSaveNoteTitleRenameKeepsIdentityAndUpdatesLinks(t *testing.T) {
	s, w := fixture(t)
	source := mustNote(t, s, "旧标题")
	other := mustNote(t, s, "相关")

	// The related note already points at the source through both link styles.
	linked := other.Content + "[来源](旧标题.md)\n[[旧标题|原笔记]]\n"
	if _, err := s.SaveNote(SaveInput{Path: other.Path, Hash: other.Hash, Content: linked}); err != nil {
		t.Fatal(err)
	}

	renamed := strings.Replace(source.Content, `title: "旧标题"`, `title: "新标题"`, 1)
	if renamed == source.Content {
		t.Fatalf("fixture front matter has no title: %q", source.Content)
	}
	renamed = strings.Replace(renamed, "# 旧标题", "# 新标题", 1) + "新增正文内容\n"

	got, err := s.SaveNote(SaveInput{Path: source.Path, Hash: source.Hash, Content: renamed})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != source.ID {
		t.Fatalf("rename changed note ID: %s -> %s", source.ID, got.ID)
	}
	if got.Path != "Inbox/新标题.md" {
		t.Fatalf("same-directory rename not applied, path = %q", got.Path)
	}
	if !strings.Contains(got.Content, "title: \"新标题\"") || !strings.Contains(got.Content, "id: "+source.ID) {
		t.Fatalf("front matter not preserved: %s", got.Content)
	}
	if !strings.Contains(got.Content, "新增正文内容") {
		t.Fatalf("edited content not saved: %s", got.Content)
	}

	if _, err = os.Stat(filepath.Join(w.Path, filepath.FromSlash(source.Path))); !os.IsNotExist(err) {
		t.Fatal("source file still exists after rename")
	}
	if onDisk := workspaceFile(t, w.Path, got.Path); onDisk != got.Content {
		t.Fatalf("renamed file content mismatch:\n%s", onDisk)
	}

	// The stable ID must be discoverable at the new path.
	if current := snapshotNote(t, s, source.ID); current.Path != got.Path || current.ID != source.ID {
		t.Fatalf("snapshot identity drifted: %+v", current)
	}

	// Reverse links now target the new filename.
	backlinks := snapshotNote(t, s, other.ID).Content
	if strings.Contains(backlinks, "旧标题") {
		t.Fatalf("stale reverse link kept old title: %s", backlinks)
	}
	if !strings.Contains(backlinks, "[[Inbox/新标题|原笔记]]") {
		t.Fatalf("wiki backlink not rewritten: %s", backlinks)
	}
	decoded := map[string]bool{}
	for _, ref := range markdownLinks(backlinks) {
		if ref.Wiki {
			continue
		}
		if u, err := url.Parse(ref.Target); err == nil {
			decoded[u.Path] = true
		}
	}
	if !decoded["新标题.md"] {
		t.Fatalf("markdown backlink not rewritten: %s", backlinks)
	}
}

func TestSaveNoteTitleRenameRejectsExistingTarget(t *testing.T) {
	s, w := fixture(t)
	first := mustNote(t, s, "甲")
	second := mustNote(t, s, "乙")

	incoming := strings.Replace(first.Content, `title: "甲"`, `title: "乙"`, 1) + "不应写入\n"
	if _, err := s.SaveNote(SaveInput{Path: first.Path, Hash: first.Hash, Content: incoming}); err == nil || !strings.Contains(err.Error(), "目标已存在") {
		t.Fatalf("expected duplicate-name failure, got %v", err)
	}

	// Neither the source nor the existing destination may be touched.
	if onDisk := workspaceFile(t, w.Path, first.Path); onDisk != first.Content {
		t.Fatalf("source content was overwritten: %s", onDisk)
	}
	if onDisk := workspaceFile(t, w.Path, second.Path); onDisk != second.Content {
		t.Fatalf("destination content was overwritten: %s", onDisk)
	}
	afterFirst := snapshotNote(t, s, first.ID)
	if afterFirst.Path != first.Path || strings.Contains(afterFirst.Content, "不应写入") {
		t.Fatalf("failed rename leaked changes: %+v", afterFirst)
	}
	if afterSecond := snapshotNote(t, s, second.ID); afterSecond.Path != second.Path || afterSecond.Content != second.Content {
		t.Fatalf("destination note changed: %+v", afterSecond)
	}
}

func TestSaveNoteBodyEditOrTitleRemovalDoesNotRename(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "保持路径")

	t.Run("body edit", func(t *testing.T) {
		got, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "普通正文修改\n"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Path != n.Path || got.ID != n.ID {
			t.Fatalf("body edit renamed note: %+v", got)
		}
		if onDisk := workspaceFile(t, w.Path, n.Path); !strings.Contains(onDisk, "普通正文修改") {
			t.Fatalf("body edit not persisted: %s", onDisk)
		}
		snap, err := s.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Notes) != 1 || snap.Notes[0].Path != n.Path {
			t.Fatalf("body edit created extra notes: %+v", snap.Notes)
		}
	})

	t.Run("title removed", func(t *testing.T) {
		current := snapshotNote(t, s, n.ID)
		without := strings.Replace(current.Content, "title: \"保持路径\"\n", "", 1)
		if without == current.Content {
			t.Fatalf("title line not found in %q", current.Content)
		}
		got, err := s.SaveNote(SaveInput{Path: current.Path, Hash: current.Hash, Content: without})
		if err != nil {
			t.Fatal(err)
		}
		if got.Path != n.Path || got.ID != n.ID {
			t.Fatalf("title removal renamed note: %+v", got)
		}
		if onDisk := workspaceFile(t, w.Path, n.Path); onDisk != without {
			t.Fatalf("title removal not persisted: %s", onDisk)
		}
		snap, err := s.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Notes) != 1 || snap.Notes[0].Path != n.Path {
			t.Fatalf("title removal created extra notes: %+v", snap.Notes)
		}
	})
}

func TestSaveNoteAddsFrontMatterTitleToSyntheticNoteKeepingID(t *testing.T) {
	s, w := fixture(t)
	external := "# 外部笔记\n\n- [ ] 外部任务\n"
	if err := os.WriteFile(filepath.Join(w.Path, "外部.md"), []byte(external), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Notes) != 1 || !snap.Notes[0].SyntheticID {
		t.Fatalf("expected one synthetic note, got %+v", snap.Notes)
	}
	originalID := snap.Notes[0].ID
	originalHash := snap.Notes[0].Hash

	incoming := "---\ntitle: 新增标题\n---\n\n# 新增标题\n\n正文\n"
	got, err := s.SaveNote(SaveInput{Path: snap.Notes[0].Path, Hash: originalHash, Content: incoming})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != originalID {
		t.Fatalf("synthetic ID not preserved: %s -> %s", originalID, got.ID)
	}
	if got.Path != "新增标题.md" {
		t.Fatalf("rename not applied, path = %q", got.Path)
	}
	if !strings.Contains(got.Content, "id: "+originalID) {
		t.Fatalf("stable ID not written into front matter: %s", got.Content)
	}
	if _, err = os.Stat(filepath.Join(w.Path, "外部.md")); !os.IsNotExist(err) {
		t.Fatal("original external file still exists")
	}
	if onDisk := workspaceFile(t, w.Path, "新增标题.md"); onDisk != got.Content {
		t.Fatalf("renamed content mismatch: %s", onDisk)
	}

	// The injected ID keeps identity stable across further title renames.
	stable := snapshotNote(t, s, originalID)
	again := strings.Replace(stable.Content, "title: 新增标题", "title: 再次改名", 1)
	again = strings.Replace(again, "# 新增标题", "# 再次改名", 1)
	second, err := s.SaveNote(SaveInput{Path: stable.Path, Hash: stable.Hash, Content: again})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != originalID || second.Path != "再次改名.md" {
		t.Fatalf("ID/path unstable on second rename: %+v", second)
	}
	if current := snapshotNote(t, s, originalID); current.Path != "再次改名.md" {
		t.Fatalf("snapshot lost stable ID: %+v", current)
	}
}

func TestSaveNoteExternalConflictKeepsDataOnTitleRename(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "冲突旧名")
	path := filepath.Join(w.Path, filepath.FromSlash(n.Path))

	external := strings.Replace(n.Content, "# 冲突旧名", "# 外部改过", 1) + "外部追加数据\n"
	if err := os.WriteFile(path, []byte(external), 0600); err != nil {
		t.Fatal(err)
	}

	incoming := strings.Replace(n.Content, `title: "冲突旧名"`, `title: "冲突新名"`, 1) + "本地编辑\n"
	if _, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: incoming}); err == nil || !strings.Contains(err.Error(), "CONFLICT") {
		t.Fatalf("expected conflict on stale hash, got %v", err)
	}

	// The external revision must survive untouched and no rename may happen.
	if onDisk := workspaceFile(t, w.Path, n.Path); onDisk != external {
		t.Fatalf("external content was lost: %s", onDisk)
	}
	if _, err := os.Stat(filepath.Join(w.Path, "Inbox", "冲突新名.md")); !os.IsNotExist(err) {
		t.Fatal("conflicting save still created the renamed file")
	}
	conflicted := snapshotNote(t, s, n.ID)
	if conflicted.Path != n.Path || !strings.Contains(conflicted.Content, "外部追加数据") {
		t.Fatalf("external revision not visible after reload: %+v", conflicted)
	}

	// Reload-then-merge keeps the external data while applying the rename.
	merged := strings.Replace(conflicted.Content, `title: "冲突旧名"`, `title: "冲突新名"`, 1)
	merged = strings.Replace(merged, "# 外部改过", "# 冲突新名", 1) + "本地编辑\n"
	saved, err := s.SaveNote(SaveInput{Path: conflicted.Path, Hash: conflicted.Hash, Content: merged})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != n.ID || saved.Path != "Inbox/冲突新名.md" {
		t.Fatalf("merged save identity mismatch: %+v", saved)
	}
	if onDisk := workspaceFile(t, w.Path, saved.Path); !strings.Contains(onDisk, "外部追加数据") || !strings.Contains(onDisk, "本地编辑") {
		t.Fatalf("merged save lost data: %s", onDisk)
	}
	if _, err := os.Stat(filepath.Join(w.Path, filepath.FromSlash(n.Path))); !os.IsNotExist(err) {
		t.Fatal("old file still exists after merged rename")
	}
}

func TestSaveNoteNewTitleMatchingHeadingStillRenames(t *testing.T) {
	s, w := fixture(t)
	if err := os.WriteFile(filepath.Join(w.Path, "文件名.md"), []byte("# 正文标题\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	n := snap.Notes[0]
	saved, err := s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: "---\ntitle: 正文标题\n---\n# 正文标题\n"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Path != "正文标题.md" || saved.ID != n.ID {
		t.Fatalf("title not synchronized: %+v", saved)
	}
}

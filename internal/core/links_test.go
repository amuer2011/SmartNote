package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownLinksRecognizeDestinationsAndSkipCode(t *testing.T) {
	text := "---\ntitle: '[not a link](fake.md)'\n---\n[one](../one.md#标题)\n![photo](<images/a b(1).png> \"caption\")\n[log][ref]\n[ref]: files/log\\(1\\).txt \"log\"\n![[images/pic.png|截图]]\n[[知识/索引#段落|链接]]\n`[code](ignored.md)`\n```md\n[example](ignored2.md)\n```\n    [indented](ignored3.md)\n"
	refs := markdownLinks(text)
	expected := []string{"../one.md#标题", "images/a b(1).png", "files/log(1).txt", "images/pic.png", "知识/索引#段落"}
	if len(refs) != len(expected) {
		t.Fatalf("refs: %+v", refs)
	}
	for i, r := range refs {
		if r.Target != expected[i] {
			t.Errorf("got %q expected %q", r.Target, expected[i])
		}
	}
}
func TestMoveNoteUpdatesRelativeReferencesAndPreservesCode(t *testing.T) {
	s, w := fixture(t)
	source := mustNote(t, s, "来源")
	other := mustNote(t, s, "相关")
	source, err := s.SaveNote(SaveInput{Path: source.Path, Hash: source.Hash, Content: source.Content + "[相关](相关.md)\n[附件](../assets/log.txt)\n- [ ] 执行任务\n"})
	if err != nil {
		t.Fatal(err)
	}
	other, err = s.SaveNote(SaveInput{Path: other.Path, Hash: other.Hash, Content: other.Content + "[来源](来源.md#小节)\n[[来源|原笔记]]\n[引用][src]\n[src]: 来源.md\n`[示例](来源.md)`\n"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.MoveNote(source.ID, source.Hash, "归档/子目录/新名称.md")
	if err != nil {
		t.Fatal(err)
	}
	if result.Note.ID != source.ID || result.UpdatedLinks != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Note.Content, "../../Inbox/") || !strings.Contains(result.Note.Content, "../../assets/log.txt") {
		t.Fatalf("outgoing links: %s", result.Note.Content)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range snap.Notes {
		if n.ID == other.ID {
			if !strings.Contains(n.Content, "../%E5%BD%92%E6%A1%A3/") || !strings.Contains(n.Content, "[[归档/子目录/新名称|原笔记]]") || !strings.Contains(n.Content, "`[示例](来源.md)`") {
				t.Fatalf("incoming links: %s", n.Content)
			}
		}
	}
	if _, err = os.Stat(filepath.Join(w.Path, filepath.FromSlash(source.Path))); !os.IsNotExist(err) {
		t.Fatal("original file still exists")
	}
	if _, err = os.Stat(filepath.Join(result.RecoveryPath, "journal.json")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.MoveNote(result.Note.ID, result.Note.Hash, other.Path); err == nil {
		t.Fatal("must reject existing destination")
	}
	if _, err = s.MoveNote(result.Note.ID, "stale", "another.md"); err == nil {
		t.Fatal("must reject stale input")
	}
}
func TestMovingExternalNotePreservesIdentity(t *testing.T) {
	s, w := fixture(t)
	text := "# 外部笔记\n\n- [ ] 稳定任务\n"
	if err := os.WriteFile(filepath.Join(w.Path, "external.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	id := snap.Notes[0].ID
	taskID := snap.Tasks[0].ID
	result, err := s.MoveNote(id, snap.Notes[0].Hash, "归档/external.md")
	if err != nil {
		t.Fatal(err)
	}
	snap, err = s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if result.Note.ID != id || snap.Tasks[0].ID != taskID {
		t.Fatal("move changed stable identities")
	}
}
func TestInterruptedMoveRecoveryKeepsOriginals(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "恢复")
	root := filepath.Join(s.data(w), "operations", "interrupted")
	after := n.Content + "changed"
	j := moveJournal{State: "prepared", WorkspaceID: w.ID, Source: n.Path, Target: "Moved.md", TargetHash: digest([]byte(after)), TargetCreated: true, Changes: []moveChange{{Path: n.Path, BeforeHash: n.Hash, AfterHash: digest([]byte(after)), Backup: "0.md"}}}
	if err := atomicWrite(filepath.Join(root, "0.md"), []byte(n.Content)); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(root, "journal.json"), j); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(filepath.Join(w.Path, "Moved.md"), []byte(after)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(w.Path, n.Path)); err != nil {
		t.Fatal(err)
	}
	if err := s.recoverMoves(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(w.Path, n.Path))
	if err != nil || string(b) != n.Content {
		t.Fatal("original not recovered")
	}
	if _, err = os.Stat(filepath.Join(w.Path, "Moved.md")); !os.IsNotExist(err) {
		t.Fatal("uncommitted target not removed")
	}
	// A concurrent destination is never deleted if exclusive creation never succeeded.
	j.TargetCreated = false
	j.State = "prepared"
	j.Target = "Concurrent.md"
	if err = atomicWrite(filepath.Join(w.Path, j.Target), []byte(after)); err != nil {
		t.Fatal(err)
	}
	if err = s.rollbackMove(w, root, &j); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(w.Path, j.Target)); err != nil {
		t.Fatal("concurrent file lost")
	}
}

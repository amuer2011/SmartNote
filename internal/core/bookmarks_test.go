package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBookmarksPersistMoveBackupAndDelete(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "书签笔记")
	n, e := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n第一段\n收藏这里\n后一段"})
	if e != nil {
		t.Fatal(e)
	}
	row := strings.Count(strings.Split(n.Content, "收藏这里")[0], "\n")
	input := BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Line: row, Column: 2, Label: "重点"}
	b, e := s.AddBookmark(input)
	if e != nil || b.Text != "收藏这里" || b.Before != "第一段" || b.After != "后一段" {
		t.Fatal(b, e)
	}
	same, e := s.AddBookmark(input)
	if e != nil || same.ID != b.ID {
		t.Fatal(same, e)
	}
	bad := input
	bad.Hash = "stale"
	if _, e = s.AddBookmark(bad); e == nil {
		t.Fatal("stale accepted")
	}
	bad = input
	bad.Line = -1
	if _, e = s.AddBookmark(bad); e == nil {
		t.Fatal("invalid line accepted")
	}
	if _, e = s.MoveNote(n.ID, n.Hash, "新目录/重命名.md"); e != nil {
		t.Fatal(e)
	}
	reopened, e := New(s.root)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	items, e := reopened.GetBookmarks(w.ID)
	if e != nil || len(items) != 1 || items[0].NoteID != n.ID {
		t.Fatal(items, e)
	}
	backup := filepath.Join(t.TempDir(), "backup.zip")
	if e = s.Export(backup, true); e != nil {
		t.Fatal(e)
	}
	restored, e := s.Import(backup, filepath.Join(t.TempDir(), "restored"))
	if e != nil {
		t.Fatal(e)
	}
	items, e = s.GetBookmarks(restored.ID)
	if e != nil || len(items) != 1 || items[0].ID != b.ID {
		t.Fatal(items, e)
	}
	if e = s.DeleteBookmark(restored.ID, b.ID); e != nil {
		t.Fatal(e)
	}
	items, e = s.GetBookmarks(restored.ID)
	if e != nil || len(items) != 0 {
		t.Fatal(items, e)
	}
	if _, e = s.GetBookmarks(w.ID); e == nil {
		t.Fatal("inactive workspace accepted")
	}
}

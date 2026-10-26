package core

import (
	"strings"
	"testing"
)

func bookmarkNoteFixture(t *testing.T, s *Service, title string) Note {
	t.Helper()
	n := mustNote(t, s, title)
	n, err := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n第一段\n位置行\n后一段"})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func bookmarkLineOf(t *testing.T, n Note, text string) int {
	t.Helper()
	for i, line := range strings.Split(strings.ReplaceAll(n.Content, "\r\n", "\n"), "\n") {
		if line == text {
			return i
		}
	}
	t.Fatalf("内容中不存在行 %q", text)
	return -1
}

func TestBookmarkNoteAndPositionCoexist(t *testing.T) {
	s, w := fixture(t)
	n := bookmarkNoteFixture(t, s, "整篇笔记")
	row := bookmarkLineOf(t, n, "位置行")

	noteInput := BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: "note", Line: 12345, Column: 6789}
	note, err := s.AddBookmark(noteInput)
	if err != nil {
		t.Fatal(err)
	}
	if note.Kind != "note" {
		t.Fatalf("kind = %q, want note", note.Kind)
	}
	if note.Line != 0 || note.Column != 0 {
		t.Fatalf("note bookmark should not keep location: %+v", note)
	}
	if note.Text != "" || note.Before != "" || note.After != "" {
		t.Fatalf("note bookmark should not keep text context: %+v", note)
	}
	if note.Label != n.Title {
		t.Fatalf("label = %q, want note title %q", note.Label, n.Title)
	}

	positionInput := BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: "position", Line: row, Column: 0}
	position, err := s.AddBookmark(positionInput)
	if err != nil {
		t.Fatal(err)
	}
	if position.Kind != "position" || position.Line != row || position.Text != "位置行" {
		t.Fatalf("unexpected position bookmark: %+v", position)
	}
	if position.ID == note.ID {
		t.Fatal("note and position bookmarks must not be deduplicated together")
	}

	items, err := s.GetBookmarks(w.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("bookmarks = %+v, err = %v", items, err)
	}

	sameNote, err := s.AddBookmark(noteInput)
	if err != nil || sameNote.ID != note.ID {
		t.Fatalf("repeated note bookmark = %+v, err = %v", sameNote, err)
	}
	samePosition, err := s.AddBookmark(positionInput)
	if err != nil || samePosition.ID != position.ID {
		t.Fatalf("repeated position bookmark = %+v, err = %v", samePosition, err)
	}
	items, err = s.GetBookmarks(w.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("bookmarks after repeats = %+v, err = %v", items, err)
	}
}

func TestBookmarkNoteDeduplicates(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "笔记去重")
	input := BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: "note"}

	first, err := s.AddBookmark(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.AddBookmark(input)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate note bookmark ids differ: %q vs %q", first.ID, second.ID)
	}
	items, err := s.GetBookmarks(w.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("bookmarks = %+v, err = %v", items, err)
	}
}

func TestBookmarkNoteStillDeduplicatesAfterEdit(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "编辑后去重")
	input := BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: "note"}

	first, err := s.AddBookmark(input)
	if err != nil {
		t.Fatal(err)
	}
	edited, err := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n编辑后的正文\n"})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Hash == n.Hash {
		t.Fatal("editing the note should change its hash")
	}
	again, err := s.AddBookmark(BookmarkInput{WorkspaceID: w.ID, NoteID: edited.ID, Hash: edited.Hash, Kind: "note"})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID {
		t.Fatalf("note bookmark changed id after edit: %q vs %q", first.ID, again.ID)
	}
	if again.Line != 0 || again.Column != 0 || again.Text != "" || again.Before != "" || again.After != "" {
		t.Fatalf("unexpected note bookmark after edit: %+v", again)
	}
	items, err := s.GetBookmarks(w.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("bookmarks = %+v, err = %v", items, err)
	}
}

func TestBookmarkKindPersistsAcrossRestart(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "持久化种类")
	created, err := s.AddBookmark(BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: "note"})
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	items, err := reopened.GetBookmarks(w.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("bookmarks = %+v, err = %v", items, err)
	}
	got := items[0]
	if got.ID != created.ID || got.Kind != "note" {
		t.Fatalf("persisted bookmark = %+v, want kind note", got)
	}
	if got.Line != 0 || got.Column != 0 || got.Text != "" || got.Before != "" || got.After != "" {
		t.Fatalf("persisted note bookmark should not keep location: %+v", got)
	}
}

func TestBookmarkLegacyPositionKind(t *testing.T) {
	s, w := fixture(t)
	n := bookmarkNoteFixture(t, s, "旧位置书签")
	row := bookmarkLineOf(t, n, "位置行")
	input := BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Line: row, Column: 0}

	legacy, err := s.AddBookmark(input)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Line != row || legacy.Column != 0 || legacy.Text != "位置行" {
		t.Fatalf("legacy empty kind should behave as position: %+v", legacy)
	}
	if legacy.Kind != "" && legacy.Kind != "position" {
		t.Fatalf("legacy empty kind stored as %q", legacy.Kind)
	}

	explicit := input
	explicit.Kind = "position"
	same, err := s.AddBookmark(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if same.ID != legacy.ID {
		t.Fatalf("empty kind and position should deduplicate together: %q vs %q", legacy.ID, same.ID)
	}
	items, err := s.GetBookmarks(w.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("bookmarks = %+v, err = %v", items, err)
	}
}

func TestBookmarkRejectsUnknownKind(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "非法种类")
	for _, kind := range []string{"unknown", "Note", "POSITION", "paragraph"} {
		_, err := s.AddBookmark(BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: kind})
		if err == nil {
			t.Fatalf("kind %q should be rejected", kind)
		}
	}
	items, err := s.GetBookmarks(w.ID)
	if err != nil || len(items) != 0 {
		t.Fatalf("rejected bookmarks were stored: %+v, err = %v", items, err)
	}
}

func TestDeleteNoteBookmarkKeepsPositionBookmark(t *testing.T) {
	s, w := fixture(t)
	n := bookmarkNoteFixture(t, s, "删除留位置")
	row := bookmarkLineOf(t, n, "位置行")

	note, err := s.AddBookmark(BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: "note"})
	if err != nil {
		t.Fatal(err)
	}
	position, err := s.AddBookmark(BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Kind: "position", Line: row, Column: 0})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteBookmark(w.ID, note.ID); err != nil {
		t.Fatal(err)
	}
	items, err := s.GetBookmarks(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != position.ID || items[0].Kind != "position" {
		t.Fatalf("position bookmark should survive note bookmark deletion: %+v", items)
	}
}

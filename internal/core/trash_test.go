package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashBrowseDeleteAndEmptyAreWorkspaceScoped(t *testing.T) {
	s, w := fixture(t)
	n, e := s.CreateNote("回收测试", "Inbox")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.DeleteNote(n.ID, n.Hash); e != nil {
		t.Fatal(e)
	}
	writeEntryFile(t, w.Path, "待删除/子目录/文件.txt", "保留正文")
	if _, e = s.DeleteDirectory("待删除"); e != nil {
		t.Fatal(e)
	}
	entries, e := s.ListTrash(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	var note, file TrashEntry
	for _, entry := range entries {
		if entry.Source == "notes" && !entry.Directory {
			note = entry
		}
		if entry.Source == "folders" && entry.Name == "文件.txt" {
			file = entry
		}
	}
	if note.Path == "" || file.Path == "" {
		t.Fatal("both trash sources must be visible", entries)
	}
	content, e := s.ReadTrashFile(w.ID, "notes", note.Path)
	if e != nil || content != n.Content {
		t.Fatal("note preview failed", e)
	}
	content, e = s.ReadTrashFile(w.ID, "folders", file.Path)
	if e != nil || content != "保留正文" {
		t.Fatal("folder file preview failed", e)
	}
	for _, source := range []string{"notes", "folders"} {
		for _, path := range []string{".", "..", "../settings.json", "/", "", `\`, `\..\settings.json`, `C:\Windows`, "batch/../.."} {
			if e = s.DeleteTrashEntry(w.ID, source, path); e == nil {
				t.Fatal("unsafe deletion accepted", source, path)
			}
		}
	}
	if _, e = s.ListTrash("stale-workspace"); e == nil {
		t.Fatal("stale workspace listed")
	}
	if e = s.EmptyTrash("stale-workspace"); e == nil {
		t.Fatal("stale workspace emptied")
	}
	if e = s.DeleteTrashEntry(w.ID, "unknown", file.Path); e == nil {
		t.Fatal("unknown root accepted")
	}
	if _, e = s.MoveDirectory(".smartnote-trash", "已迁移回收站"); e == nil {
		t.Fatal("trash root moved")
	}
	if _, err := s.DeleteDirectory(".smartnote-trash"); err == nil {
		t.Fatal("fixed trash root recycled")
	}
	if e = s.CreateDirectory(".smartnote-trash/新文件夹"); e == nil {
		t.Fatal("ordinary creation in trash accepted")
	}
	if e = s.DeleteTrashEntry(w.ID, "folders", file.Path); e != nil {
		t.Fatal(e)
	}
	entries, e = s.ListTrash(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if entry.Source == "folders" && entry.Path == file.Path {
			t.Fatal("permanent deletion recycled file")
		}
	}
	if e = s.EmptyTrash(w.ID); e != nil {
		t.Fatal(e)
	}
	entries, e = s.ListTrash(w.ID)
	if e != nil || len(entries) != 0 {
		t.Fatal("trash not empty", e, entries)
	}
	for _, source := range []string{"notes", "folders"} {
		root, e := s.trashRoot(w, source)
		if e != nil {
			t.Fatal(e)
		}
		if info, e := os.Stat(root); e != nil || !info.IsDir() {
			t.Fatal("fixed root removed", root, e)
		}
	}
	if _, e = os.Stat(filepath.Join(w.Path, "已迁移回收站")); !os.IsNotExist(e) {
		t.Fatal("trash was migrated", e)
	}
	snap, e := s.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	for _, dir := range snap.Directories {
		if strings.Contains(dir, ".smartnote-trash") {
			t.Fatal("trash leaked into normal movable tree")
		}
	}
}

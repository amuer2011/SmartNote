package core

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWorkspaceDirectoriesAndNestedNoteCreation(t *testing.T) {
	s, w := fixture(t)
	for _, dir := range []string{"空文件夹/子目录", ".hidden/private"} {
		if err := os.MkdirAll(filepath.Join(w.Path, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.CreateNote("新文件", "工作/项目/设计")
	if err != nil {
		t.Fatal(err)
	}
	if n.Path != "工作/项目/设计/新文件.md" {
		t.Fatalf("unexpected path: %s", n.Path)
	}
	if _, err := os.ReadFile(filepath.Join(w.Path, filepath.FromSlash(n.Path))); err != nil {
		t.Fatal(err)
	}
	rootNote, err := s.CreateNote("根目录笔记", ".")
	if err != nil || rootNote.Path != "根目录笔记.md" {
		t.Fatalf("root note: %+v %v", rootNote, err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"空文件夹", "空文件夹/子目录", "工作", "工作/项目", "工作/项目/设计"} {
		if !slices.Contains(snap.Directories, want) {
			t.Fatalf("missing %s: %v", want, snap.Directories)
		}
	}
	if slices.Contains(snap.Directories, ".hidden") {
		t.Fatal("hidden directory exposed")
	}
	for _, dir := range []string{"../outside", filepath.Join(w.Path, "absolute"), ".hidden/private"} {
		if _, err := s.CreateNote("不能写入", dir); err == nil {
			t.Fatalf("unsafe directory accepted: %s", dir)
		}
	}
}

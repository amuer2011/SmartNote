package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyEntryNamesIdentitiesAndSourcePreservation(t *testing.T) {
	s, w := fixture(t)
	n, err := s.CreateNote("方案", "资料")
	if err != nil {
		t.Fatal(err)
	}
	before := entryFileContent(t, w.Path, n.Path)
	for _, want := range []string{"资料/方案-副本.md", "资料/方案-副本(1).md", "资料/方案-副本(2).md"} {
		result, e := s.CopyEntry(n.Path)
		if e != nil || result.Path != want {
			t.Fatalf("copy = %+v, %v; want %s", result, e, want)
		}
	}
	result, err := s.CopyEntry("资料/方案-副本.md")
	if err != nil || result.Path != "资料/方案-副本(3).md" {
		t.Fatalf("copy of copy = %+v, %v", result, err)
	}
	if got := entryFileContent(t, w.Path, n.Path); got != before {
		t.Fatal("source modified")
	}
	ids := entryNoteIDs(t, s)
	seen := map[string]bool{}
	for path, id := range ids {
		if seen[id] {
			t.Fatalf("shared ID: %s", path)
		}
		seen[id] = true
	}
}

func TestCopyDirectoryRetainsTreeAttachmentsAndMarkdownBody(t *testing.T) {
	s, w := fixture(t)
	content := "---\r\nid: original\r\ntitle: 自定义标题\r\ntags: [笔记, 工作]\r\ncustom: 保留\r\n---\r\n\r\n# 正文\r\n\r\n![图片](资源/pic.png)\r\n"
	writeEntryFile(t, w.Path, "项目.v1/说明.MD", content)
	writeEntryFile(t, w.Path, "项目.v1/子目录/plain.md", "# 无元数据\n\n[[说明]]\n")
	writeEntryFile(t, w.Path, "项目.v1/资源/pic.png", "image-bytes")
	if err := os.MkdirAll(filepath.Join(w.Path, "项目.v1", "空目录"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"项目.v1-副本", "项目.v1-副本(1)"} {
		result, err := s.CopyEntry("项目.v1")
		if err != nil || result.Path != want {
			t.Fatalf("copy tree = %+v, %v", result, err)
		}
		copied := entryFileContent(t, w.Path, want+"/说明.MD")
		if !strings.HasSuffix(copied, "\r\n\r\n# 正文\r\n\r\n![图片](资源/pic.png)\r\n") || !strings.Contains(copied, "custom: 保留") {
			t.Fatalf("lost content: %s", copied)
		}
		if entryFileContent(t, w.Path, want+"/资源/pic.png") != "image-bytes" {
			t.Fatal("attachment changed")
		}
		if info, e := os.Stat(filepath.Join(w.Path, want, "空目录")); e != nil || !info.IsDir() {
			t.Fatal("lost empty folder")
		}
	}
	if entryFileContent(t, w.Path, "项目.v1/说明.MD") != content {
		t.Fatal("source changed")
	}
	ids := entryNoteIDs(t, s)
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatal("shared copied note ID")
		}
		seen[id] = true
	}
}

func TestCopyEntryCollisionSafetyAndFailedPreparation(t *testing.T) {
	s, w := fixture(t)
	writeEntryFile(t, w.Path, "报告.md", "# 报告\n")
	if err := os.Mkdir(filepath.Join(w.Path, "报告-副本.md"), 0700); err != nil {
		t.Fatal(err)
	}
	result, err := s.CopyEntry("报告.md")
	if err != nil || result.Path != "报告-副本(1).md" {
		t.Fatalf("directory collision: %+v, %v", result, err)
	}
	for _, source := range []string{"", ".", "..", "../outside.md", ".smartnote-trash", w.Path} {
		if _, e := s.CopyEntry(source); e == nil {
			t.Fatalf("accepted unsafe path: %q", source)
		}
	}
	writeEntryFile(t, w.Path, "坏目录/good.md", "# Good\n")
	writeEntryFile(t, w.Path, "坏目录/broken.md", "---\nid: unclosed\n")
	if _, e := s.CopyEntry("坏目录"); e == nil {
		t.Fatal("accepted malformed metadata")
	}
	if _, e := os.Stat(filepath.Join(w.Path, "坏目录-副本")); !os.IsNotExist(e) {
		t.Fatal("failed operation published partial folder")
	}
	stages, e := filepath.Glob(filepath.Join(w.Path, ".smartnote-copy-*"))
	if e != nil || len(stages) > 0 {
		t.Fatalf("staging remains: %v, %v", stages, e)
	}
}

package core

import (
	"strings"
	"testing"
)

func TestPasteRebasesOnlyCopiedLinks(t *testing.T) {
	s, w := fixture(t)
	if err := s.CreateDirectory("归档"); err != nil {
		t.Fatal(err)
	}
	writeEntryFile(t, w.Path, "资料/note.md", "# 笔记\n[附件](assets/pic.png)\n[其他](other.md#part)\n[self](note.md)\n")
	writeEntryFile(t, w.Path, "资料/other.md", "# Other\n")
	writeEntryFile(t, w.Path, "资料/assets/pic.png", "image")
	before := entryFileContent(t, w.Path, "资料/note.md")
	result, err := s.PasteEntry("资料/note.md", "归档")
	if err != nil {
		t.Fatal(err)
	}
	copy := entryFileContent(t, w.Path, result.Path)
	for _, target := range []string{encodedPath("../资料/assets/pic.png"), encodedPath("../资料/other.md") + "#part", encodedPath("note-副本.md")} {
		if !strings.Contains(copy, "]("+target+")") {
			t.Fatalf("lost pasted link %s: %s", target, copy)
		}
	}
	if entryFileContent(t, w.Path, "资料/note.md") != before {
		t.Fatal("source links changed")
	}
	writeEntryFile(t, w.Path, "项目/main.md", "# Main\n![内部](assets/pic.png)\n[外部](../资料/other.md)\n[[项目/child]]\n")
	writeEntryFile(t, w.Path, "项目/child.md", "# Child\n")
	writeEntryFile(t, w.Path, "项目/assets/pic.png", "image")
	result, err = s.PasteEntry("项目", "归档")
	if err != nil {
		t.Fatal(err)
	}
	copy = entryFileContent(t, w.Path, result.Path+"/main.md")
	for _, target := range []string{"![内部](assets/pic.png)", "](" + encodedPath("../../资料/other.md") + ")", "[[归档/项目-副本/child]]"} {
		if !strings.Contains(copy, target) {
			t.Fatalf("folder link not rebased: %s\n%s", target, copy)
		}
	}
}

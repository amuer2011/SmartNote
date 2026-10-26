package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPasteNoteCopiesToDirectoryWithFreshIDs(t *testing.T) {
	s, w := fixture(t)
	content := "---\nid: original\n标题: 方案\n---\n\n# 正文\n\n粘贴副本。\n"
	writeEntryFile(t, w.Path, "资料/方案.md", content)
	if err := os.MkdirAll(filepath.Join(w.Path, "归档"), 0700); err != nil {
		t.Fatal(err)
	}
	before := entryFileContent(t, w.Path, "资料/方案.md")
	for _, want := range []string{"归档/方案-副本.md", "归档/方案-副本(1).md"} {
		result, err := s.PasteEntry("资料/方案.md", "归档")
		if err != nil || result.Path != want {
			t.Fatalf("paste = %+v, %v; want %s", result, err, want)
		}
		if got := entryFileContent(t, w.Path, want); !strings.Contains(got, "# 正文") {
			t.Fatalf("lost note body: %s", got)
		}
	}
	if got := entryFileContent(t, w.Path, "资料/方案.md"); got != before {
		t.Fatal("source note changed")
	}
	ids := entryNoteIDs(t, s)
	first, second := ids["归档/方案-副本.md"], ids["归档/方案-副本(1).md"]
	if ids["资料/方案.md"] != "original" || first == "" || second == "" || first == second || first == "original" || second == "original" {
		t.Fatalf("pasted notes must not share identities: %v", ids)
	}
}

func TestPasteDirectoryPreservesTreeInTargetAndRoot(t *testing.T) {
	s, w := fixture(t)
	content := "---\nid: folder-original\ntitle: 自定义标题\n---\n\n# 正文\n\n![图片](资源/pic.png)\n"
	writeEntryFile(t, w.Path, "项目.v1/说明.MD", content)
	writeEntryFile(t, w.Path, "项目.v1/子目录/plain.md", "# 无元数据\n\n[[说明]]\n")
	writeEntryFile(t, w.Path, "项目.v1/资源/pic.png", "image-bytes")
	if err := os.MkdirAll(filepath.Join(w.Path, "项目.v1", "空目录"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(w.Path, "归档"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ directory, want string }{
		{"归档", "归档/项目.v1-副本"},
		{"", "项目.v1-副本"},
		{".", "项目.v1-副本(1)"},
	} {
		result, err := s.PasteEntry("项目.v1", tc.directory)
		if err != nil || result.Path != tc.want {
			t.Fatalf("paste to %q = %+v, %v; want %s", tc.directory, result, err, tc.want)
		}
		copied := entryFileContent(t, w.Path, tc.want+"/说明.MD")
		if !strings.Contains(copied, "# 正文") || !strings.Contains(copied, "![图片](资源/pic.png)") {
			t.Fatalf("lost content in %s: %s", tc.want, copied)
		}
		if entryFileContent(t, w.Path, tc.want+"/子目录/plain.md") == "" {
			t.Fatalf("lost nested note in %s", tc.want)
		}
		if entryFileContent(t, w.Path, tc.want+"/资源/pic.png") != "image-bytes" {
			t.Fatalf("attachment changed in %s", tc.want)
		}
		if info, e := os.Stat(filepath.Join(w.Path, filepath.FromSlash(tc.want), "空目录")); e != nil || !info.IsDir() {
			t.Fatalf("lost empty folder in %s", tc.want)
		}
	}
	if got := entryFileContent(t, w.Path, "项目.v1/说明.MD"); got != content {
		t.Fatal("source directory changed")
	}
	ids := entryNoteIDs(t, s)
	seen := map[string]bool{}
	for _, path := range []string{
		"项目.v1/说明.MD",
		"归档/项目.v1-副本/说明.MD",
		"项目.v1-副本/说明.MD",
		"项目.v1-副本(1)/说明.MD",
	} {
		id := ids[path]
		if id == "" || seen[id] {
			t.Fatalf("missing or shared copied note ID at %s: %v", path, ids)
		}
		seen[id] = true
	}
	if ids["项目.v1/说明.MD"] != "folder-original" {
		t.Fatalf("source ID changed: %v", ids)
	}
}

func TestPasteDirectoryRejectsSelfChildAndInvalidTargets(t *testing.T) {
	s, w := fixture(t)
	content := "---\nid: stable\n---\n\n# 项目\n"
	writeEntryFile(t, w.Path, "项目/说明.md", content)
	writeEntryFile(t, w.Path, "项目/子目录/内部.md", "# 内部\n")
	writeEntryFile(t, w.Path, "普通文件.txt", "不要改动")
	writeEntryFile(t, w.Path, ".hidden/秘密.md", "# 秘密\n")
	for _, dir := range []string{"归档"} {
		if err := os.MkdirAll(filepath.Join(w.Path, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, directory := range []string{
		"项目", "项目/子目录", "../越界", ".hidden", "不存在", "普通文件.txt", filepath.Join(w.Path, "绝对"),
	} {
		if _, err := s.PasteEntry("项目", directory); err == nil {
			t.Errorf("pasted directory into unsafe destination %q", directory)
		}
	}
	for _, source := range []string{".hidden/秘密.md", "不存在.md"} {
		if _, err := s.PasteEntry(source, "归档"); err == nil {
			t.Errorf("pasted invisible source %q", source)
		}
	}
	if got := entryFileContent(t, w.Path, "项目/说明.md"); got != content {
		t.Fatal("rejected paste changed source")
	}
	if got := entryFileContent(t, w.Path, "普通文件.txt"); got != "不要改动" {
		t.Fatal("rejected paste changed unrelated file")
	}
	for _, path := range []string{
		"项目-副本",
		"项目/项目-副本",
		"项目/子目录/项目-副本",
		".hidden/项目-副本",
		"归档/项目-副本",
		"归档/秘密-副本.md",
	} {
		if _, err := os.Lstat(filepath.Join(w.Path, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Errorf("failed paste left partial copy at %s", path)
		}
	}
	if _, err := os.Lstat(filepath.Join(w.Path, "不存在")); !os.IsNotExist(err) {
		t.Error("failed paste created an invalid destination")
	}
	if err := filepath.WalkDir(w.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".smartnote-copy-") {
			t.Errorf("failed paste left staging directory: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

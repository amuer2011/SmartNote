package core

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFolderImportPreservesVaultAndConfiguration(t *testing.T) {
	s, old := fixture(t)
	sample, err := s.CreateNote("已有样本", "分类")
	if err != nil {
		t.Fatal(err)
	}
	config := defaultSemantic()
	config.Threshold, config.Automatic = 0.63, true
	config.Categories = []SemanticCategory{{ID: "custom", Name: "自定义分类", Description: "数据库 SQL", Enabled: true, Directory: "分类", Examples: []SemanticArchiveRef{{NoteID: sample.ID, Hash: sample.Hash, Path: sample.Path}}}}
	if _, err = s.SaveSemanticConfig(old.ID, config); err != nil {
		t.Fatal(err)
	}
	source, parent := t.TempDir(), t.TempDir()
	files := map[string][]byte{
		"工作/入口.md":  []byte("---\ntitle: 原始标题\ntags: [数据库]\ncustom: 保留\n---\n\n[[学习/目标|笔记别名]]\n![[照片.png|图片别名]]\n![标准](../附件/照片.png)\n[[资料.pdf|文档]]\n![缺失](missing.png)\n![重名](same.png)\n`![代码](missing-code.png)`\n"),
		"学习/目标.md":  []byte("# 目标\n这是学习笔记"),
		"BOM.md":    append([]byte{0xef, 0xbb, 0xbf}, []byte("---\ntitle: BOM标题\n---\n正文")...),
		"附件/照片.png": []byte("photo"), "资料/资料.pdf": []byte("pdf"),
		"a/same.png": []byte("one"), "b/same.png": []byte("two"),
		"未引用.txt":                           []byte("保留未引用附件"),
		".obsidian/plugins/example/main.js": []byte("不导入插件"), ".git/config": []byte("不导入版本控制"),
	}
	for name, content := range files {
		if err := atomicWrite(filepath.Join(source, filepath.FromSlash(name)), content); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(source, "空目录", "子目录"), 0755); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	if err := os.Chtimes(filepath.Join(source, "工作", "入口.md"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	result, err := s.ImportFolder(FolderImportInput{Source: source, Parent: parent, Name: "导入知识库", TemplateWorkspaceID: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Notes != 3 || result.Attachments != 5 || result.Skipped != 2 || result.WarningCount != 2 {
		t.Fatalf("unexpected import summary: %+v", result)
	}
	for name, before := range files {
		after, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("source modified: %s %v", name, err)
		}
	}
	for _, name := range []string{"附件/照片.png", "资料/资料.pdf", "未引用.txt", "a/same.png", "b/same.png", "空目录/子目录"} {
		if _, err := os.Stat(filepath.Join(result.Workspace.Path, filepath.FromSlash(name))); err != nil {
			t.Fatal("original layout lost", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(result.Workspace.Path, ".obsidian")); !os.IsNotExist(err) {
		t.Fatal("plugin settings copied")
	}
	config, err = s.GetSemanticConfig(result.Workspace.ID)
	if err != nil || config.Automatic || config.AutomaticVersion != 1 || config.Threshold != 0.63 || len(config.Categories) != 1 || len(config.Categories[0].Examples) != 0 {
		t.Fatalf("preserve mode or category migration failed: %+v %v", config, err)
	}
	snapshot, err := s.Snapshot()
	if err != nil || len(snapshot.Notes) != 3 || len(snapshot.Attachments) != 5 {
		t.Fatalf("import snapshot: %+v %v", snapshot, err)
	}
	var entry Note
	for _, note := range snapshot.Notes {
		if note.Path == "工作/入口.md" {
			entry = note
		}
		if note.Path == "BOM.md" && note.Title != "BOM标题" {
			t.Fatal("UTF-8 BOM prevented front matter parsing")
		}
	}
	if strings.Count(entry.Content, "smartnote-asset://") != 3 || !strings.Contains(entry.Content, "[[学习/目标|笔记别名]]") || !strings.Contains(entry.Content, "custom: 保留") || !strings.Contains(entry.Content, "missing.png") || !strings.Contains(entry.Content, "same.png") || !entry.Updated.Equal(stamp) {
		t.Fatalf("content or timestamp lost: %+v", entry)
	}
	if archived, err := s.AutoArchive(result.Workspace.ID, entry.ID, entry.Hash); err != nil || archived != nil {
		t.Fatalf("preserve mode classified a note: %+v %v", archived, err)
	}
	edges, err := s.Graph()
	if err != nil || len(edges) != 1 {
		t.Fatalf("Obsidian note link lost: %+v %v", edges, err)
	}
}

func TestFolderImportAutomaticArchiveMaintainsLinksAndSource(t *testing.T) {
	s, old := fixture(t)
	config := defaultSemantic()
	config.Automatic = false
	config.Categories = []SemanticCategory{{ID: "db", Name: "数据库", Description: "数据库 SQL", Directory: "技术/数据库", Enabled: true}}
	if _, err := s.SaveSemanticConfig(old.ID, config); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"入口.md": "[[子目录/数据库]]", "子目录/数据库.md": "# 数据库\n![图片](../图片.png)", "图片.png": "image"} {
		if err := atomicWrite(filepath.Join(source, filepath.FromSlash(name)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.ImportFolder(FolderImportInput{Source: source, Organize: true, TemplateWorkspaceID: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	s.embedTexts = func(texts []string) ([][]float64, error) {
		vectors := make([][]float64, len(texts))
		for i := range vectors {
			vectors[i] = []float64{1, 0}
		}
		return vectors, nil
	}
	snapshot, _ := s.Snapshot()
	for _, note := range snapshot.Notes {
		if note.Path == "子目录/数据库.md" {
			moved, err := s.AutoArchive(out.Workspace.ID, note.ID, note.Hash)
			if err != nil || moved == nil || moved.Note.Path != "技术/数据库/数据库.md" || !strings.Contains(moved.Note.Content, "smartnote-asset://") {
				t.Fatalf("automatic import organization: %+v %v", moved, err)
			}
		}
	}
	entry, _ := os.ReadFile(filepath.Join(out.Workspace.Path, "入口.md"))
	if !strings.Contains(string(entry), "[[技术/数据库/数据库]]") {
		t.Fatal("backlink not updated", string(entry))
	}
	if out.Workspace.Path != source {
		t.Fatal("direct import created another directory")
	}
	if _, err := os.Stat(filepath.Join(source, "子目录", "数据库.md")); !os.IsNotExist(err) {
		t.Fatal("direct organization did not move the selected directory's note")
	}
}

func TestFolderImportDirectKeepsDirectoryAndReusesRegistration(t *testing.T) {
	s, old := fixture(t)
	source := t.TempDir()
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{"子目录/入口.md": "# 入口\n![图片](../图片.png)", "图片.png": "image", ".obsidian/config.json": "keep config"} {
		if err := atomicWrite(filepath.Join(source, filepath.FromSlash(path)), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(source, "空目录"), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := s.ImportFolder(FolderImportInput{Source: source, TemplateWorkspaceID: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workspace.Path != source || out.Workspace.Name != filepath.Base(source) || out.Notes != 1 || out.Attachments != 1 {
		t.Fatalf("not imported directly: %+v", out)
	}
	if config, err := s.GetSemanticConfig(out.Workspace.ID); err != nil || config.Automatic {
		t.Fatalf("preserve mode: %+v %v", config, err)
	}
	if config, _ := os.ReadFile(filepath.Join(source, ".obsidian", "config.json")); string(config) != "keep config" {
		t.Fatal("hidden configuration changed")
	}
	if _, err := os.Stat(filepath.Join(source, "空目录")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(source, "子目录", "入口.md")); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenWorkspace(old.ID); err != nil {
		t.Fatal(err)
	}
	again, err := s.ImportFolder(FolderImportInput{Source: source, Organize: true, TemplateWorkspaceID: old.ID})
	if err != nil || again.Workspace.ID != out.Workspace.ID || again.Attachments != 1 || len(s.Settings().Workspaces) != 2 {
		t.Fatalf("reimport lost metadata or duplicated registration: %+v %v", again, err)
	}
	if config, err := s.GetSemanticConfig(out.Workspace.ID); err != nil || !config.Automatic {
		t.Fatalf("reimport mode: %+v %v", config, err)
	}
	if stages, _ := filepath.Glob(filepath.Join(s.root, ".smartnote-folder-import-*")); len(stages) != 0 {
		t.Fatal("staging folders left", stages)
	}
}

func TestFolderImportDirectRollsBackOnRegistrationFailure(t *testing.T) {
	s, old := fixture(t)
	source := t.TempDir()
	body := "# 入口\n![图片](图片.png)"
	for path, content := range map[string]string{"入口.md": body, "第二篇.md": body, "图片.png": "image"} {
		if err := atomicWrite(filepath.Join(source, path), []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	settingsPath := filepath.Join(s.root, "settings.json")
	if err := os.Remove(settingsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(settingsPath, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportFolder(FolderImportInput{Source: source}); err == nil {
		t.Fatal("registration failure ignored")
	}
	for _, path := range []string{"入口.md", "第二篇.md"} {
		got, err := os.ReadFile(filepath.Join(source, path))
		if err != nil || string(got) != body {
			t.Fatalf("failed import modified %s: %s %v", path, got, err)
		}
	}
	if settings := s.Settings(); settings.ActiveID != old.ID || len(settings.Workspaces) != 1 {
		t.Fatal("failed direct import changed registration")
	}
}

func TestFolderImportRejectsUnsafeOrInvalidTargets(t *testing.T) {
	for _, kind := range []string{"existing", "nested", "traversal", "reserved", "no-notes", "duplicate-id", "invalid-utf8"} {
		t.Run(kind, func(t *testing.T) {
			s, old := fixture(t)
			source, parent := t.TempDir(), t.TempDir()
			if err := atomicWrite(filepath.Join(source, "note.md"), []byte("---\nid: same\n---\n正文")); err != nil {
				t.Fatal(err)
			}
			input := FolderImportInput{Source: source, Parent: parent, Name: "新空间"}
			switch kind {
			case "existing":
				if err := atomicWrite(filepath.Join(parent, input.Name, "keep.md"), []byte("原有数据")); err != nil {
					t.Fatal(err)
				}
			case "nested":
				input.Parent = source
			case "traversal":
				input.Name = "../escape"
			case "reserved":
				input.Name = "CON"
			case "no-notes":
				input.Source = t.TempDir()
			case "invalid-utf8":
				if err := atomicWrite(filepath.Join(source, "invalid.md"), []byte{0xff, 0xfe, 0x41}); err != nil {
					t.Fatal(err)
				}
			case "duplicate-id":
				if err := atomicWrite(filepath.Join(source, "duplicate.md"), []byte("---\nid: same\n---\n另一个正文")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.ImportFolder(input); err == nil {
				t.Fatal("invalid folder import accepted")
			}
			if settings := s.Settings(); settings.ActiveID != old.ID || len(settings.Workspaces) != 1 {
				t.Fatal("failed import changed workspace registration")
			}
			if stages, _ := filepath.Glob(filepath.Join(parent, ".smartnote-folder-import-*")); len(stages) > 0 {
				t.Fatal("failed import left staging files", stages)
			}
			if kind == "existing" {
				b, _ := os.ReadFile(filepath.Join(parent, input.Name, "keep.md"))
				if string(b) != "原有数据" {
					t.Fatal("existing data changed")
				}
			}
		})
	}
}

func TestFolderImportDoesNotReadResourcesOutsideVault(t *testing.T) {
	s, _ := fixture(t)
	base := t.TempDir()
	source := filepath.Join(base, "vault")
	body := "# 外部附件\n![库外图片](../private.png)\n![无效链接](broken%ZZ.png)\n"
	if err := atomicWrite(filepath.Join(source, "入口.md"), []byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(filepath.Join(base, "private.png"), []byte("must not be imported")); err != nil {
		t.Fatal(err)
	}
	out, err := s.ImportFolder(FolderImportInput{Source: source, Parent: t.TempDir(), Name: "边界测试"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Attachments != 0 || out.WarningCount != 2 {
		t.Fatalf("external resource imported or warnings lost: %+v", out)
	}
	copy, err := os.ReadFile(filepath.Join(out.Workspace.Path, "入口.md"))
	if err != nil || string(copy) != body {
		t.Fatalf("unresolved links changed: %s %v", copy, err)
	}
}

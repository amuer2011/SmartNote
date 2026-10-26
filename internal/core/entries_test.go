package core

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// entryFileContent 按工作区相对斜杠路径读取文件内容。
func entryFileContent(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("读取 %s: %v", rel, err)
	}
	return string(b)
}

func writeEntryFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func entryNoteIDs(t *testing.T, s *Service) map[string]string {
	t.Helper()
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, n := range snap.Notes {
		ids[n.Path] = n.ID
	}
	return ids
}

func TestMoveDirectoryCarriesTreeAndRewritesLinks(t *testing.T) {
	s, w := fixture(t)

	main, err := s.CreateNote("主笔记", "项目/设计")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveNote(SaveInput{Path: main.Path, Hash: main.Hash, Content: main.Content +
		"[detail](detail.md)\n[资源数据](../资源/数据.txt)\n[[外部/引用者]]\n"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateNote("detail", "项目/设计"); err != nil {
		t.Fatal(err)
	}
	// 无 Front Matter 的笔记依赖合成 ID 注入来保持身份。
	writeEntryFile(t, w.Path, "项目/设计/外部笔记.md", "# 外部笔记\n\n可移动。\n")
	writeEntryFile(t, w.Path, "项目/设计/资源/原始数据.dat", "非 Markdown 原始内容")
	if err = os.MkdirAll(filepath.Join(w.Path, "项目", "设计", "空目录"), 0700); err != nil {
		t.Fatal(err)
	}
	writeEntryFile(t, w.Path, "项目/资源/数据.txt", "外部资源")
	writeEntryFile(t, w.Path, "外部/引用者.md", "# 引用者\n\n"+
		"[设计稿](../项目/设计/主笔记.md)\n"+
		"[设计片段](../项目/设计/主笔记.md#小节)\n"+
		"[[项目/设计/主笔记|设计稿]]\n"+
		"[[主笔记]]\n"+
		"`[代码](../项目/设计/主笔记.md)`\n")

	before := entryNoteIDs(t, s)
	for _, path := range []string{"项目/设计/主笔记.md", "项目/设计/外部笔记.md"} {
		if before[path] == "" {
			t.Fatalf("前置条件缺失: %s 未出现在快照中", path)
		}
	}

	result, err := s.MoveDirectory("项目/设计", "归档/设计")
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != "归档/设计" {
		t.Fatalf("Path = %q，期望 归档/设计", result.Path)
	}
	// 移动笔记指向外部的相对链接 1 处，外部笔记的 4 个链接（Markdown/Wiki/片段）。
	if result.UpdatedLinks != 5 {
		t.Fatalf("UpdatedLinks = %d，期望 5", result.UpdatedLinks)
	}
	if result.RecoveryPath == "" {
		t.Fatal("RecoveryPath 为空")
	}
	var journal directoryJournal
	if err = readJSON(filepath.Join(result.RecoveryPath, "journal.json"), &journal); err != nil {
		t.Fatal(err)
	}
	if journal.State != "committed" || journal.Source != "项目/设计" || journal.Target != "归档/设计" {
		t.Fatalf("移动日志未记录完成状态: %+v", journal)
	}
	if _, err = os.Stat(filepath.Join(w.Path, "项目", "设计")); !os.IsNotExist(err) {
		t.Fatal("移动后源目录仍然存在")
	}

	moved := entryFileContent(t, w.Path, "归档/设计/主笔记.md")
	for _, want := range []string{
		"[detail](detail.md)", // 同目录内的链接无需改写
		"../../%E9%A1%B9%E7%9B%AE/%E8%B5%84%E6%BA%90/%E6%95%B0%E6%8D%AE.txt", // 移出后重新指向外部
		"[[外部/引用者]]", // 指向外部笔记的 Wiki 链接保持不变
	} {
		if !strings.Contains(moved, want) {
			t.Fatalf("移动后的笔记缺少 %q:\n%s", want, moved)
		}
	}

	referrer := entryFileContent(t, w.Path, "外部/引用者.md")
	for _, want := range []string{
		"../%E5%BD%92%E6%A1%A3/%E8%AE%BE%E8%AE%A1/%E4%B8%BB%E7%AC%94%E8%AE%B0.md",
		"../%E5%BD%92%E6%A1%A3/%E8%AE%BE%E8%AE%A1/%E4%B8%BB%E7%AC%94%E8%AE%B0.md#%E5%B0%8F%E8%8A%82",
		"[[归档/设计/主笔记|设计稿]]",
		"[[归档/设计/主笔记]]",
		"`[代码](../项目/设计/主笔记.md)`", // 行内代码中的链接不能被改写
	} {
		if !strings.Contains(referrer, want) {
			t.Fatalf("外部笔记缺少 %q:\n%s", want, referrer)
		}
	}

	if info, err := os.Stat(filepath.Join(w.Path, "归档", "设计", "空目录")); err != nil || !info.IsDir() {
		t.Fatalf("空子目录没有随文件夹移动: %v", err)
	}
	if got := entryFileContent(t, w.Path, "归档/设计/资源/原始数据.dat"); got != "非 Markdown 原始内容" {
		t.Fatalf("非 Markdown 文件内容变化: %q", got)
	}

	after := entryNoteIDs(t, s)
	for _, pair := range [][2]string{
		{"项目/设计/主笔记.md", "归档/设计/主笔记.md"},
		{"项目/设计/detail.md", "归档/设计/detail.md"},
		{"项目/设计/外部笔记.md", "归档/设计/外部笔记.md"},
	} {
		if before[pair[0]] == "" || after[pair[1]] != before[pair[0]] {
			t.Fatalf("笔记 ID 未保留: %s -> %s (%q != %q)", pair[0], pair[1], after[pair[1]], before[pair[0]])
		}
	}
	if !strings.Contains(entryFileContent(t, w.Path, "归档/设计/外部笔记.md"), "id: "+before["项目/设计/外部笔记.md"]) {
		t.Fatal("无 Front Matter 的笔记移动后未写入稳定 ID")
	}

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range snap.Notes {
		if strings.HasPrefix(n.Path, "项目/设计/") {
			t.Fatalf("快照仍显示旧路径: %s", n.Path)
		}
	}
	if !slices.Contains(snap.Directories, "归档/设计/空目录") {
		t.Fatalf("快照缺少移动后的空目录: %v", snap.Directories)
	}
}

func TestMoveDirectoryRejectsUnsafeTargets(t *testing.T) {
	s, w := fixture(t)
	if err := s.CreateDirectory("项目/设计"); err != nil {
		t.Fatal(err)
	}
	writeEntryFile(t, w.Path, "项目/设计/笔记.md", "# 笔记\n")
	if err := s.CreateDirectory("已有目标"); err != nil {
		t.Fatal(err)
	}
	writeEntryFile(t, w.Path, "目标文件.md", "# 目标\n")
	writeEntryFile(t, w.Path, "普通文件.txt", "内容")

	for _, c := range []struct{ source, target, why string }{
		{"项目/设计", "项目/设计", "自身"},
		{"项目/设计", "项目/设计/内部/更深", "自己的子目录"},
		{"项目/设计", "已有目标", "目标目录已存在"},
		{"项目/设计", "目标文件.md", "目标是已存在文件"},
		{"项目/设计", "普通文件.txt", "目标是普通文件"},
		{"不存在的目录", "归档/新目录", "源不存在"},
		{"普通文件.txt", "归档/文件", "源不是文件夹"},
		{".", "归档/根目录", "不允许移动根目录"},
		{"项目/设计", ".", "目标为根目录"},
		{"项目/设计", "", "空目标"},
		{"项目/设计", "../越界", "越界目标"},
		{"../越界", "归档/x", "越界源"},
		{"项目/设计", ".hidden/秘密", "隐藏目标"},
		{"项目/设计", filepath.Join(w.Path, "绝对"), "绝对目标"},
	} {
		if _, err := s.MoveDirectory(c.source, c.target); err == nil {
			t.Errorf("应拒绝移动 %s -> %s（%s）", c.source, c.target, c.why)
		}
	}
	if _, err := os.Stat(filepath.Join(w.Path, "项目", "设计", "笔记.md")); err != nil {
		t.Fatal("被拒绝的操作破坏了源目录")
	}
	if _, err := os.Stat(filepath.Join(w.Path, "项目", "设计", "内部")); !os.IsNotExist(err) {
		t.Fatal("被拒绝的操作创建了目标目录")
	}
	if _, err := s.MoveDirectory("项目/设计", "归档/设计"); err != nil {
		t.Fatalf("拒绝之后合法移动仍应成功: %v", err)
	}
}

func TestDeleteDirectoryMovesWholeTreeToTrash(t *testing.T) {
	s, w := fixture(t)
	note, err := s.CreateNote("待删笔记", "待删除")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateNote("深层笔记", "待删除/子"); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(w.Path, "待删除", "子", "空目录"), 0700); err != nil {
		t.Fatal(err)
	}
	writeEntryFile(t, w.Path, "待删除/素材/数据.bin", "非 Markdown 内容")
	writeEntryFile(t, w.Path, "待删除/.隐藏文件", "隐藏内容")

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(snap.Directories, "待删除") {
		t.Fatalf("前置条件：目录未出现在快照中: %v", snap.Directories)
	}

	result, err := s.DeleteDirectory("待删除")
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != "待删除" {
		t.Fatalf("Path = %q，期望 待删除", result.Path)
	}
	trashRoot := filepath.Join(w.Path, ".smartnote-trash")
	if result.RecoveryPath != trashRoot {
		t.Fatalf("RecoveryPath = %q，期望回收根目录 %q", result.RecoveryPath, trashRoot)
	}
	if _, err = os.Stat(filepath.Join(w.Path, "待删除")); !os.IsNotExist(err) {
		t.Fatal("删除后源目录仍然存在")
	}

	// 整体改名后内容应完整出现在回收根目录下，包括空目录、非 Markdown 和隐藏文件。
	found := ""
	if err = filepath.WalkDir(trashRoot, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() && d.Name() == "待删除" {
			found = p
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Fatal("回收目录中没有找到被删除的文件夹")
	}
	if got := entryFileContent(t, found, filepath.Base(note.Path)); got != note.Content {
		t.Fatalf("回收的笔记内容变化: %q != %q", got, note.Content)
	}
	entryFileContent(t, found, "子/深层笔记.md")
	if got := entryFileContent(t, found, "素材/数据.bin"); got != "非 Markdown 内容" {
		t.Fatalf("非 Markdown 文件未完整回收: %q", got)
	}
	if got := entryFileContent(t, found, ".隐藏文件"); got != "隐藏内容" {
		t.Fatalf("隐藏文件未完整回收: %q", got)
	}
	if info, err := os.Stat(filepath.Join(found, "子", "空目录")); err != nil || !info.IsDir() {
		t.Fatalf("空子目录未随文件夹回收: %v", err)
	}
	recoveryJSON, err := os.ReadFile(filepath.Join(filepath.Dir(found), ".recovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(recoveryJSON), `"original": "待删除"`) {
		t.Fatalf("回收元数据缺少原始路径: %s", recoveryJSON)
	}

	snap, err = s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range snap.Notes {
		if strings.HasPrefix(n.Path, "待删除") {
			t.Fatalf("快照仍显示已删除的笔记: %s", n.Path)
		}
	}
	for _, dir := range snap.Directories {
		if dir == "待删除" || strings.HasPrefix(dir, "待删除/") || strings.Contains(dir, ".smartnote-trash") {
			t.Fatalf("快照仍显示已删除的目录: %s", dir)
		}
	}

	if _, err = s.DeleteDirectory("不存在的目录"); err == nil {
		t.Fatal("删除不存在的目录应被拒绝")
	}
	writeEntryFile(t, w.Path, "普通文件.txt", "内容")
	for _, p := range []string{".", "../越界", ".hidden", "普通文件.txt", filepath.Join(w.Path, "绝对")} {
		if _, err = s.DeleteDirectory(p); err == nil {
			t.Errorf("应拒绝删除 %q", p)
		}
	}
	if _, err = os.Stat(filepath.Join(w.Path, "普通文件.txt")); err != nil {
		t.Fatal("被拒绝的删除影响了其他文件")
	}
}

func TestCreateDirectoryNestedAndRejections(t *testing.T) {
	s, w := fixture(t)
	if err := s.CreateDirectory("新目录/嵌套/更深"); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"新目录", "新目录/嵌套", "新目录/嵌套/更深"} {
		info, err := os.Stat(filepath.Join(w.Path, filepath.FromSlash(dir)))
		if err != nil || !info.IsDir() {
			t.Fatalf("目录未创建: %s (%v)", dir, err)
		}
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"新目录", "新目录/嵌套", "新目录/嵌套/更深"} {
		if !slices.Contains(snap.Directories, dir) {
			t.Fatalf("快照缺少空目录 %s: %v", dir, snap.Directories)
		}
	}
	if err = s.CreateDirectory("新目录"); err == nil {
		t.Fatal("重名目录应被拒绝")
	}
	if err = s.CreateDirectory("新目录/嵌套"); err == nil {
		t.Fatal("重名嵌套目录应被拒绝")
	}
	writeEntryFile(t, w.Path, "普通文件.txt", "内容")
	if err = s.CreateDirectory("普通文件.txt"); err == nil {
		t.Fatal("与文件重名应被拒绝")
	}
	if err = s.CreateDirectory("普通文件.txt/子"); err == nil {
		t.Fatal("在文件之下创建目录应被拒绝")
	}
	for _, p := range []string{"", ".", "../越界", ".hidden/秘密", "C:/越界", filepath.Join(w.Path, "绝对")} {
		if err = s.CreateDirectory(p); err == nil {
			t.Errorf("应拒绝创建目录 %q", p)
		}
	}
	if got := entryFileContent(t, w.Path, "普通文件.txt"); got != "内容" {
		t.Fatalf("被拒绝的创建破坏了现有文件: %q", got)
	}
}

func TestInterruptedDirectoryMoveRecovery(t *testing.T) {
	for _, externalEdit := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "preserve-external-edit"}[externalEdit], func(t *testing.T) {
			s, w := fixture(t)
			n, err := s.CreateNote("恢复", "Source/Empty")
			if err != nil {
				t.Fatal(err)
			}
			before := n.Content
			after := before + "\noperation edit\n"
			recovery := filepath.Join(s.data(w), "directory-operations", "interrupted")
			j := directoryJournal{State: "prepared", Source: "Source", Target: "Target", Changes: []moveChange{{Path: n.Path, BeforeHash: n.Hash, AfterHash: digest([]byte(after)), Backup: "0.md"}}}
			if err = atomicWrite(filepath.Join(recovery, "0.md"), []byte(before)); err != nil {
				t.Fatal(err)
			}
			if err = writeJSON(filepath.Join(recovery, "journal.json"), j); err != nil {
				t.Fatal(err)
			}
			if err = os.Rename(filepath.Join(w.Path, "Source"), filepath.Join(w.Path, "Target")); err != nil {
				t.Fatal(err)
			}
			moved := filepath.Join(w.Path, filepath.FromSlash(mappedEntry(n.Path, "Source", "Target")))
			if externalEdit {
				after = "external changes that must survive"
			}
			if err = atomicWrite(moved, []byte(after)); err != nil {
				t.Fatal(err)
			}
			err = s.recoverDirectories()
			if externalEdit {
				if err == nil {
					t.Fatal("external changes should require manual recovery")
				}
				if b, e := os.ReadFile(moved); e != nil || string(b) != after {
					t.Fatal("external content was overwritten")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if got := entryFileContent(t, w.Path, n.Path); got != before {
					t.Fatal("original content not recovered")
				}
				if _, err = os.Stat(filepath.Join(w.Path, "Target")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("uncommitted target remains")
				}
			}
		})
	}
}

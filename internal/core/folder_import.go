package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type FolderImportInput struct {
	Source              string `json:"source"`
	Parent              string `json:"parent"`
	Name                string `json:"name"`
	Organize            bool   `json:"organize"`
	TemplateWorkspaceID string `json:"templateWorkspaceId"`
}

type FolderImportResult struct {
	Workspace    Workspace `json:"workspace"`
	Notes        int       `json:"notes"`
	Attachments  int       `json:"attachments"`
	Skipped      int       `json:"skipped"`
	WarningCount int       `json:"warningCount"`
	Warnings     []string  `json:"warnings"`
}

// ImportFolder uses the selected directory directly when Parent and Name are
// empty. Explicit destinations remain supported for older callers.
func (s *Service) ImportFolder(input FolderImportInput) (FolderImportResult, error) {
	out := FolderImportResult{Warnings: []string{}}
	warn := func(message string) {
		out.WarningCount++
		if len(out.Warnings) < 50 {
			out.Warnings = append(out.Warnings, message)
		}
	}
	direct := input.Parent == "" && input.Name == ""
	name := strings.TrimSpace(input.Name)
	if !direct && (name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\:*?\"<>|\x00\r\n") || strings.HasSuffix(name, ".")) {
		return out, errors.New("请输入有效的新工作区名称，名称不能包含路径分隔符")
	}
	if input.Source == "" || (!direct && input.Parent == "") {
		return out, errors.New("请选择笔记文件夹")
	}
	source, err := filepath.Abs(input.Source)
	if err != nil {
		return out, err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return out, err
	}
	parentPath := input.Parent
	if direct {
		parentPath, name = filepath.Dir(source), filepath.Base(source)
		if rel, e := filepath.Rel(source, s.root); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return out, errors.New("笔记文件夹不能包含软件自身的数据目录")
		}
	}
	parent, err := filepath.Abs(parentPath)
	if err != nil {
		return out, err
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return out, fmt.Errorf("保存位置不可用：%w", err)
	}
	for _, directory := range []string{source, parent} {
		info, e := os.Stat(directory)
		if e != nil || !info.IsDir() {
			return out, fmt.Errorf("文件夹不可用：%s", directory)
		}
	}
	dest := source
	if !direct {
		dest, err = safePath(parent, name)
		if err != nil {
			return out, err
		}
		if rel, e := filepath.Rel(source, dest); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return out, errors.New("保存位置不能位于来源文件夹内部，请选择其他位置")
		}
		if _, e := os.Lstat(dest); !errors.Is(e, os.ErrNotExist) {
			return out, errors.New("目标目录已存在，请修改工作区名称；导入不会覆盖现有文件")
		}
	} else if err = probeWorkspaceWrite(source); err != nil {
		return out, fmt.Errorf("所选文件夹无法写入：%w", err)
	}
	s.mu.Lock()
	if direct {
		for _, w := range s.settings.Workspaces {
			if strings.EqualFold(w.Path, source) {
				out, err = s.reopenImportedFolder(w, input.Organize)
				s.mu.Unlock()
				return out, err
			}
		}
	}
	previousID := s.settings.ActiveID
	config := defaultSemantic()
	if input.TemplateWorkspaceID != "" {
		var w Workspace
		w, err = s.archiveWorkspace(input.TemplateWorkspaceID)
		if err == nil {
			config, err = s.semanticConfig(w)
		}
	}
	s.mu.Unlock()
	if err != nil {
		return out, err
	}
	config.Automatic, config.AutomaticVersion, config.Revision = input.Organize, 1, uid()
	// Category descriptions and threshold transfer; examples refer to the old vault.
	for i := range config.Categories {
		config.Categories[i].Examples = nil
	}
	stageParent := parent
	if direct {
		stageParent = s.root
	}
	stage, err := os.MkdirTemp(stageParent, ".smartnote-folder-import-*")
	if err != nil {
		return out, err
	}
	keepRecovery := false
	defer func() {
		if !keepRecovery {
			os.RemoveAll(stage)
		}
	}()
	notesRoot := filepath.Join(stage, "notes")
	if err = os.Mkdir(notesRoot, 0755); err != nil {
		return out, err
	}
	workspace := Workspace{ID: uid(), Name: name, Path: dest}
	committed := false
	defer func() {
		if !committed && !keepRecovery {
			os.RemoveAll(s.data(workspace))
		}
	}()
	files, total := 0, int64(0)
	originalHashes := map[string]string{}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			out.Skipped++
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			out.Skipped++
			warn("跳过链接或特殊文件：" + filepath.ToSlash(rel))
			return nil
		}
		if !entry.IsDir() && (entry.Name() == ".DS_Store" || strings.EqualFold(entry.Name(), "Thumbs.db") || strings.EqualFold(entry.Name(), "desktop.ini")) {
			out.Skipped++
			return nil
		}
		target, e := safePath(notesRoot, filepath.ToSlash(rel))
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		files++
		if files > 20000 {
			return errors.New("文件夹超过 20000 个文件")
		}
		sourcePath, e := safePath(source, filepath.ToSlash(rel))
		if e != nil {
			return e
		}
		file, e := os.Open(sourcePath)
		if e != nil {
			return e
		}
		info, e := file.Stat()
		if e != nil || !info.Mode().IsRegular() {
			file.Close()
			return fmt.Errorf("无法读取普通文件：%s", rel)
		}
		limit := int64(maxAttachment)
		markdown := strings.EqualFold(filepath.Ext(rel), ".md")
		if markdown {
			limit = 32 << 20
		}
		if info.Size() > limit || info.Size() > (2<<30)-total {
			file.Close()
			return fmt.Errorf("文件超过大小限制（笔记32 MB、附件100 MB、总计2 GB）：%s", rel)
		}
		content, e := io.ReadAll(io.LimitReader(file, limit+1))
		file.Close()
		if e != nil {
			return e
		}
		if int64(len(content)) > limit || int64(len(content)) > (2<<30)-total {
			return fmt.Errorf("文件读取期间超过大小限制：%s", rel)
		}
		if markdown && !utf8.Valid(content) {
			return fmt.Errorf("笔记不是 UTF-8 编码，请先转换编码：%s", rel)
		}
		total += int64(len(content))
		if markdown {
			originalHashes[filepath.ToSlash(rel)] = digest(content)
			content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
		}
		if e = atomicWrite(target, content); e != nil {
			return e
		}
		return os.Chtimes(target, info.ModTime(), info.ModTime())
	})
	if err != nil {
		return out, err
	}
	if err = s.convertImportedAssetsOptions(notesRoot, s.data(workspace), importAssetOptions{keepFiles: true, vaultPaths: true, warn: warn}); err != nil {
		return out, err
	}
	notes, err := s.scan(Workspace{ID: workspace.ID, Path: notesRoot})
	if err != nil {
		return out, err
	}
	if len(notes) == 0 {
		return out, errors.New("来源文件夹没有可导入的 .md 笔记")
	}
	if err = validateSemantic(Workspace{Path: notesRoot}, config); err != nil {
		return out, err
	}
	if err = writeJSON(filepath.Join(s.data(workspace), "semantic-archive.json"), config); err != nil {
		return out, err
	}
	attachments := []Attachment{}
	if err = readJSON(filepath.Join(s.data(workspace), "attachments-manifest.json"), &attachments); err != nil {
		return out, err
	}
	out.Workspace, out.Notes, out.Attachments = workspace, len(notes), len(attachments)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings.ActiveID != previousID {
		return out, errors.New("工作区已切换，已取消本次导入，请重新操作")
	}
	if direct {
		keepRecovery, err = s.commitImportedFolder(workspace, notes, originalHashes, stage)
		committed = err == nil
		return out, err
	}
	if _, e := os.Lstat(dest); !errors.Is(e, os.ErrNotExist) {
		return out, errors.New("目标目录已存在，导入未覆盖任何文件")
	}
	if err = os.Rename(notesRoot, dest); err != nil {
		return out, err
	}
	committed = true
	previous := s.settings
	s.settings.Workspaces = append(s.settings.Workspaces, workspace)
	s.settings.ActiveID = workspace.ID
	if err = s.persist(); err != nil {
		s.settings = previous
		return out, fmt.Errorf("文件已导入到 %s，但工作区注册失败；可通过打开本地工作区恢复：%w", dest, err)
	}
	return out, nil
}

package core

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type backupManifest struct {
	Version   int               `json:"version"`
	Workspace Workspace         `json:"workspace"`
	Hashes    map[string]string `json:"hashes"`
}

func (s *Service) Export(dest string, native bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	notes, err := s.scan(w)
	if err != nil {
		return err
	}
	attachments := []Attachment{}
	if err = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &attachments); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".smartnote-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	zw := zip.NewWriter(f)
	manifest := backupManifest{Version: 1, Workspace: w, Hashes: map[string]string{}}
	add := func(name string, b []byte) error {
		writer, e := zw.Create(name)
		if e != nil {
			return e
		}
		if _, e = writer.Write(b); e != nil {
			return e
		}
		manifest.Hashes[name] = digest(b)
		return nil
	}
	for _, n := range notes {
		content := n.Content
		name := n.Path
		if native {
			name = "notes/" + name
		} else {
			for _, a := range attachments {
				if a.Deleted {
					continue
				}
				if a.RemoteOnly {
					content = strings.ReplaceAll(content, "smartnote-asset://"+a.ID, a.RemoteURL)
					continue
				}
				rel, e := filepath.Rel(filepath.Dir(filepath.FromSlash(n.Path)), filepath.Join("_smartnote_attachments", a.ID+filepath.Ext(a.Name)))
				if e != nil {
					return e
				}
				content = strings.ReplaceAll(content, "smartnote-asset://"+a.ID, filepath.ToSlash(rel))
			}
		}
		if err = add(name, []byte(content)); err != nil {
			return err
		}
	}
	for _, a := range attachments {
		if a.Deleted {
			continue
		}
		if a.RemoteOnly {
			if !validAttachmentRemote(a) {
				return fmt.Errorf("附件图床链接无效: %s", a.Name)
			}
			continue
		}
		path, e := safePath(filepath.Join(s.data(w), "attachments"), a.ID)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if digest(b) != a.SHA256 {
			return fmt.Errorf("附件损坏: %s", a.Name)
		}
		name := "_smartnote_attachments/" + a.ID + filepath.Ext(a.Name)
		if native {
			name = "metadata/attachments/" + a.ID
		}
		if err = add(name, b); err != nil {
			return err
		}
	}
	if native {
		conversationFiles, e := assistantConversationFiles(s.data(w))
		if e != nil {
			return e
		}
		metadataFiles := append([]string{"attachments-manifest.json", "tasks-metadata.json", "relations.json", "graph-layouts.json", "graph-annotations.json", "bookmarks.json", "task-reminders.json", "archive-rules.json", "semantic-archive.json", "assistant-memory.json"}, conversationFiles...)
		for _, name := range metadataFiles {
			b, e := os.ReadFile(filepath.Join(s.data(w), name))
			if errors.Is(e, os.ErrNotExist) {
				continue
			}
			if e != nil {
				return e
			}
			if err = add("metadata/"+name, b); err != nil {
				return err
			}
		}
		b, _ := json.MarshalIndent(manifest, "", "  ")
		if err = add("smartnote-backup.json", b); err != nil {
			return err
		}
	}
	if err = zw.Close(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), dest)
}

// Import always restores into a new empty folder: existing workspaces are never overwritten.
func (s *Service) Import(source, dest string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := filepath.Abs(dest)
	if err != nil {
		return Workspace{}, err
	}
	if _, e := os.Stat(abs); !errors.Is(e, os.ErrNotExist) {
		return Workspace{}, errors.New("请选择尚不存在的新目录，导入不会覆盖现有文件")
	}
	reader, err := zip.OpenReader(source)
	if err != nil {
		return Workspace{}, err
	}
	defer reader.Close()
	if len(reader.File) > 20000 {
		return Workspace{}, errors.New("导入包超过 20000 个文件")
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return Workspace{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(abs), ".smartnote-import-*")
	if err != nil {
		return Workspace{}, err
	}
	defer os.RemoveAll(stage)
	total := uint64(0)
	seen := map[string]bool{}
	for _, entry := range reader.File {
		if entry.Mode()&os.ModeSymlink != 0 {
			return Workspace{}, errors.New("导入包包含符号链接")
		}
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		if entry.FileInfo().IsDir() {
			continue
		}
		path, e := safePath(stage, name)
		if e != nil {
			return Workspace{}, e
		}
		key := strings.ToLower(filepath.Clean(name))
		if seen[key] {
			return Workspace{}, errors.New("导入包包含重复文件")
		}
		seen[key] = true
		if entry.UncompressedSize64 > maxAttachment || entry.UncompressedSize64 > 2<<30-total {
			return Workspace{}, errors.New("导入包超过单文件 100 MB 或总计 2 GB 限制")
		}
		total += entry.UncompressedSize64
		r, e := entry.Open()
		if e != nil {
			return Workspace{}, e
		}
		b, e := io.ReadAll(io.LimitReader(r, maxAttachment+1))
		r.Close()
		if e != nil {
			return Workspace{}, e
		}
		if len(b) > maxAttachment {
			return Workspace{}, errors.New("导入文件过大")
		}
		if uint64(len(b)) != entry.UncompressedSize64 {
			return Workspace{}, errors.New("ZIP 文件大小不一致")
		}
		if err = atomicWrite(path, b); err != nil {
			return Workspace{}, err
		}
	}
	manifestPath := filepath.Join(stage, "smartnote-backup.json")
	_, statErr := os.Stat(manifestPath)
	native := statErr == nil
	w := Workspace{ID: uid(), Name: filepath.Base(abs), Path: abs}
	noteRoot := stage
	if native {
		var m backupManifest
		if err = readJSON(manifestPath, &m); err != nil {
			return Workspace{}, err
		}
		if m.Version != 1 || m.Hashes == nil {
			return Workspace{}, errors.New("不支持的备份格式")
		}
		for name, hash := range m.Hashes {
			path, e := safePath(stage, name)
			if e != nil {
				return Workspace{}, e
			}
			if !strings.HasPrefix(name, "notes/") && !strings.HasPrefix(name, "metadata/") {
				return Workspace{}, errors.New("备份包含未知路径")
			}
			b, e := os.ReadFile(path)
			if e != nil || digest(b) != hash {
				return Workspace{}, fmt.Errorf("备份校验失败: %s", name)
			}
		}
		for key := range seen {
			if key != "smartnote-backup.json" {
				found := false
				for name := range m.Hashes {
					if strings.EqualFold(filepath.Clean(name), key) {
						found = true
						break
					}
				}
				if !found {
					return Workspace{}, errors.New("存在未校验的备份文件")
				}
			}
		}
		w.Name = m.Workspace.Name
		noteRoot = filepath.Join(stage, "notes")
		if err = os.MkdirAll(noteRoot, 0700); err != nil {
			return Workspace{}, err
		}
		if err = validateMetadata(filepath.Join(stage, "metadata")); err != nil {
			return Workspace{}, err
		}
		semantic := defaultSemantic()
		semantic.AutomaticVersion = 0
		if err = readJSON(filepath.Join(stage, "metadata", "semantic-archive.json"), &semantic); err != nil {
			return Workspace{}, err
		}
		normalizeSemantic(&semantic)
		if err = validateSemantic(Workspace{Path: noteRoot}, semantic); err != nil {
			return Workspace{}, err
		}
		if _, statErr := os.Stat(filepath.Join(stage, "metadata", "semantic-archive.json")); statErr == nil {
			if err = writeJSON(filepath.Join(stage, "metadata", "semantic-archive.json"), semantic); err != nil {
				return Workspace{}, err
			}
		}
		var archiveConfig ArchiveRulesConfig
		if err = readJSON(filepath.Join(stage, "metadata", "archive-rules.json"), &archiveConfig); err != nil {
			return Workspace{}, fmt.Errorf("归档规则无效：%w", err)
		}
		if err = validateArchiveRules(Workspace{Path: noteRoot}, archiveConfig.Rules); err != nil {
			return Workspace{}, fmt.Errorf("归档规则无效：%w", err)
		}
	}
	if !native {
		// Convert ordinary local Markdown resources into the independent attachment library.
		if err = s.convertImportedAssets(stage, s.data(w)); err != nil {
			os.RemoveAll(s.data(w))
			return Workspace{}, err
		}
	}
	// Reject duplicate note identities before committing any user-visible directory.
	if _, err = s.scan(Workspace{Path: noteRoot}); err != nil {
		if !native {
			os.RemoveAll(s.data(w))
		}
		return Workspace{}, err
	}
	if native {
		meta := filepath.Join(stage, "metadata")
		if _, err = os.Stat(meta); err == nil {
			if err = os.MkdirAll(filepath.Dir(s.data(w)), 0700); err != nil {
				return Workspace{}, err
			}
			if err = os.Rename(meta, s.data(w)); err != nil {
				return Workspace{}, err
			}
		}
	}
	if err = os.Rename(noteRoot, abs); err != nil {
		os.RemoveAll(s.data(w))
		return Workspace{}, err
	}
	s.settings.Workspaces = append(s.settings.Workspaces, w)
	s.settings.ActiveID = w.ID
	if err = s.persist(); err != nil {
		return Workspace{}, err
	}
	return w, nil
}

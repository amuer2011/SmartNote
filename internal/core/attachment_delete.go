package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type AttachmentUsage struct {
	ID    string `json:"id"`
	Notes int    `json:"notes"`
	Trash int    `json:"trash"`
}
type DeleteAttachmentsInput struct {
	WorkspaceID string   `json:"workspaceId"`
	IDs         []string `json:"ids"`
	Force       bool     `json:"force"`
}
type DeleteAttachmentsResult struct {
	Deleted  []string          `json:"deleted"`
	Skipped  map[string]string `json:"skipped"`
	Warnings []string          `json:"warnings"`
}

// Count conservatively, including raw URLs, HTML images and code examples. It is
// preferable to retain an unused original rather than break a recoverable note.
func attachmentReferenced(content string, a Attachment) bool {
	return strings.Contains(content, "smartnote-asset://"+a.ID) || (a.RemoteURL != "" && strings.Contains(content, a.RemoteURL))
}
func (s *Service) attachmentUsage(w Workspace, items []Attachment) ([]AttachmentUsage, error) {
	notes, err := s.scan(w)
	if err != nil {
		return nil, err
	}
	out := []AttachmentUsage{}
	byID := map[string]Attachment{}
	for _, a := range items {
		if a.Deleted {
			continue
		}
		byID[a.ID] = a
		u := AttachmentUsage{ID: a.ID}
		for _, n := range notes {
			if attachmentReferenced(n.Content, a) {
				u.Notes++
			}
		}
		out = append(out, u)
	}
	// Recycled notes still need their originals if restored later.
	for _, root := range []string{filepath.Join(s.data(w), "trash"), filepath.Join(w.Path, ".smartnote-trash")} {
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
			if os.IsNotExist(e) && path == root {
				return nil
			}
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("回收目录包含链接，无法安全检查附件引用")
			}
			if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() || info.Size() > maxAttachment {
				return errors.New("回收笔记无法安全读取")
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			text := string(b)
			for i := range out {
				if attachmentReferenced(text, byID[out[i].ID]) {
					out[i].Trash++
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Service) AttachmentUsage(workspaceID string) ([]AttachmentUsage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	items := []Attachment{}
	if err = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &items); err != nil {
		return nil, err
	}
	return s.attachmentUsage(w, items)
}
func (s *Service) DeleteAttachments(in DeleteAttachmentsInput) (DeleteAttachmentsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := DeleteAttachmentsResult{Deleted: []string{}, Skipped: map[string]string{}, Warnings: []string{}}
	w, err := s.archiveWorkspace(in.WorkspaceID)
	if err != nil {
		return out, err
	}
	if len(in.IDs) == 0 || len(in.IDs) > 10000 {
		return out, errors.New("请选择1到10000个附件")
	}
	items := []Attachment{}
	manifest := filepath.Join(s.data(w), "attachments-manifest.json")
	if err = readJSON(manifest, &items); err != nil {
		return out, err
	}
	usage, err := s.attachmentUsage(w, items)
	if err != nil {
		return out, err
	}
	used := map[string]AttachmentUsage{}
	for _, u := range usage {
		used[u.ID] = u
	}
	requested := map[string]bool{}
	for _, id := range in.IDs {
		requested[id] = true
		out.Skipped[id] = "附件已删除或不存在"
	}
	paths := map[string]string{}
	for i, a := range items {
		if !requested[a.ID] || a.Deleted {
			continue
		}
		u := used[a.ID]
		if !in.Force && (u.Notes > 0 || u.Trash > 0) {
			out.Skipped[a.ID] = fmt.Sprintf("仍有%d篇笔记、%d篇回收笔记引用", u.Notes, u.Trash)
			continue
		}
		path, e := safePath(filepath.Join(s.data(w), "attachments"), a.ID)
		if e != nil {
			return out, e
		}
		if !a.RemoteOnly {
			info, e := os.Lstat(path)
			if e != nil && !os.IsNotExist(e) {
				return out, e
			}
			if e == nil && !info.Mode().IsRegular() {
				return out, errors.New("附件不是普通文件，无法删除")
			}
			paths[a.ID] = path
		}
		items[i].Deleted = true
		out.Deleted = append(out.Deleted, a.ID)
		delete(out.Skipped, a.ID)
	}
	if len(out.Deleted) == 0 {
		return out, nil
	}
	// Persist tombstones first. A crash or a stale second device cannot resurrect
	// the attachment identity; incomplete physical cleanup is safe to retry later.
	if err = writeJSON(manifest, items); err != nil {
		return DeleteAttachmentsResult{}, err
	}
	for _, id := range out.Deleted {
		if path := paths[id]; path != "" {
			if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
				out.Warnings = append(out.Warnings, "附件记录已删除，但本地原件清理失败："+e.Error())
			}
		}
	}
	return out, nil
}

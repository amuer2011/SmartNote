package core

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const taskArchiveDirectory = "已完成待办"

// The directory rule also protects manually created or edited archive notes.
// The marker keeps the protection when a generated note is moved elsewhere.
func isTaskArchiveNote(n Note) bool {
	p := filepath.ToSlash(filepath.Clean(n.Path))
	return n.TaskArchived || p == taskArchiveDirectory || strings.HasPrefix(p, taskArchiveDirectory+"/")
}

func archiveNoteFilename(r TaskArchiveRecord) string {
	name := strings.Map(func(c rune) rune {
		if c < 32 || strings.ContainsRune(`<>:"/\|?*`, c) {
			return '_'
		}
		return c
	}, r.Title)
	runes := []rune(strings.Trim(name, " ."))
	if len(runes) > 64 {
		runes = runes[:64]
	}
	if len(runes) == 0 {
		runes = []rune("完成任务")
	}
	// Prefix avoids reserved Windows device names; the record ID separates repeats.
	return "完成-" + string(runes) + "-" + r.ID + ".md"
}

// Create once. Refreshes never overwrite the user's edits, including a rename
// or move with the stable note ID intact. Exclusive creation protects collisions.
func (s *Service) materializeTaskArchive(w Workspace, records []TaskArchiveRecord) error {
	if len(records) == 0 {
		return nil
	}
	notes, err := s.scan(w)
	if err != nil {
		return err
	}
	byID := map[string]Note{}
	for _, n := range notes {
		byID[n.ID] = n
	}
	metadata := map[string]Task{}
	if err = readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &metadata); err != nil {
		return err
	}
	for _, r := range records {
		if source, ok := byID[r.NoteID]; ok {
			if isTaskArchiveNote(source) {
				continue
			}
			if r.ReopenedAt == "" && completedNote(source, metadata) {
				moved, e := s.moveCompletedNote(w, source, r)
				if e != nil {
					return e
				}
				byID[source.ID] = moved
				continue
			}
		}
		id := "task_archive_" + r.ID
		if _, exists := byID[id]; exists {
			continue
		}
		at, e := time.Parse(time.RFC3339Nano, r.CompletedAt)
		if e != nil {
			return e
		}
		rel := filepath.ToSlash(filepath.Join(taskArchiveDirectory, at.Format("2006"), at.Format("01"), archiveNoteFilename(r)))
		path, e := safePath(w.Path, rel)
		if e != nil {
			return e
		}
		// Never replace an existing file, even if the user removed its YAML marker.
		if _, e = os.Lstat(path); e == nil {
			continue
		} else if !os.IsNotExist(e) {
			return e
		}
		meta, e := yaml.Marshal(map[string]any{"id": id, "title": r.Title, "task_archive": true, "created": r.CompletedAt, "completed_at": r.CompletedAt, "source_note_id": r.NoteID, "archive_record_id": r.ID})
		if e != nil {
			return e
		}
		due := r.DueAt
		if due == "" {
			due = "未设置"
		}
		body := r.NoteContent
		bodyHeading := "完成时的笔记正文"
		if body == "" {
			if source, ok := byID[r.NoteID]; ok {
				body = source.Content
				bodyHeading = "来源正文（历史补录时）"
			}
		}
		if body == "" {
			body = "原记录未保存正文，来源笔记已不存在。"
		} else {
			body = archiveBody(body)
		}
		text := "---\n" + string(meta) + "---\n\n# " + strings.Join(strings.Fields(r.Title), " ") + "\n\n"
		text += fmt.Sprintf("- 完成时间：%s\n- 原截止日期：%s\n- 来源路径：%s\n", at.Format("2006-01-02 15:04:05"), due, strings.Join(strings.Fields(r.NotePath), " "))
		if source, ok := byID[r.NoteID]; ok {
			link, e := filepath.Rel(filepath.Dir(path), filepath.Join(w.Path, filepath.FromSlash(source.Path)))
			if e == nil {
				text += "- [打开原笔记](<" + (&url.URL{Path: filepath.ToSlash(link)}).String() + ">)\n"
			}
		}
		if r.Imported {
			text += "- 历史补录：无法恢复原完成时间，以补录时间归档。\n"
		}
		text += "\n> 此笔记为完成归档，可自由编辑；不参与待办识别和自动分类。撤销完成后的状态以完成归档列表为准。\n\n## " + bodyHeading + "\n\n" + body + "\n"
		if e = writeExclusive(path, []byte(text)); e != nil {
			return fmt.Errorf("待办完成记录已保存，生成归档笔记失败：%w", e)
		}
	}
	return nil
}

// Move only after every active task is done; an unlimited recurrence stays active.
func completedNote(n Note, metadata map[string]Task) bool {
	seen := false
	for _, t := range tasksFor(n) {
		if stored, ok := metadata[t.ID]; ok {
			t = mergeTask(t, stored)
		}
		if t.Deleted || t.Candidate {
			continue
		}
		seen = true
		if !t.Done || (t.Recurrence != nil && t.Recurrence.Until == "") {
			return false
		}
	}
	return seen
}

func (s *Service) moveCompletedNote(w Workspace, n Note, record TaskArchiveRecord) (Note, error) {
	at, e := time.Parse(time.RFC3339Nano, record.CompletedAt)
	if e != nil {
		return Note{}, e
	}
	target := filepath.ToSlash(filepath.Join(taskArchiveDirectory, at.Format("2006/01"), filepath.Base(n.Path)))
	path, e := safePath(w.Path, target)
	if e != nil {
		return Note{}, e
	}
	if _, e = os.Lstat(path); e == nil {
		base := strings.TrimSuffix(filepath.Base(n.Path), filepath.Ext(n.Path))
		target = filepath.ToSlash(filepath.Join(filepath.Dir(target), base+"-"+record.ID+".md"))
	} else if !os.IsNotExist(e) {
		return Note{}, e
	}
	content, e := completedNoteContent(n, record)
	if e != nil {
		return Note{}, e
	}
	// Reuse the journalled move, stable ID, hash protection and link rewrites.
	result, e := s.moveNoteLocked(n.ID, n.Hash, target, &content)
	if e != nil {
		return Note{}, fmt.Errorf("任务已完成，移动归档失败（原文仍受移动日志保护）：%w", e)
	}
	return result.Note, nil
}

func completedNoteContent(n Note, record TaskArchiveRecord) (string, error) {
	text := strings.ReplaceAll(n.Content, "\r\n", "\n")
	meta := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	body := n.Content
	if strings.HasPrefix(text, "---\n") {
		end := strings.Index(text[4:], "\n---")
		if end < 0 {
			return "", fmt.Errorf("Front Matter 未闭合")
		}
		var doc yaml.Node
		if e := yaml.Unmarshal([]byte(text[4:4+end]), &doc); e != nil {
			return "", e
		}
		if len(doc.Content) > 0 {
			meta = *doc.Content[0]
		}
		if meta.Kind != yaml.MappingNode {
			return "", fmt.Errorf("Front Matter 必须为键值结构")
		}
		body = text[4+end+4:]
	}
	for key, value := range map[string]string{"id": n.ID, "task_archive": "true", "completed_at": record.CompletedAt, "archive_record_id": record.ID, "source_path": n.Path} {
		tag := "!!str"
		if key == "task_archive" {
			tag = "!!bool"
		}
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
		found := false
		for i := 0; i+1 < len(meta.Content); i += 2 {
			if meta.Content[i].Value == key {
				meta.Content[i+1] = node
				found = true
				break
			}
		}
		if !found {
			meta.Content = append(meta.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, node)
		}
	}
	b, e := yaml.Marshal(&meta)
	if e != nil {
		return "", e
	}
	result := "---\n" + string(b) + "---" + body
	if !strings.HasPrefix(text, "---\n") {
		result = "---\n" + string(b) + "---\n" + body
	}
	if strings.Contains(n.Content, "\r\n") {
		result = strings.ReplaceAll(strings.ReplaceAll(result, "\r\n", "\n"), "\n", "\r\n")
	}
	return result, nil
}

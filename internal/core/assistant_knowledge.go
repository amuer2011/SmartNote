package core

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

//go:embed assistant_guide.md
var assistantGuide string

func assistantTruncate(text string, limit int) string {
	r := []rune(text)
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return text
}

type assistantToolArgs struct {
	Query     string `json:"query"`
	NoteID    string `json:"noteId"`
	OldText   string `json:"oldText"`
	NewText   string `json:"newText"`
	Reason    string `json:"reason"`
	Directory string `json:"directory"`
	Tag       string `json:"tag"`
	Project   string `json:"project"`
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`
	Status    string `json:"status"`
	From      string `json:"from"`
	To        string `json:"to"`
}

func assistantPage(items []map[string]any, arg assistantToolArgs) map[string]any {
	limit := arg.Limit
	if limit < 1 || limit > 40 {
		limit = 20
	}
	offset := arg.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return map[string]any{"items": items[offset:end], "total": len(items), "offset": offset, "nextOffset": end, "hasMore": end < len(items)}
}
func assistantMatches(query string, values ...string) bool {
	return strings.Contains(strings.ToLower(strings.Join(values, "\n")), strings.ToLower(strings.TrimSpace(query)))
}

// Read metadata under the active-workspace lock; no Snapshot/Graph calls here:
// those can project task archives or do an unbounded all-pairs graph scan.
func (s *Service) assistantKnowledge(w Workspace, notes []Note, name string, arg assistantToolArgs) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	active, err := s.active()
	if err != nil || active.ID != w.ID {
		return nil, errors.New("工作区已变化，请重新发送")
	}
	if len([]rune(arg.Query)) > 500 || arg.Offset < 0 || arg.Offset > 1000000 {
		return nil, errors.New("查询过长或分页范围无效")
	}
	items := []map[string]any{}
	switch name {
	case "search_conversations":
		if strings.TrimSpace(arg.Query) == "" {
			return nil, errors.New("请提供历史交流中的关键词")
		}
		files, e := assistantConversationFiles(s.data(w))
		if e != nil {
			return nil, e
		}
		type fileInfo struct {
			path     string
			size     int64
			modified time.Time
		}
		ordered := []fileInfo{}
		for _, file := range files {
			path := filepath.Join(s.data(w), filepath.FromSlash(file))
			info, e := os.Stat(path)
			if e != nil {
				return nil, e
			}
			ordered = append(ordered, fileInfo{path, info.Size(), info.ModTime()})
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].modified.After(ordered[j].modified) })
		byID := map[string]Note{}
		for _, n := range notes {
			byID[n.ID] = n
		}
		var bytesRead int64
		scanned := 0
		partial := false
		for _, file := range ordered {
			if scanned >= 200 {
				partial = true
				break
			}
			scanned++
			if bytesRead+file.size > 16<<20 {
				partial = true
				continue
			}
			bytesRead += file.size
			conversation, e := loadAssistantConversation(file.path)
			if e != nil {
				return nil, errors.New("历史交流读取失败")
			}
			for i := len(conversation.Entries) - 1; i >= 0; i-- {
				entry := conversation.Entries[i]
				if entry.Notice || !assistantMatches(arg.Query, entry.Content, conversation.Title) {
					continue
				}
				text := entry.Content
				position := strings.Index(strings.ToLower(text), strings.ToLower(strings.TrimSpace(arg.Query)))
				if position > 0 {
					prefix := []rune(strings.ToLower(text)[:position])
					start := len(prefix) - 80
					if start > 0 {
						text = "…" + string([]rune(text)[start:])
					}
				}
				excerpt := regexp.MustCompile(`\[\d+\]`).ReplaceAllString(assistantTruncate(text, 700), "（历史引用，需重新核对）")
				related := []map[string]any{}
				for _, source := range entry.Sources {
					if n, ok := byID[source.NoteID]; ok {
						related = append(related, map[string]any{"noteId": n.ID, "title": n.Title, "path": n.Path, "changedSinceConversation": n.Hash != source.Hash})
					}
					if len(related) >= 6 {
						break
					}
				}
				items = append(items, map[string]any{"conversationId": conversation.ID, "conversationTitle": conversation.Title, "updated": conversation.Updated, "role": entry.Role, "excerpt": excerpt, "relatedNotes": related})
				if len(items) >= 6 {
					break
				}
			}
			if len(items) >= 6 {
				partial = scanned < len(ordered)
				break
			}
		}
		return map[string]any{"items": items, "scannedConversations": scanned, "partial": partial, "notice": "历史交流仅供回忆，笔记和任务事实请查询最新资料；结果最多6条，最多检索最近200个对话/16MiB。"}, nil
	case "get_workspace_context":
		dirs, e := workspaceDirectories(w.Path)
		if e != nil {
			return nil, e
		}
		cfg, e := s.semanticConfig(w)
		if e != nil {
			return nil, e
		}
		categories := []map[string]any{}
		for _, c := range cfg.Categories {
			categories = append(categories, map[string]any{"name": c.Name, "description": assistantTruncate(c.Description, 300), "directory": c.Directory, "enabled": c.Enabled})
		}
		// Cap this overview; list_notes can page through the full note catalog.
		directoryListTruncated := len(dirs) > 100
		if len(dirs) > 100 {
			dirs = dirs[:100]
		}
		return map[string]any{"workspace": w.Name, "notes": len(notes), "directories": dirs, "directoryListTruncated": directoryListTruncated, "automaticArchive": cfg.Automatic, "similarityThreshold": cfg.Threshold, "categories": categories}, nil
	case "list_notes":
		sorted := append([]Note{}, notes...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Updated.After(sorted[j].Updated) })
		dir := strings.Trim(strings.ReplaceAll(arg.Directory, "\\", "/"), "/")
		for _, n := range sorted {
			if arg.Directory != "" {
				parent := filepath.ToSlash(filepath.Dir(n.Path))
				if dir != "." && parent != dir || dir == "." && parent != "." {
					continue
				}
			}
			if arg.Tag != "" && !containsModel(n.Tags, arg.Tag) || arg.Project != "" && n.Project != arg.Project || !assistantMatches(arg.Query, n.Title, n.Path, strings.Join(n.Tags, " "), n.Project) {
				continue
			}
			items = append(items, map[string]any{"noteId": n.ID, "title": n.Title, "path": n.Path, "tags": n.Tags, "project": n.Project, "updated": n.Updated, "taskArchived": n.TaskArchived})
		}
		return assistantPage(items, arg), nil
	case "list_bookmarks":
		bookmarks := []Bookmark{}
		if e := readJSON(filepath.Join(s.data(w), "bookmarks.json"), &bookmarks); e != nil {
			return nil, e
		}
		byID := map[string]Note{}
		for _, n := range notes {
			byID[n.ID] = n
		}
		for _, b := range bookmarks {
			n, ok := byID[b.NoteID]
			if !ok || !assistantMatches(arg.Query, b.Label, b.Text, n.Title) {
				continue
			}
			items = append(items, map[string]any{"noteId": n.ID, "title": n.Title, "path": n.Path, "label": b.Label, "text": assistantTruncate(b.Text, 300), "line": b.Line, "kind": b.Kind})
		}
		return assistantPage(items, arg), nil
	case "list_attachments":
		attachments := []Attachment{}
		if e := readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &attachments); e != nil {
			return nil, e
		}
		for _, a := range attachments {
			if !a.Deleted && assistantMatches(arg.Query, a.Name, a.Mime) {
				items = append(items, map[string]any{"id": a.ID, "name": a.Name, "mime": a.Mime, "size": a.Size, "created": a.Created})
			}
		}
		return assistantPage(items, arg), nil
	case "get_note_links":
		var target *Note
		for i := range notes {
			if notes[i].ID == arg.NoteID {
				target = &notes[i]
				break
			}
		}
		if target == nil {
			return nil, errors.New("笔记不存在")
		}
		linked := map[string]bool{}
		add := func(n Note, kind, direction string) {
			key := n.ID + kind + direction
			if linked[key] {
				return
			}
			linked[key] = true
			items = append(items, map[string]any{"noteId": n.ID, "title": n.Title, "path": n.Path, "type": kind, "direction": direction})
		}
		for _, ref := range markdownLinks(target.Content) {
			if n := resolveNoteLink(*target, ref, notes); n != nil {
				add(*n, "MARKDOWN_LINK", "outgoing")
			}
		}
		for _, n := range notes {
			if n.ID == target.ID {
				continue
			}
			for _, ref := range markdownLinks(n.Content) {
				if match := resolveNoteLink(n, ref, notes); match != nil && match.ID == target.ID {
					add(n, "MARKDOWN_LINK", "incoming")
					break
				}
			}
			if n.Project != "" && n.Project == target.Project {
				add(n, "SAME_PROJECT", "related")
			}
			for _, tag := range n.Tags {
				if containsModel(target.Tags, tag) {
					add(n, "SAME_TOPIC", "related")
					break
				}
			}
		}
		relations, e := s.graphRelations(w)
		if e != nil {
			return nil, e
		}
		for _, r := range relations {
			other := ""
			if r.Source == target.ID {
				other = r.Target
			} else if r.Target == target.ID {
				other = r.Source
			}
			if other != "" {
				for _, n := range notes {
					if n.ID == other {
						add(n, "USER_CONFIRMED", "related")
						break
					}
				}
			}
		}
		return assistantPage(items, arg), nil
	case "list_tasks":
		if arg.Status == "" {
			arg.Status = "pending"
		}
		if !containsModel([]string{"pending", "done", "confirmation", "all"}, arg.Status) {
			return nil, errors.New("任务状态须为 pending/done/confirmation/all")
		}
		if arg.From != "" || arg.To != "" {
			if arg.From == "" {
				arg.From = arg.To
			}
			if arg.To == "" {
				arg.To = arg.From
			}
			from, e := time.Parse("2006-01-02", arg.From)
			to, e2 := time.Parse("2006-01-02", arg.To)
			if e != nil || e2 != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
				return nil, errors.New("日期范围须为 YYYY-MM-DD，最多367天")
			}
		}
		meta := map[string]Task{}
		if e := readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &meta); e != nil {
			return nil, e
		}
		for _, n := range notes {
			for _, task := range tasksFor(n) {
				if old, ok := meta[task.ID]; ok {
					task = mergeTask(task, old)
				}
				if task.Deleted || task.IntentDecision == "rejected" || !assistantMatches(arg.Query, task.Title, n.Title) {
					continue
				}
				pendingConfirmation := needsTaskConfirmation(task)
				if task.Done || arg.Status == "done" || arg.Status == "pending" && pendingConfirmation || arg.Status == "confirmation" && !pendingConfirmation {
					continue
				}
				for _, date := range assistantTaskDates(task, arg.From, arg.To) {
					if containsModel(task.CompletedDates, date) {
						continue
					}
					items = append(items, map[string]any{"taskId": task.ID, "noteId": n.ID, "noteTitle": n.Title, "title": task.Title, "dueAt": date, "priority": task.Priority, "needsConfirmation": pendingConfirmation, "done": false, "recurrence": task.Recurrence})
				}
			}
		}
		if arg.Status == "done" || arg.Status == "all" {
			for _, h := range archiveRecords(meta) {
				date := h.DueAt
				if date == "" && len(h.CompletedAt) >= 10 {
					date = h.CompletedAt[:10]
				}
				if h.ReopenedAt != "" || !assistantMatches(arg.Query, h.Title, h.NoteTitle) || arg.From != "" && (date < arg.From || date > arg.To) {
					continue
				}
				items = append(items, map[string]any{"taskId": h.TaskID, "noteId": h.NoteID, "noteTitle": h.NoteTitle, "title": h.Title, "dueAt": date, "completedAt": h.CompletedAt, "done": true})
			}
		}
		sort.SliceStable(items, func(i, j int) bool { return fmt.Sprint(items[i]["dueAt"]) < fmt.Sprint(items[j]["dueAt"]) })
		return assistantPage(items, arg), nil
	}
	return nil, errors.New("此工具未提供")
}

func assistantTaskDates(task Task, from, to string) []string {
	if from == "" {
		return []string{task.DueAt}
	}
	if task.Recurrence == nil {
		if task.DueAt >= from && task.DueAt <= to {
			return []string{task.DueAt}
		}
		return nil
	}
	r := task.Recurrence
	if r.Interval < 1 {
		return nil
	}
	anchor, e := time.Parse("2006-01-02", r.Anchor)
	start, e2 := time.Parse("2006-01-02", from)
	if e != nil || e2 != nil {
		return nil
	}
	delta := 0
	switch r.Unit {
	case "day":
		delta = int(start.Sub(anchor).Hours() / 24)
	case "week":
		delta = int(start.Sub(anchor).Hours()/24) / 7
	case "month":
		delta = (start.Year()-anchor.Year())*12 + int(start.Month()-anchor.Month())
	case "year":
		delta = start.Year() - anchor.Year()
	}
	index := delta/r.Interval - 1
	if index < 0 {
		index = 0
	}
	out := []string{}
	for i := index; i < index+400; i++ {
		date := occurrence(r, i)
		if date == "" || date > to {
			break
		}
		if date >= from {
			out = append(out, date)
		}
	}
	return out
}

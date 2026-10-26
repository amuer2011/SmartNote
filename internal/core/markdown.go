package core

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var checkbox = regexp.MustCompile(`^\s*[-*+] \[([ xX])\]\s+(.+)$`)
var taskMarker = regexp.MustCompile(`\s*<!--\s*task:([a-zA-Z0-9_-]+)\s*-->`)
var reminderIntent = regexp.MustCompile(`(提醒我|提醒一下|记得|别忘了|不要忘记).+`)

func actionableReminder(line string) bool {
	return reminderIntent.MatchString(line) && !strings.ContainsAny(line, "？?") && !strings.Contains(line, "如果") && !strings.Contains(line, "已经") && !strings.Contains(line, "不要提醒") && !strings.Contains(line, "不用提醒") && !strings.Contains(line, "不需要提醒")
}

var datedAction = regexp.MustCompile(`(检查|充值|备份|运动|锻炼|跑步|阅读|学习|打卡|签到|维护|统计|出差|开会|会议|参加|提交|交付|办理|续费|延期|还款|缴费|预约|体检|复诊|面试|考试|出发|拜访|联系|购买|采购|处理|完成|更新|修复|整理|准备|填写|去|要)`)

var intent = regexp.MustCompile(`(明天|今天|下周|后天|需要|记得|待办|TODO|计划).*(检查|完成|整理|提交|修复|联系|准备|更新|处理|确认|实现)`)

func parseNote(path string, b []byte, updated time.Time) Note {
	content := string(b)
	body := strings.ReplaceAll(content, "\r\n", "\n")
	meta := map[string]any{}
	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---"); end >= 0 {
			_ = yaml.Unmarshal([]byte(body[4:4+end]), &meta)
			body = body[4+end+4:]
		}
	}
	n := Note{Path: filepath.ToSlash(path), Content: content, Hash: digest(b), Updated: updated, Tags: []string{}}
	n.TaskArchived, _ = meta["task_archive"].(bool)
	n.ID, _ = meta["id"].(string)
	if n.ID == "" {
		n.SyntheticID = true
		n.ID = "path_" + digest([]byte(n.Path))[:24]
	}
	n.Title, _ = meta["title"].(string)
	if n.Title == "" {
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "# ") {
				n.Title = strings.TrimSpace(line[2:])
				break
			}
		}
	}
	if n.Title == "" {
		n.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	n.Project, _ = meta["project"].(string)
	switch created := meta["created"].(type) {
	case time.Time:
		n.Created = created
	case string:
		n.Created, _ = time.Parse(time.RFC3339, created)
	}
	if tags, ok := meta["tags"].([]any); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				n.Tags = append(n.Tags, s)
			}
		}
	}
	if tags, ok := meta["tags"].(string); ok {
		n.Tags = strings.Fields(tags)
	}
	return n
}
func taskRequest(line string) bool {
	return (actionableReminder(line) || intent.MatchString(line) || (repeatExpression.MatchString(line) && datedAction.MatchString(line)) || ((relativeOffset.MatchString(line) || monthDayExpression.MatchString(line) || calendarExpression.MatchString(line) || dateExpression.MatchString(line) || weekdayExpression.MatchString(line) || strings.Contains(line, "今天") || strings.Contains(line, "明天") || strings.Contains(line, "后天")) && datedAction.MatchString(line))) && !strings.ContainsAny(line, "？?") && !strings.Contains(line, "如果") && !strings.Contains(line, "已经") && !strings.Contains(line, "曾经") && !strings.Contains(line, "去年") && !strings.Contains(line, "取消") && !strings.Contains(line, "无需") && !strings.Contains(line, "不需要") && !strings.Contains(line, "不用") && !strings.Contains(line, "不要提醒") && !strings.HasPrefix(strings.TrimSpace(line), ">")
}
func tasksFor(n Note) []Task {
	tasks := []Task{}
	if isTaskArchiveNote(n) {
		return tasks
	}
	fence := byte(0)
	fenceLength := 0
	front := strings.HasPrefix(n.Content, "---\n") || strings.HasPrefix(n.Content, "---\r\n")
	counts := map[string]int{}
	for i, line := range strings.Split(strings.ReplaceAll(n.Content, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if front {
			if i > 0 && trimmed == "---" {
				front = false
			}
			continue
		}
		if len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			count := 0
			for count < len(trimmed) && trimmed[count] == trimmed[0] {
				count++
			}
			if count >= 3 {
				if fence == 0 {
					fence = trimmed[0]
					fenceLength = count
				} else if fence == trimmed[0] && count >= fenceLength && strings.TrimSpace(trimmed[count:]) == "" {
					fence = 0
				}
				continue
			}
		}
		if fence != 0 {
			continue
		}
		if m := checkbox.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(taskMarker.ReplaceAllString(m[2], ""))
			counts[title]++
			id := fmt.Sprintf("%s_%s_%d", n.ID, digest([]byte(title))[:12], counts[title])
			if mark := taskMarker.FindStringSubmatch(line); mark != nil {
				id = mark[1]
			}
			tasks = append(tasks, Task{ID: id, NoteID: n.ID, NoteTitle: n.Title, Title: title, Line: i, Done: m[1] != " "})
		} else if !strings.HasPrefix(trimmed, "#") && taskRequest(line) && !strings.ContainsAny(line, "？?") && !strings.Contains(line, "如果") && !strings.Contains(line, "已经") {
			title := strings.TrimSpace(line)
			counts[title]++
			tasks = append(tasks, Task{ID: fmt.Sprintf("%s_candidate_%s_%d", n.ID, digest([]byte(title))[:12], counts[title]), NoteID: n.ID, NoteTitle: n.Title, Title: title, Line: i, Candidate: true})
		}
	}
	// Keep one stable note-level reminder, separate from Markdown checkbox tasks.
	reminderLine, reminderText := -1, ""
	if taskRequest(n.Title) {
		reminderLine, reminderText = 0, n.Title
	}
	for _, task := range tasks {
		if task.Candidate && taskRequest(task.Title) {
			reminderLine, reminderText = task.Line, task.Title
			break
		}
	}
	if reminderLine >= 0 {
		filtered := tasks[:0]
		done := false
		for _, task := range tasks {
			if !task.Candidate && task.Title == n.Title && taskRequest(task.Title) {
				done = task.Done
				continue
			}
			if !task.Candidate {
				filtered = append(filtered, task)
			}
		}
		tasks = append(filtered, Task{ID: n.ID + "_intent", NoteID: n.ID, NoteTitle: n.Title, Title: n.Title, Line: reminderLine, Inferred: true, IntentDecision: "pending", SourceRevision: noteTaskRevision(n), Done: done, Recurrence: inferRecurrence(reminderText, n), DueAt: inferDue(reminderText, n)})
	}
	for i := range tasks {
		if tasks[i].Recurrence != nil {
			tasks[i].DueAt = tasks[i].Recurrence.Anchor
			tasks[i].Done = false
		}
		tasks[i].Priority = "normal"
		tasks[i].Tags = append([]string{}, n.Tags...)
		tasks[i].Project = n.Project
		if tasks[i].DueAt == "" {
			tasks[i].DueAt = inferDue(tasks[i].Title, n)
		}
	}
	return tasks
}

// explicitTitle ignores headings and incomplete YAML while the user is typing.
func explicitTitle(content string) string {
	text := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return ""
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return ""
	}
	meta := map[string]any{}
	if yaml.Unmarshal([]byte(text[4:4+end]), &meta) != nil {
		return ""
	}
	title, _ := meta["title"].(string)
	return strings.TrimSpace(title)
}

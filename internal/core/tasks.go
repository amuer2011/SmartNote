package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var calendarExpression = regexp.MustCompile(`(\d{4})[年/](\d{1,2})[月/](\d{1,2})日?`)
var monthDayExpression = regexp.MustCompile(`(?:^|[^0-9])(\d{1,2})月(\d{1,2})[日号]`)
var weekdayExpression = regexp.MustCompile(`(下下周|下下星期|下周|下星期|本周|这周|本星期|这星期|周|星期)([一二三四五六日天])`)
var dateExpression = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)

func inferDue(title string, n Note) string {
	if match := dateExpression.FindStringSubmatch(title); match != nil {
		if _, err := time.Parse("2006-01-02", match[1]); err == nil {
			return match[1]
		}
	}
	if match := calendarExpression.FindStringSubmatch(title); match != nil {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		value := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		if _, err := time.Parse("2006-01-02", value); err == nil {
			return value
		}
	}
	base := n.Created
	if base.IsZero() {
		base = n.Updated
	}
	if base.IsZero() {
		return ""
	}
	if m := relativeOffset.FindStringSubmatch(title); m != nil {
		amount, _ := strconv.Atoi(m[1])
		if amount > 0 && amount <= 100000 {
			switch m[2] {
			case "天", "日":
				return base.AddDate(0, 0, amount).Format("2006-01-02")
			case "周", "星期":
				return base.AddDate(0, 0, amount*7).Format("2006-01-02")
			case "月", "个月":
				return clampDate(base.Year(), base.Month()+time.Month(amount), base.Day(), base.Location()).Format("2006-01-02")
			case "年":
				return clampDate(base.Year()+amount, base.Month(), base.Day(), base.Location()).Format("2006-01-02")
			}
		}
	}
	if match := monthDayExpression.FindStringSubmatch(title); match != nil {
		month, _ := strconv.Atoi(match[1])
		day, _ := strconv.Atoi(match[2])
		value := fmt.Sprintf("%04d-%02d-%02d", base.Year(), month, day)
		if _, err := time.Parse("2006-01-02", value); err == nil {
			return value
		}
	}
	for _, relative := range []struct {
		Word string
		Days int
	}{{"后天", 2}, {"明天", 1}, {"今天", 0}} {
		if strings.Contains(title, relative.Word) {
			return base.AddDate(0, 0, relative.Days).Format("2006-01-02")
		}
	}
	if match := weekdayExpression.FindStringSubmatch(title); match != nil {
		weekdays := map[string]int{"一": 1, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "日": 7, "天": 7}
		day := int(base.Weekday())
		if day == 0 {
			day = 7
		}
		offset := 0
		if strings.HasPrefix(match[1], "下下") {
			offset = 14
		} else if strings.HasPrefix(match[1], "下") {
			offset = 7
		}
		return base.AddDate(0, 0, weekdays[match[2]]-day+offset).Format("2006-01-02")
	}
	return ""
}
func mergeTask(t Task, stored Task) Task {
	t.History = stored.History
	t.Deleted = stored.Deleted
	if t.Inferred && stored.IgnoreNoteAutomation {
		t.IgnoreNoteAutomation = true
		t.IntentDecision = "rejected"
		return t
	}
	if t.Inferred && stored.SourceRevision != "" && t.SourceRevision != stored.SourceRevision {
		return t
	}
	if t.Inferred {
		t.IntentDecision = stored.IntentDecision
		if t.IntentDecision == "" {
			t.IntentDecision = "accepted"
		} // Existing metadata predates manual confirmation.
	}
	if t.Recurrence != nil {
		if stored.Recurrence != nil && t.Recurrence.Unit == stored.Recurrence.Unit && t.Recurrence.Interval == stored.Recurrence.Interval && (t.SourceRevision == stored.SourceRevision || t.Recurrence.Source == stored.Recurrence.Source) {
			t.Recurrence = stored.Recurrence
			t.CompletedDates = stored.CompletedDates
		} else {
			return t
		}
	}
	if t.Candidate || t.Inferred {
		t.Candidate = stored.Candidate
		t.Done = stored.Done
	}
	if stored.Priority != "" {
		t.Priority = stored.Priority
	}
	t.DueAt = stored.DueAt
	return t
}
func (s *Service) UpdateTaskDetails(input TaskDetails) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	if input.Priority != "low" && input.Priority != "normal" && input.Priority != "high" {
		return errors.New("优先级必须为 low、normal 或 high")
	}
	if input.DueAt != "" {
		if _, err = time.Parse("2006-01-02", input.DueAt); err != nil {
			return errors.New("截止日期格式应为 YYYY-MM-DD")
		}
	}
	notes, err := s.scan(w)
	if err != nil {
		return err
	}
	meta := map[string]Task{}
	p := filepath.Join(s.data(w), "tasks-metadata.json")
	if err = readJSON(p, &meta); err != nil {
		return err
	}
	for _, n := range notes {
		for _, t := range tasksFor(n) {
			if t.ID != input.ID {
				continue
			}
			if stored, ok := meta[t.ID]; ok {
				t = mergeTask(t, stored)
			}
			if t.Candidate {
				return errors.New("请先确认候选任务")
			}
			t.Priority = input.Priority
			previousDue := t.DueAt
			t.DueAt = input.DueAt
			if t.Recurrence != nil && input.DueAt != previousDue {
				if input.DueAt == "" {
					return errors.New("循环待办必须设置开始日期")
				}
				if t.Recurrence.Until != "" && input.DueAt > t.Recurrence.Until {
					return errors.New("开始日期不能晚于循环截止日期")
				}
				t.Recurrence = &Recurrence{Unit: t.Recurrence.Unit, Interval: t.Recurrence.Interval, Anchor: input.DueAt, Day: mustDateDay(input.DueAt), Source: t.Recurrence.Source, Until: t.Recurrence.Until}
				t.CompletedDates = nil
				t.Done = false
			}
			lines := strings.Split(strings.ReplaceAll(n.Content, "\r\n", "\n"), "\n")
			if checkbox.MatchString(lines[t.Line]) && !taskMarker.MatchString(lines[t.Line]) {
				lines[t.Line] += " <!-- task:" + t.ID + " -->"
				text := strings.Join(lines, "\n")
				if strings.Contains(n.Content, "\r\n") {
					text = strings.ReplaceAll(text, "\n", "\r\n")
				}
				if _, err = s.save(w, SaveInput{Path: n.Path, Hash: n.Hash, Content: text}); err != nil {
					return err
				}
			}
			meta[t.ID] = t
			return writeJSON(p, meta)
		}
	}
	return errors.New("任务不存在或已变化，请刷新")
}

func mustDateDay(value string) int { d, _ := time.Parse("2006-01-02", value); return d.Day() }

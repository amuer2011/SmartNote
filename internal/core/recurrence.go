package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

type Recurrence struct {
	Until    string `json:"until,omitempty"`
	Day      int    `json:"day"`
	Source   string `json:"source"`
	Unit     string `json:"unit"`
	Interval int    `json:"interval"`
	Anchor   string `json:"anchor"`
}

var relativeOffset = regexp.MustCompile(`([0-9]+)\s*(天|日|周|星期|个月|月|年)(?:以)?后`)
var repeatExpression = regexp.MustCompile(`每(?:隔)?([0-9一二两三四五六七八九十百]+)?(天|日|周|星期|个月|月|年)`)
var repeatWeekday = regexp.MustCompile(`每(?:隔)?(?:[0-9一二两三四五六七八九十百]+)?(?:周|星期)([一二三四五六日天])`)
var repeatMonthDay = regexp.MustCompile(`每(?:隔)?(?:[0-9一二两三四五六七八九十百]+)?(?:个月|月)([0-9]{1,2})[日号]`)

func repeatCount(v string) int {
	if v == "" {
		return 1
	}
	if n, e := strconv.Atoi(v); e == nil {
		return n
	}
	digits := map[rune]int{'一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, current := 0, 0
	for _, r := range v {
		if r == '十' || r == '百' {
			unit := 10
			if r == '百' {
				unit = 100
			}
			if current == 0 {
				current = 1
			}
			total += current * unit
			current = 0
		} else {
			current = digits[r]
		}
	}
	return total + current
}
func noteBase(n Note) time.Time {
	if !n.Created.IsZero() {
		return n.Created
	}
	return n.Updated
}
func clampDate(year int, month time.Month, day int, loc *time.Location) time.Time {
	last := time.Date(year, month+1, 0, 12, 0, 0, 0, loc).Day()
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 12, 0, 0, 0, loc)
}
func inferRecurrence(text string, n Note) *Recurrence {
	m := repeatExpression.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	count := repeatCount(m[1])
	if count < 1 || count > 10000 {
		return nil
	}
	unit := map[string]string{"天": "day", "日": "day", "周": "week", "星期": "week", "月": "month", "个月": "month", "年": "year"}[m[2]]
	base := noteBase(n)
	if base.IsZero() {
		return nil
	}
	anchor := base
	desiredDay := base.Day()
	if unit == "week" {
		if w := repeatWeekday.FindStringSubmatch(text); w != nil {
			week := map[string]int{"一": 1, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "日": 7, "天": 7}
			day := int(base.Weekday())
			if day == 0 {
				day = 7
			}
			delta := (week[w[1]] - day + 7) % 7
			anchor = base.AddDate(0, 0, delta)
		}
	}
	if unit == "month" {
		if d := repeatMonthDay.FindStringSubmatch(text); d != nil {
			day, _ := strconv.Atoi(d[1])
			desiredDay = day
			if day < 1 || day > 31 {
				return nil
			}
			anchor = clampDate(base.Year(), base.Month(), day, base.Location())
			if anchor.Format("2006-01-02") < base.Format("2006-01-02") {
				anchor = clampDate(base.Year(), base.Month()+1, day, base.Location())
			}
		}
	}
	if unit == "year" {
		if d := monthDayExpression.FindStringSubmatch(text); d != nil {
			month, _ := strconv.Atoi(d[1])
			day, _ := strconv.Atoi(d[2])
			desiredDay = day
			if month < 1 || month > 12 || day < 1 || day > 31 {
				return nil
			}
			anchor = clampDate(base.Year(), time.Month(month), day, base.Location())
			if anchor.Before(time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location())) {
				anchor = clampDate(base.Year()+1, time.Month(month), day, base.Location())
			}
		}
	}
	// Explicit starting date, or a relative offset, can postpone the first occurrence.
	if relativeOffset.MatchString(text) || dateExpression.MatchString(text) || calendarExpression.MatchString(text) {
		if due := inferDue(text, n); due != "" {
			if d, e := time.Parse("2006-01-02", due); e == nil {
				anchor = d
				desiredDay = d.Day()
			}
		}
	}
	source := fmt.Sprintf("%s|%d|%s|%d", unit, count, anchor.Format("2006-01-02"), desiredDay)
	return &Recurrence{Unit: unit, Interval: count, Anchor: anchor.Format("2006-01-02"), Day: desiredDay, Source: source}
}
func occurrence(r *Recurrence, index int) string {
	if r == nil || index < 0 || r.Interval < 1 {
		return ""
	}
	a, e := time.Parse("2006-01-02", r.Anchor)
	if e != nil {
		return ""
	}
	step := index * r.Interval
	day := a.Day()
	if r.Day > 0 {
		day = r.Day
	}
	switch r.Unit {
	case "day":
		a = a.AddDate(0, 0, step)
	case "week":
		a = a.AddDate(0, 0, step*7)
	case "month":
		a = clampDate(a.Year(), a.Month()+time.Month(step), day, time.UTC)
	case "year":
		a = clampDate(a.Year()+step, a.Month(), day, time.UTC)
	default:
		return ""
	}
	if a.Year() > 9999 {
		return ""
	}
	date := a.Format("2006-01-02")
	if r.Until != "" && date > r.Until {
		return ""
	}
	return date
}
func completeOccurrence(t Task, date string, done bool) (Task, error) {
	if t.Recurrence == nil {
		return t, errors.New("不是循环待办")
	}
	valid := false
	for i := 0; i < 200000; i++ {
		d := occurrence(t.Recurrence, i)
		if d == date {
			valid = true
			break
		}
		if d == "" || d > date {
			break
		}
	}
	if !valid {
		return t, errors.New("日期不属于该循环计划")
	}
	completed := map[string]bool{}
	for _, d := range t.CompletedDates {
		completed[d] = true
	}
	if done {
		completed[date] = true
	} else {
		delete(completed, date)
	}
	t.CompletedDates = []string{}
	for d := range completed {
		t.CompletedDates = append(t.CompletedDates, d)
	}
	sort.Strings(t.CompletedDates)
	for i := 0; i < 200000; i++ {
		d := occurrence(t.Recurrence, i)
		if d == "" {
			if t.Recurrence.Until != "" {
				t.Done = true
				t.DueAt = date
				return t, nil
			}
			return t, errors.New("循环日期超出支持范围")
		}
		if !completed[d] {
			t.DueAt = d
			t.Done = false
			return t, nil
		}
	}
	return t, errors.New("循环次数超出支持范围")
}
func (s *Service) SetTaskOccurrence(workspaceID, id, date string, done bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.active()
	if e != nil {
		return e
	}
	if w.ID != workspaceID {
		return errors.New("工作区已切换")
	}
	notes, e := s.scan(w)
	if e != nil {
		return e
	}
	meta := map[string]Task{}
	path := filepath.Join(s.data(w), "tasks-metadata.json")
	if e = readJSON(path, &meta); e != nil {
		return e
	}
	for _, n := range notes {
		for _, t := range tasksFor(n) {
			if t.ID != id {
				continue
			}
			if old, ok := meta[id]; ok {
				t = mergeTask(t, old)
			}
			if t.Deleted {
				return errors.New("循环待办已移除")
			}
			t, e = completeOccurrence(t, date, done)
			if e != nil {
				return e
			}
			if t.Inferred {
				t.IntentDecision = "accepted"
			}
			t = taskHistory(t, date, done, n, false)
			meta[id] = t
			return s.commitTaskMetadata(w, meta)
		}
	}
	return errors.New("循环待办不存在")
}
func (s *Service) SetRecurringEnabled(workspaceID string, ids []string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.active()
	if e != nil {
		return e
	}
	if w.ID != workspaceID {
		return errors.New("工作区已切换")
	}
	if len(ids) == 0 || len(ids) > 1000 {
		return errors.New("请选择 1–1000 项循环待办")
	}
	notes, e := s.scan(w)
	if e != nil {
		return e
	}
	meta := map[string]Task{}
	path := filepath.Join(s.data(w), "tasks-metadata.json")
	if e = readJSON(path, &meta); e != nil {
		return e
	}
	all := map[string]Task{}
	for _, n := range notes {
		for _, t := range tasksFor(n) {
			if t.Recurrence != nil {
				if old, ok := meta[t.ID]; ok {
					t = mergeTask(t, old)
				}
				all[t.ID] = t
			}
		}
	}
	for _, id := range ids {
		t, ok := all[id]
		if !ok {
			return fmt.Errorf("循环待办已变化：%s", id)
		}
		t.Deleted = !enabled
		meta[id] = t
	}
	return s.commitTaskMetadata(w, meta)
}

// Reminder receipts belong to occurrences rather than to the entire series.
func reminderOccurrence(t Task, last string) (Task, bool) {
	if t.Recurrence == nil {
		return t, true
	}
	completed := map[string]bool{}
	for _, d := range t.CompletedDates {
		completed[d] = true
	}
	latest := ""
	for i := 0; i < 200000; i++ {
		d := occurrence(t.Recurrence, i)
		if d == "" || d > last {
			break
		}
		if !completed[d] {
			latest = d
		}
	}
	if latest == "" {
		return t, false
	}
	t.DueAt = latest
	t.Done = false
	return t, true
}
func openOccurrence(t Task, date string) bool {
	if t.Recurrence == nil {
		return t.DueAt == date
	}
	for _, d := range t.CompletedDates {
		if d == date {
			return false
		}
	}
	for i := 0; i < 200000; i++ {
		d := occurrence(t.Recurrence, i)
		if d == date {
			return true
		}
		if d == "" || d > date {
			break
		}
	}
	return false
}

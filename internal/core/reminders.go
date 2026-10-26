package core

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ReminderConfig struct {
	Revision string `json:"revision"`
	Enabled  bool   `json:"enabled"`
	Time     string `json:"time"`
	LeadDays int    `json:"leadDays"`
}
type reminderReceipt struct {
	Day   string `json:"day"`
	Until string `json:"until"`
}
type reminderStore struct {
	Config   ReminderConfig             `json:"config"`
	Receipts map[string]reminderReceipt `json:"receipts"`
}
type ReminderAction struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      string `json:"taskId"`
	DueAt       string `json:"dueAt"`
	Action      string `json:"action"`
	Minutes     int    `json:"minutes"`
}

func defaultReminders() reminderStore {
	return reminderStore{Config: ReminderConfig{Enabled: true, Time: "09:00"}, Receipts: map[string]reminderReceipt{}}
}
func validateReminderConfig(c ReminderConfig) error {
	if len(c.Time) != 5 {
		return errors.New("提醒时间格式应为 HH:mm")
	}
	if _, err := time.Parse("15:04", c.Time); err != nil {
		return errors.New("提醒时间格式应为 HH:mm")
	}
	if c.LeadDays < 0 || c.LeadDays > 7 {
		return errors.New("提前提醒天数范围为 0–7")
	}
	return nil
}
func loadReminders(path string) (reminderStore, error) {
	data := defaultReminders()
	if err := readJSON(path, &data); err != nil {
		return data, err
	}
	if data.Receipts == nil {
		data.Receipts = map[string]reminderReceipt{}
	}
	return data, validateReminderConfig(data.Config)
}
func (s *Service) GetReminderConfig(workspaceID string) (ReminderConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return ReminderConfig{}, err
	}
	data, err := loadReminders(filepath.Join(s.data(w), "task-reminders.json"))
	return data.Config, err
}
func (s *Service) SaveReminderConfig(workspaceID string, c ReminderConfig) (ReminderConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return c, err
	}
	if err = validateReminderConfig(c); err != nil {
		return c, err
	}
	path := filepath.Join(s.data(w), "task-reminders.json")
	data, err := loadReminders(path)
	if err != nil {
		return c, err
	}
	if c.Revision != data.Config.Revision {
		return c, errors.New("提醒配置已变化，请重新打开设置后修改")
	}
	c.Revision = uid()
	data.Config = c
	return c, writeJSON(path, data)
}
func reminderKey(task Task) string { return digest([]byte(task.ID + "\x00" + task.DueAt)) }
func dueReminders(tasks []Task, data reminderStore, now time.Time) []Task {
	out := []Task{}
	if !data.Config.Enabled {
		return out
	}
	today := now.Format("2006-01-02")
	last := now.AddDate(0, 0, data.Config.LeadDays).Format("2006-01-02")
	for _, task := range tasks {
		var available bool
		task, available = reminderOccurrence(task, last)
		if !available {
			continue
		}
		if task.Done || task.Candidate || task.DueAt == "" || task.DueAt > last {
			continue
		}
		if _, err := time.Parse("2006-01-02", task.DueAt); err != nil {
			continue
		}
		receipt := data.Receipts[reminderKey(task)]
		if receipt.Day == today {
			continue
		}
		until, untilErr := time.Parse(time.RFC3339, receipt.Until)
		if untilErr == nil && now.Before(until) {
			continue
		}
		// A snoozed task crossing midnight still returns at its chosen time.
		expiredSnoozeToday := untilErr == nil && until.In(now.Location()).Format("2006-01-02") == today
		if now.Format("15:04") < data.Config.Time && !expiredSnoozeToday {
			continue
		}
		out = append(out, task)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DueAt < out[j].DueAt })
	return out
}
func (s *Service) DueReminders(workspaceID string) ([]Task, error) {
	snapshot, err := s.Snapshot()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return nil, err
	}
	if w.ID != workspaceID || snapshot.Settings.ActiveID != workspaceID {
		return nil, errors.New("工作区已切换，请重试")
	}
	data, err := loadReminders(filepath.Join(s.data(w), "task-reminders.json"))
	if err != nil {
		return nil, err
	}
	return dueReminders(snapshot.Tasks, data, time.Now()), nil
}
func (s *Service) ActOnReminder(input ReminderAction) error {
	if input.Action != "dismiss" && input.Action != "snooze" {
		return errors.New("未知提醒操作")
	}
	if input.Action == "snooze" && (input.Minutes < 1 || input.Minutes > 1440) {
		return errors.New("稍后提醒范围为 1–1440 分钟")
	}
	snapshot, err := s.Snapshot()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	if w.ID != input.WorkspaceID || snapshot.Settings.ActiveID != w.ID {
		return errors.New("工作区已切换，请重试")
	}
	var target *Task
	for i := range snapshot.Tasks {
		task := &snapshot.Tasks[i]
		if task.ID == input.TaskID && !task.Done && !task.Candidate && openOccurrence(*task, input.DueAt) && input.DueAt != "" {
			copy := *task
			copy.DueAt = input.DueAt
			target = &copy
			break
		}
	}
	if target == nil {
		return errors.New("任务或截止日期已变化，请刷新提醒")
	}
	path := filepath.Join(s.data(w), "task-reminders.json")
	data, err := loadReminders(path)
	if err != nil {
		return err
	}
	now := time.Now()
	receipt := reminderReceipt{}
	if input.Action == "dismiss" {
		receipt.Day = now.Format("2006-01-02")
	} else {
		receipt.Until = now.Add(time.Duration(input.Minutes) * time.Minute).Format(time.RFC3339)
	}
	// Remove receipts for deleted/completed tasks or changed deadlines.
	valid := map[string]bool{}
	for _, task := range snapshot.Tasks {
		if !task.Done && !task.Candidate && strings.TrimSpace(task.DueAt) != "" {
			if current, ok := reminderOccurrence(task, now.AddDate(0, 0, data.Config.LeadDays).Format("2006-01-02")); ok {
				valid[reminderKey(current)] = true
			}
		}
	}
	for key := range data.Receipts {
		if !valid[key] {
			delete(data.Receipts, key)
		}
	}
	data.Receipts[reminderKey(*target)] = receipt
	return writeJSON(path, data)
}

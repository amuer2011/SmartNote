package core

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateMetadata(root string) error {
	if _, err := loadGraphAnnotations(root); err != nil {
		return err
	}
	if err := validateAssistantMetadata(root); err != nil {
		return fmt.Errorf("知识库助手记录无效: %w", err)
	}
	items := []Attachment{}
	if err := readJSON(filepath.Join(root, "attachments-manifest.json"), &items); err != nil {
		return fmt.Errorf("附件 Manifest 无效: %w", err)
	}
	ids := map[string]bool{}
	for _, a := range items {
		if a.ID == "" || ids[a.ID] {
			return fmt.Errorf("附件 ID 缺失或重复: %s", a.ID)
		}
		ids[a.ID] = true
		path, err := safePath(filepath.Join(root, "attachments"), a.ID)
		if err != nil {
			return err
		}
		if a.Deleted {
			continue
		}
		if a.RemoteOnly {
			if !validAttachmentRemote(a) {
				return fmt.Errorf("附件图床链接无效: %s", a.Name)
			}
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if digest(b) != a.SHA256 || int64(len(b)) != a.Size {
			return fmt.Errorf("附件 Manifest 校验失败: %s", a.Name)
		}
	}
	if _, err := loadReminders(filepath.Join(root, "task-reminders.json")); err != nil {
		return fmt.Errorf("任务提醒配置无效: %w", err)
	}
	tasks := map[string]Task{}
	if err := readJSON(filepath.Join(root, "tasks-metadata.json"), &tasks); err != nil {
		return fmt.Errorf("任务元数据无效: %w", err)
	}
	layouts := GraphLayouts{}
	if err := readJSON(filepath.Join(root, "graph-layouts.json"), &layouts); err != nil {
		return fmt.Errorf("拓扑布局无效: %w", err)
	}
	if err := validateGraphLayouts(layouts); err != nil {
		return err
	}
	relations := []Relation{}
	if err := readJSON(filepath.Join(root, "relations.json"), &relations); err != nil {
		return fmt.Errorf("人工关联无效: %w", err)
	}
	if err := validateRelations(relations); err != nil {
		return err
	}
	return nil
}

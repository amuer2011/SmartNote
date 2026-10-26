package core

import "errors"

func (s *Service) SetUIPreferences(ui UIPreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ui.FontSize < 12 || ui.FontSize > 22 {
		return errors.New("界面字号范围为 12–22")
	}
	if ui.FontFamily == "" {
		ui.FontFamily = "system"
	}
	if ui.EditorFontFamily == "" {
		ui.EditorFontFamily = ui.FontFamily
	}
	if ui.EditorFontSize == 0 {
		ui.EditorFontSize = 14
	}
	if ui.EditorFontSize < 10 || ui.EditorFontSize > 48 {
		return errors.New("编辑器字号范围为 10–48")
	}
	if !supportedFont(ui.FontFamily) || !supportedFont(ui.EditorFontFamily) {
		return errors.New("不支持的字体")
	}
	previous := s.settings.UI
	if err := normalizeLayout(&ui.Layout); err != nil {
		return err
	}
	s.settings.UI = ui
	if err := s.persist(); err != nil {
		s.settings.UI = previous
		return err
	}
	return nil
}

func normalizeLayout(layout *LayoutPreferences) error {
	for _, tabs := range [][]string{layout.RightTabOrder, layout.HiddenRightTabs} {
		seen := map[string]bool{}
		for _, id := range tabs {
			if (id != "outline" && id != "related" && id != "ai") || seen[id] {
				return errors.New("无效右侧页签设置")
			}
			seen[id] = true
		}
	}
	if layout.SidebarWidth == 0 {
		layout.SidebarWidth = 244
	}
	if layout.ListWidth == 0 {
		layout.ListWidth = 292
	}
	if layout.InspectorWidth == 0 {
		layout.InspectorWidth = 340
	}
	if layout.SplitRatio == 0 {
		layout.SplitRatio = 50
	}
	if layout.SidebarWidth < 180 || layout.SidebarWidth > 480 || layout.ListWidth < 160 || layout.ListWidth > 600 || layout.InspectorWidth < 120 || layout.SplitRatio < 15 || layout.SplitRatio > 85 {
		return errors.New("面板尺寸超出可调整范围")
	}
	for _, panel := range layout.HiddenPanels {
		switch panel {
		case "sidebar", "list", "editor", "preview", "inspector":
		default:
			return errors.New("未知面板")
		}
	}
	return nil
}

// Listing an inactive workspace never changes the active workspace or editor.
func (s *Service) ListWorkspaceFiles(id string) (WorkspaceFiles, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.settings.Workspaces {
		if w.ID != id {
			continue
		}
		notes, err := s.scan(w)
		if err != nil {
			return WorkspaceFiles{}, err
		}
		dirs, err := workspaceDirectories(w.Path)
		return WorkspaceFiles{Notes: notes, Directories: dirs}, err
	}
	return WorkspaceFiles{}, errors.New("工作区不存在")
}

func supportedFont(family string) bool {
	switch family {
	case "system", "SimSun", "NSimSun", "FangSong", "SimHei", "KaiTi", "Microsoft YaHei", "DengXian", "STSong", "STFangsong", "STKaiti", "STXihei", "STZhongsong", "FZShuTi", "FZYaoti", "Source Han Sans SC", "Source Han Serif SC", "Noto Sans CJK SC", "Noto Serif CJK SC", "Arial", "Times New Roman", "Consolas":
	default:
		return false
	}
	return true
}

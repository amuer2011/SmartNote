package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ArchiveRule struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled"`
	Field     string   `json:"field"`
	Match     string   `json:"match"`
	Keywords  []string `json:"keywords"`
	Directory string   `json:"directory"`
	Tags      []string `json:"tags"`
}
type ArchiveRulesConfig struct {
	Revision string        `json:"revision"`
	Rules    []ArchiveRule `json:"rules"`
}
type ArchiveSuggestion struct {
	Score       float64  `json:"score"`
	WorkspaceID string   `json:"workspaceId"`
	NoteID      string   `json:"noteId"`
	Hash        string   `json:"hash"`
	Directory   string   `json:"directory"`
	Tags        []string `json:"tags"`
	Reasons     []string `json:"reasons"`
	Source      string   `json:"source"`
	RuleID      string   `json:"ruleId"`
}
type ArchiveInput struct {
	WorkspaceID string   `json:"workspaceId"`
	NoteID      string   `json:"noteId"`
	Hash        string   `json:"hash"`
	SourcePath  string   `json:"sourcePath,omitempty"`
	Directory   string   `json:"directory"`
	Tags        []string `json:"tags"`
}
type ArchiveResult struct {
	Note         Note   `json:"note"`
	UpdatedLinks int    `json:"updatedLinks"`
	RecoveryPath string `json:"recoveryPath"`
	OperationID  string `json:"operationId"`
}
type archiveOperation struct {
	NoteID     string   `json:"noteId"`
	BeforePath string   `json:"beforePath"`
	BeforeTags []string `json:"beforeTags"`
	AfterHash  string   `json:"afterHash"`
	AfterPath  string   `json:"afterPath"`
	State      string   `json:"state"`
}

// Archive actions always name their workspace; a stale window must not affect another workspace.
func (s *Service) archiveWorkspace(id string) (Workspace, error) {
	w, err := s.active()
	if err != nil {
		return w, err
	}
	if w.ID != id {
		return w, errors.New("工作区已切换，请重新打开整理窗口")
	}
	return w, nil
}
func archiveDirectory(w Workspace, dir string) (string, error) {
	dir = strings.TrimSpace(strings.ReplaceAll(dir, "\\", "/"))
	if dir == "" || dir == "." {
		return ".", nil
	}
	p, err := safePath(w.Path, dir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(w.Path, p)
	if err != nil {
		return "", err
	}
	dir = filepath.ToSlash(rel)
	if err = visibleNotePath(dir); err != nil {
		return "", err
	}
	if info, e := os.Stat(p); e == nil && !info.IsDir() {
		return "", errors.New("归档目录是文件，不能保存笔记")
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return "", e
	}
	return dir, nil
}
func normalizedWords(words []string, limit int) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	if len(words) > limit {
		return nil, errors.New("关键词或标签数量过多")
	}
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		if len(word) > 256 || strings.ContainsAny(word, "\r\n\x00") {
			return nil, errors.New("关键词或标签过长或含有换行")
		}
		key := strings.ToLower(word)
		if !seen[key] {
			out = append(out, word)
			seen[key] = true
		}
	}
	return out, nil
}
func validateArchiveRules(w Workspace, rules []ArchiveRule) error {
	if len(rules) > 100 {
		return errors.New("最多支持 100 条归档规则")
	}
	ids := map[string]bool{}
	for i := range rules {
		r := &rules[i]
		r.Name = strings.TrimSpace(r.Name)
		if r.ID == "" || ids[r.ID] || len(r.ID) > 128 || r.Name == "" || len(r.Name) > 256 {
			return errors.New("规则 ID 重复或名称无效")
		}
		ids[r.ID] = true
		if r.Field != "title" && r.Field != "content" && r.Field != "project" && r.Field != "tag" {
			return errors.New("无效的匹配字段")
		}
		if r.Match != "any" && r.Match != "all" {
			return errors.New("无效的匹配方式")
		}
		var err error
		if r.Keywords, err = normalizedWords(r.Keywords, 32); err != nil {
			return err
		}
		if len(r.Keywords) == 0 {
			return errors.New("规则关键词不能为空")
		}
		if r.Tags, err = normalizedWords(r.Tags, 32); err != nil {
			return err
		}
		if r.Directory, err = archiveDirectory(w, r.Directory); err != nil {
			return fmt.Errorf("规则 %s：%w", r.Name, err)
		}
	}
	return nil
}
func (s *Service) archiveRules(w Workspace) (ArchiveRulesConfig, error) {
	config := ArchiveRulesConfig{Rules: []ArchiveRule{}}
	err := readJSON(filepath.Join(s.data(w), "archive-rules.json"), &config)
	if config.Rules == nil {
		config.Rules = []ArchiveRule{}
	}
	return config, err
}
func (s *Service) GetArchiveRules(workspaceID string) (ArchiveRulesConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return ArchiveRulesConfig{}, err
	}
	return s.archiveRules(w)
}
func (s *Service) SaveArchiveRules(workspaceID string, config ArchiveRulesConfig) (ArchiveRulesConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return config, err
	}
	previous, err := s.archiveRules(w)
	if err != nil {
		return config, err
	}
	if previous.Revision != config.Revision {
		return config, errors.New("CONFLICT: 规则已被修改，请重新加载后编辑")
	}
	if err = validateArchiveRules(w, config.Rules); err != nil {
		return config, err
	}
	config.Revision = uid()
	if config.Rules == nil {
		config.Rules = []ArchiveRule{}
	}
	err = writeJSON(filepath.Join(s.data(w), "archive-rules.json"), config)
	return config, err
}
func archiveBody(content string) string {
	text := strings.ReplaceAll(content, "\r\n", "\n")
	if strings.HasPrefix(text, "---\n") {
		lines := strings.Split(text, "\n")
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" {
				return strings.Join(lines[i+1:], "\n")
			}
		}
	}
	return text
}
func ruleMatches(r ArchiveRule, n Note) bool {
	values := []string{}
	switch r.Field {
	case "title":
		values = []string{n.Title}
	case "content":
		values = []string{archiveBody(n.Content)}
	case "project":
		values = []string{n.Project}
	case "tag":
		values = n.Tags
	}
	hits := 0
	for _, key := range r.Keywords {
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), strings.ToLower(key)) {
				hits++
				break
			}
		}
	}
	return hits > 0 && (r.Match == "any" || hits == len(r.Keywords))
}
func noteDirectory(path string) string {
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
	return dir
}
func (s *Service) SuggestArchive(workspaceID, noteID, hash string) (ArchiveSuggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := ArchiveSuggestion{WorkspaceID: workspaceID, NoteID: noteID, Hash: hash, Tags: []string{}, Reasons: []string{}}
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return out, err
	}
	notes, err := s.scan(w)
	if err != nil {
		return out, err
	}
	var current *Note
	for i := range notes {
		if notes[i].ID == noteID {
			current = &notes[i]
			break
		}
	}
	if current == nil {
		return out, errors.New("笔记不存在")
	}
	if current.Hash != hash {
		return out, errors.New("CONFLICT: 笔记已变化，请保存并重新获取建议")
	}
	config, err := s.archiveRules(w)
	if err != nil {
		return out, err
	}
	if err = validateArchiveRules(w, config.Rules); err != nil {
		return out, err
	}
	out.Directory = noteDirectory(current.Path)
	out.Source = "current"
	for _, rule := range config.Rules {
		if rule.Enabled && ruleMatches(rule, *current) {
			out.Directory = rule.Directory
			out.Tags = append(out.Tags, rule.Tags...)
			out.Source = "rule"
			out.RuleID = rule.ID
			out.Reasons = append(out.Reasons, "命中规则「"+rule.Name+"」，按规则顺序优先采用")
			break
		}
	}
	// Existing project/tag metadata supplies evidence, never a semantic-model claim.
	scores := map[string]int{}
	tagCandidates := map[string]string{}
	text := strings.ToLower(current.Title + "\n" + archiveBody(current.Content))
	for _, n := range notes {
		if n.ID == noteID {
			continue
		}
		projectMatch := current.Project != "" && strings.EqualFold(current.Project, n.Project)
		if projectMatch {
			scores[noteDirectory(n.Path)] += 3
		}
		for _, tag := range n.Tags {
			for _, existing := range current.Tags {
				if strings.EqualFold(tag, existing) {
					scores[noteDirectory(n.Path)]++
					break
				}
			}
			if strings.TrimSpace(tag) != "" && strings.Contains(text, strings.ToLower(tag)) {
				tagCandidates[strings.ToLower(tag)] = tag
			}
		}
	}
	if out.Source != "rule" && len(scores) > 0 {
		dirs := make([]string, 0, len(scores))
		for dir := range scores {
			dirs = append(dirs, dir)
		}
		sort.Slice(dirs, func(i, j int) bool {
			if scores[dirs[i]] != scores[dirs[j]] {
				return scores[dirs[i]] > scores[dirs[j]]
			}
			return dirs[i] < dirs[j]
		})
		if len(dirs) == 1 || scores[dirs[0]] > scores[dirs[1]] {
			out.Directory = dirs[0]
			out.Source = "metadata"
			out.Reasons = append(out.Reasons, "已有笔记的相同项目或标签主要分布在此目录")
		} else {
			out.Reasons = append(out.Reasons, "项目或标签对应多个目录且得分相同，保持当前目录")
		}
	}
	if len(out.Reasons) == 0 {
		out.Reasons = append(out.Reasons, "暂无明确匹配规则或目录证据，保持当前目录")
	}
	tagKeys := make([]string, 0, len(tagCandidates))
	for key := range tagCandidates {
		tagKeys = append(tagKeys, key)
	}
	sort.Strings(tagKeys)
	for _, key := range tagKeys {
		if len(out.Tags) >= 8 {
			break
		}
		out.Tags = append(out.Tags, tagCandidates[key])
	}
	unique, _ := normalizedWords(out.Tags, 100)
	out.Tags = []string{}
	for _, tag := range unique {
		exists := false
		for _, old := range current.Tags {
			if strings.EqualFold(old, tag) {
				exists = true
				break
			}
		}
		if !exists {
			out.Tags = append(out.Tags, tag)
		}
	}
	if len(out.Tags) > 0 {
		out.Reasons = append(out.Reasons, "标签来自命中规则，或工作区已有且在当前标题/正文出现的标签")
	}
	return out, nil
}

// Only the front matter is encoded again; the Markdown body remains byte-for-byte intact.
func archiveTags(n Note, tags []string) (string, error) {
	newline := "\n"
	if strings.Contains(n.Content, "\r\n") {
		newline = "\r\n"
	}
	text := strings.ReplaceAll(n.Content, "\r\n", "\n")
	root := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	body := n.Content
	if strings.HasPrefix(text, "---\n") {
		lines := strings.SplitAfter(n.Content, newline)
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSuffix(lines[i], newline) == "---" {
				end = i
				break
			}
		}
		if end < 0 {
			return "", errors.New("Front Matter 未闭合，请先修复 YAML")
		}
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "")), &doc); err != nil {
			return "", fmt.Errorf("Front Matter 无法解析：%w", err)
		}
		if len(doc.Content) > 0 {
			root = *doc.Content[0]
		}
		if root.Kind != yaml.MappingNode {
			return "", errors.New("Front Matter 必须为键值结构")
		}
		// yaml.Node decoding also rejects duplicate mapping keys.
		var check map[string]any
		if err := root.Decode(&check); err != nil {
			return "", err
		}
		body = strings.Join(lines[end+1:], "")
	}
	value := yaml.Node{}
	if err := value.Encode(tags); err != nil {
		return "", err
	}
	found := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "tags" {
			value.Anchor = root.Content[i+1].Anchor
			value.HeadComment = root.Content[i+1].HeadComment
			value.LineComment = root.Content[i+1].LineComment
			value.FootComment = root.Content[i+1].FootComment
			root.Content[i+1] = &value
			found = true
		}
	}
	if !found {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tags"}, &value)
	}
	if n.SyntheticID {
		idValue := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: n.ID}
		replaced := false
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "id" {
				root.Content[i+1] = idValue
				replaced = true
			}
		}
		if !replaced {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"}, idValue)
		}
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	meta := strings.ReplaceAll(buf.String(), "\n", newline)
	return "---" + newline + meta + "---" + newline + body, nil
}
func (s *Service) archiveNote(w Workspace, id, hash string) (Note, error) {
	notes, err := s.scan(w)
	if err != nil {
		return Note{}, err
	}
	for _, n := range notes {
		if n.ID == id {
			if n.Hash != hash {
				return Note{}, errors.New("CONFLICT: 笔记已变化，请重新获取建议")
			}
			return n, nil
		}
	}
	return Note{}, errors.New("笔记不存在或已删除")
}
func (s *Service) commitArchive(w Workspace, n Note, target, content string) (ArchiveResult, error) {
	if n.Path != target {
		if strings.EqualFold(n.Path, target) {
			return ArchiveResult{}, errors.New("暂不支持仅修改目录大小写")
		}
		result, err := s.moveNoteLocked(n.ID, n.Hash, target, &content)
		return ArchiveResult{Note: result.Note, UpdatedLinks: result.UpdatedLinks, RecoveryPath: result.RecoveryPath}, err
	}
	path, err := safePath(w.Path, n.Path)
	if err != nil {
		return ArchiveResult{}, err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return ArchiveResult{}, err
	}
	if digest(current) != n.Hash {
		return ArchiveResult{}, errors.New("CONFLICT: 笔记已被外部修改")
	}
	if err = atomicWrite(path, []byte(content)); err != nil {
		return ArchiveResult{}, err
	}
	return ArchiveResult{Note: parseNote(n.Path, []byte(content), time.Now())}, nil
}
func (s *Service) ApplyArchive(input ArchiveInput) (ArchiveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyArchiveLocked(input)
}
func (s *Service) applyArchiveLocked(input ArchiveInput) (ArchiveResult, error) {
	w, err := s.archiveWorkspace(input.WorkspaceID)
	if err != nil {
		return ArchiveResult{}, err
	}
	n, err := s.archiveNote(w, input.NoteID, input.Hash)
	if err != nil {
		return ArchiveResult{}, err
	}
	dir, err := archiveDirectory(w, input.Directory)
	if err != nil {
		return ArchiveResult{}, err
	}
	add, err := normalizedWords(input.Tags, 32)
	if err != nil {
		return ArchiveResult{}, err
	}
	tags, err := normalizedWords(append(append([]string{}, n.Tags...), add...), 100)
	if err != nil {
		return ArchiveResult{}, err
	}
	content := n.Content
	if strings.Join(tags, "\x00") != strings.Join(n.Tags, "\x00") {
		content, err = archiveTags(n, tags)
		if err != nil {
			return ArchiveResult{}, err
		}
	}
	target := filepath.ToSlash(filepath.Join(filepath.FromSlash(dir), filepath.Base(filepath.FromSlash(n.Path))))
	if target == n.Path && content == n.Content {
		return ArchiveResult{}, errors.New("目录和标签均未变化，无需整理")
	}
	opID := uid()
	recovery := filepath.Join(s.data(w), "archive", opID)
	if err = atomicWrite(filepath.Join(recovery, "before.md"), []byte(n.Content)); err != nil {
		return ArchiveResult{}, err
	}
	op := archiveOperation{NoteID: n.ID, BeforePath: n.Path, BeforeTags: n.Tags, State: "prepared", AfterPath: target, AfterHash: digest([]byte(content))}
	if err = writeJSON(filepath.Join(recovery, "operation.json"), op); err != nil {
		return ArchiveResult{}, err
	}
	out, err := s.commitArchive(w, n, target, content)
	if err != nil {
		return out, err
	}
	op.State = "committed"
	op.AfterHash = out.Note.Hash
	if err = writeJSON(filepath.Join(recovery, "operation.json"), op); err != nil {
		return out, fmt.Errorf("整理已完成，但撤销记录写入失败：%w。恢复副本：%s", err, recovery)
	}
	out.OperationID = opID
	out.RecoveryPath = recovery
	return out, nil
}
func (s *Service) UndoArchive(workspaceID, operationID, hash string) (ArchiveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if len(operationID) != 32 || strings.Trim(operationID, "0123456789abcdef") != "" {
		return ArchiveResult{}, errors.New("无效整理记录")
	}
	recovery := filepath.Join(s.data(w), "archive", operationID)
	op := archiveOperation{}
	if err = readJSON(filepath.Join(recovery, "operation.json"), &op); err != nil {
		return ArchiveResult{}, err
	}
	if op.State != "committed" || op.AfterHash != hash {
		return ArchiveResult{}, errors.New("整理记录无效或笔记已修改，不能自动撤销")
	}
	n, err := s.archiveNote(w, op.NoteID, hash)
	if err != nil {
		return ArchiveResult{}, err
	}
	if n.Path != op.AfterPath {
		return ArchiveResult{}, errors.New("笔记位置已变化，不能自动撤销整理")
	}
	content, err := archiveTags(n, op.BeforeTags)
	if err != nil {
		return ArchiveResult{}, err
	}
	out, err := s.commitArchive(w, n, op.BeforePath, content)
	if err != nil {
		return out, err
	}
	op.State = "undone"
	if err = writeJSON(filepath.Join(recovery, "operation.json"), op); err != nil {
		return out, err
	}
	out.RecoveryPath = recovery
	return out, nil
}

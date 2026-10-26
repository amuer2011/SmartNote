package core

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"smartnote/internal/embedding"
	"strings"
)

type SemanticCategory struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Directory   string               `json:"directory"`
	Enabled     bool                 `json:"enabled"`
	Examples    []SemanticArchiveRef `json:"examples,omitempty"`
}
type SemanticConfig struct {
	Revision         string             `json:"revision"`
	Endpoint         string             `json:"endpoint,omitempty"`
	Model            string             `json:"model"`
	AutomaticVersion int                `json:"automaticVersion"`
	Automatic        bool               `json:"automatic"`
	Threshold        float64            `json:"threshold"`
	Categories       []SemanticCategory `json:"categories"`
}

type SemanticArchiveRef struct {
	NoteID string `json:"noteId"`
	Hash   string `json:"hash"`
	Path   string `json:"path"`
}

type SemanticArchiveBatchItem struct {
	NoteID     string             `json:"noteId"`
	Suggestion *ArchiveSuggestion `json:"suggestion,omitempty"`
	Error      string             `json:"error,omitempty"`
}

type SemanticArchiveBatch struct {
	Revision string                     `json:"revision"`
	Items    []SemanticArchiveBatchItem `json:"items"`
}

const semanticArchiveBatchLimit = 24

func defaultSemantic() SemanticConfig {
	c := SemanticConfig{Endpoint: "", Model: embedding.ModelName, Automatic: true, AutomaticVersion: 1, Threshold: 0.55, Categories: []SemanticCategory{}}
	for i, v := range [][3]string{
		{"项目工作", "项目需求、业务设计、实施方案、进展、交付与问题跟踪", "工作/项目"},
		{"会议记录", "会议议程、讨论纪要、决议、行动项、参会人员", "工作/会议"},
		{"计划总结", "工作计划、周报、月报、复盘、目标和总结", "工作/计划总结"},
		{"数据库", "SQL、数据库迁移、数据库认证、索引、事务、备份、性能优化", "技术/数据库"},
		{"软件开发", "编程语言、接口、代码、测试、架构、前后端开发", "技术/软件开发"},
		{"运维网络", "Linux、服务器、容器、Kubernetes、网络、部署、监控和故障排查", "技术/运维网络"},
		{"人工智能", "Embedding、大语言模型、提示词、AI工具、机器学习", "技术/人工智能"},
		{"学习阅读", "课程笔记、读书摘录、学习心得、考试、知识理解", "学习/阅读课程"},
		{"方法资料", "通用方法论、操作指南、工具使用、参考资料、知识清单", "知识/方法资料"},
		{"日常生活", "生活日记、家庭、个人感悟、日常事务", "生活/日常"},
		{"健康运动", "饮食营养、锻炼、健身、体检、疾病与健康管理", "生活/健康运动"},
		{"财务消费", "预算、账单、理财、支出、收入、购物比较", "生活/财务消费"},
		{"旅行出行", "旅游攻略、行程、住宿、景点、交通安排", "生活/旅行出行"},
	} {
		c.Categories = append(c.Categories, SemanticCategory{ID: fmt.Sprintf("default-%d", i), Name: v[0], Description: v[1], Directory: v[2], Enabled: true})
	}
	return c
}
func (s *Service) semanticConfig(w Workspace) (SemanticConfig, error) {
	c := defaultSemantic()
	c.AutomaticVersion = 0 // Detect pre-default-on settings without changing their categories.
	err := readJSON(filepath.Join(s.data(w), "semantic-archive.json"), &c)
	normalizeSemantic(&c)
	return c, err
}

// Migrate v0.8 external-model settings in memory without touching user notes.
func normalizeSemantic(c *SemanticConfig) {
	if c.AutomaticVersion < 1 {
		c.Automatic = true
		c.AutomaticVersion = 1
	}
	c.Endpoint = ""
	c.Model = embedding.ModelName
	if c.Categories == nil {
		c.Categories = []SemanticCategory{}
	}
}
func validateSemantic(w Workspace, c SemanticConfig) error {
	if c.Endpoint != "" || c.Model != embedding.ModelName {
		return errors.New("当前版本使用内置离线模型，请重新加载归档设置")
	}
	if math.IsNaN(c.Threshold) || c.Threshold < 0 || c.Threshold > 1 {
		return errors.New("相似度须在 0 到 1 之间")
	}
	if len(c.Categories) > 100 {
		return errors.New("分类最多100项")
	}
	seen := map[string]bool{}
	for _, v := range c.Categories {
		if v.ID == "" || seen[v.ID] || strings.TrimSpace(v.Name) == "" || strings.TrimSpace(v.Description) == "" || len(v.Name) > 120 || len(v.Description) > 6000 {
			return errors.New("分类 ID 不可重复，名称和分类说明不能为空或过长")
		}
		seen[v.ID] = true
		if _, e := archiveDirectory(w, v.Directory); e != nil {
			return e
		}
		if e := validatePrototypeRefs(w, v.Examples); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) GetSemanticConfig(id string) (SemanticConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.archiveWorkspace(id)
	if e != nil {
		return SemanticConfig{}, e
	}
	return s.semanticConfig(w)
}
func (s *Service) SaveSemanticConfig(id string, c SemanticConfig) (SemanticConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.archiveWorkspace(id)
	if e != nil {
		return c, e
	}
	old, e := s.semanticConfig(w)
	if e != nil {
		return c, e
	}
	if old.Revision != c.Revision {
		return c, errors.New("分类配置已变化，请重新加载")
	}
	c.AutomaticVersion = 1 // An explicit opt-out in this version must survive reopening.
	if e = validateSemantic(w, c); e != nil {
		return c, e
	}
	if e = validateNewPrototypeExamples(w, old, c); e != nil {
		return c, e
	}
	if c.Categories == nil {
		c.Categories = []SemanticCategory{}
	}
	c.Revision = uid()
	return c, writeJSON(filepath.Join(s.data(w), "semantic-archive.json"), c)
}
func cosine(a, b []float64) (float64, error) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, errors.New("模型向量维度不一致")
	}
	var dot, aa, bb float64
	for i, x := range a {
		y := b[i]
		if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
			return 0, errors.New("模型返回无效向量")
		}
		dot += x * y
		aa += x * x
		bb += y * y
	}
	if aa == 0 || bb == 0 {
		return 0, errors.New("模型返回空向量")
	}
	return dot / math.Sqrt(aa*bb), nil
}
func (s *Service) SuggestSemanticArchive(id, noteID, hash string) (ArchiveSuggestion, error) {
	s.mu.Lock()
	w, e := s.archiveWorkspace(id)
	if e != nil {
		s.mu.Unlock()
		return ArchiveSuggestion{}, e
	}
	n, e := s.archiveNote(w, noteID, hash)
	if e != nil {
		s.mu.Unlock()
		return ArchiveSuggestion{}, e
	}
	c, e := s.semanticConfig(w)
	s.mu.Unlock()
	if e != nil {
		return ArchiveSuggestion{}, e
	}
	if e = validateSemantic(w, c); e != nil {
		return ArchiveSuggestion{}, e
	}
	cats := []SemanticCategory{}
	body := []rune(archiveBody(n.Content))
	if len(body) > 6000 {
		body = body[:6000]
	}
	texts := []string{n.Title + "\n" + string(body)}
	for _, v := range c.Categories {
		if v.Enabled {
			cats = append(cats, v)
			texts = append(texts, v.Name+"\n"+v.Description)
		}
	}
	if len(cats) == 0 {
		out := ArchiveSuggestion{WorkspaceID: id, NoteID: noteID, Hash: hash, Directory: noteDirectory(n.Path), Tags: []string{}, Reasons: []string{}, Source: "semantic-low"}
		out.Reasons = append(out.Reasons, "没有启用的分类，保留原目录")
		return out, nil
	}
	vectors, e := s.embedTexts(texts)
	if e != nil {
		return ArchiveSuggestion{}, e
	}
	if len(vectors) != len(texts) {
		return ArchiveSuggestion{}, errors.New("模型返回向量数量错误")
	}
	categoryVectors := make([][]float64, len(cats))
	for i := range cats {
		categoryVectors[i] = vectors[i+1]
	}
	groups := loadPrototypeGroups(w, cats)
	combined, learningReasons, e := s.blendPrototypeVectors(w, cats, categoryVectors, groups, n.ID)
	if e != nil {
		return ArchiveSuggestion{}, e
	}
	out, e := semanticSuggestion(id, n, c, cats, vectors[0], combined)
	if e != nil {
		return ArchiveSuggestion{}, e
	}

	// Inference never holds the workspace lock. Reject stale results before returning.
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e = s.archiveWorkspace(id)
	if e != nil {
		return out, e
	}
	if _, e = s.archiveNote(w, noteID, hash); e != nil {
		return out, e
	}
	latest, e := s.semanticConfig(w)
	if e != nil {
		return out, e
	}
	if latest.Revision != c.Revision {
		return out, errors.New("分类配置已变化，请重试")
	}
	if e = validatePrototypeGroups(w, groups); e != nil {
		return out, e
	}
	out.Reasons = append(out.Reasons, learningReasons...)
	return out, nil
}

// SuggestSemanticArchiveBatch analyzes only the referenced files for an
// interactive historical-archive preview. It does not modify notes, and each
// result is revalidated against disk and the configuration revision afterward.
func (s *Service) SuggestSemanticArchiveBatch(id string, refs []SemanticArchiveRef) (SemanticArchiveBatch, error) {
	if len(refs) == 0 || len(refs) > semanticArchiveBatchLimit {
		return SemanticArchiveBatch{}, fmt.Errorf("每批需要 1 到 %d 篇笔记", semanticArchiveBatchLimit)
	}

	s.mu.Lock()
	w, err := s.archiveWorkspace(id)
	if err != nil {
		s.mu.Unlock()
		return SemanticArchiveBatch{}, err
	}
	config, err := s.semanticConfig(w)
	s.mu.Unlock()
	if err != nil {
		return SemanticArchiveBatch{}, err
	}
	if err = validateSemantic(w, config); err != nil {
		return SemanticArchiveBatch{}, err
	}

	items := make([]SemanticArchiveBatchItem, len(refs))
	selected := make([]Note, len(refs))
	valid := make([]bool, len(refs))
	seen := make(map[string]bool, len(refs))
	for i, ref := range refs {
		items[i].NoteID = ref.NoteID
		switch {
		case ref.NoteID == "" || ref.Hash == "" || ref.Path == "":
			items[i].Error = "笔记标识无效"
		case seen[ref.NoteID]:
			items[i].Error = "批次中存在重复笔记"
		default:
			seen[ref.NoteID] = true
			note, readErr := readArchiveNote(w, ref.Path)
			if errors.Is(readErr, os.ErrNotExist) {
				items[i].Error = "笔记不存在或已删除"
			} else if readErr != nil {
				items[i].Error = readErr.Error()
			} else if note.ID != ref.NoteID {
				items[i].Error = "笔记标识已变化，请刷新工作区"
			} else if note.Hash != ref.Hash {
				items[i].Error = "笔记已变化，请重新分析"
			} else if isTaskArchiveNote(note) {
				items[i].Error = "已完成待办归档笔记不参与分类"
			} else {
				selected[i] = note
				valid[i] = true
			}
		}
	}

	categories := make([]SemanticCategory, 0, len(config.Categories))
	for _, category := range config.Categories {
		if category.Enabled {
			categories = append(categories, category)
		}
	}
	if len(categories) == 0 {
		for i := range items {
			if valid[i] {
				suggestion := ArchiveSuggestion{WorkspaceID: id, NoteID: selected[i].ID, Hash: selected[i].Hash, Directory: noteDirectory(selected[i].Path), Tags: []string{}, Reasons: []string{"没有启用的分类，保留原目录"}, Source: "semantic-low"}
				items[i].Suggestion = &suggestion
			}
		}
		items, err = s.revalidateSemanticBatch(id, config.Revision, selected, valid, items, nil)
		return SemanticArchiveBatch{Revision: config.Revision, Items: items}, err
	}

	categoryTexts := make([]string, len(categories))
	for i, category := range categories {
		categoryTexts[i] = category.Name + "\n" + category.Description
	}
	categoryVectors := make([][]float64, len(categories))
	for start := 0; start < len(categoryTexts); start += 16 {
		end := start + 16
		if end > len(categoryTexts) {
			end = len(categoryTexts)
		}
		vectors, embedErr := s.embedTexts(categoryTexts[start:end])
		if embedErr != nil {
			return SemanticArchiveBatch{}, embedErr
		}
		if len(vectors) != end-start {
			return SemanticArchiveBatch{}, errors.New("模型返回分类向量数量错误")
		}
		copy(categoryVectors[start:end], vectors)
	}

	groups := loadPrototypeGroups(w, categories)
	// Keep each inference call bounded so the model's per-call time limit does
	// not turn a large historical preview into one uninterruptible request.
	for start := 0; start < len(refs); start += 8 {
		end := start + 8
		if end > len(refs) {
			end = len(refs)
		}
		batchTexts := make([]string, 0, end-start)
		positions := make([]int, 0, end-start)
		for i := start; i < end; i++ {
			if !valid[i] {
				continue
			}
			body := []rune(archiveBody(selected[i].Content))
			if len(body) > 6000 {
				body = body[:6000]
			}
			batchTexts = append(batchTexts, selected[i].Title+"\n"+string(body))
			positions = append(positions, i)
		}
		if len(batchTexts) == 0 {
			continue
		}
		documentVectors, embedErr := s.embedTexts(batchTexts)
		if embedErr != nil {
			return SemanticArchiveBatch{}, embedErr
		}
		if len(documentVectors) != len(positions) {
			return SemanticArchiveBatch{}, errors.New("模型返回笔记向量数量错误")
		}
		for j, position := range positions {
			combined, learningReasons, classifyErr := s.blendPrototypeVectors(w, categories, categoryVectors, groups, selected[position].ID)
			if classifyErr != nil {
				items[position].Error = classifyErr.Error()
				continue
			}
			suggestion, classifyErr := semanticSuggestion(id, selected[position], config, categories, documentVectors[j], combined)
			if classifyErr != nil {
				items[position].Error = classifyErr.Error()
				continue
			}
			suggestion.Reasons = append(suggestion.Reasons, learningReasons...)
			items[position].Suggestion = &suggestion
		}
	}
	items, err = s.revalidateSemanticBatch(id, config.Revision, selected, valid, items, groups)
	return SemanticArchiveBatch{Revision: config.Revision, Items: items}, err
}

func semanticSuggestion(workspaceID string, note Note, config SemanticConfig, categories []SemanticCategory, documentVector []float64, categoryVectors [][]float64) (ArchiveSuggestion, error) {
	out := ArchiveSuggestion{WorkspaceID: workspaceID, NoteID: note.ID, Hash: note.Hash, Directory: noteDirectory(note.Path), Tags: []string{}, Reasons: []string{}, Source: "semantic-low"}
	best, index := -2.0, 0
	for i := range categories {
		score, err := cosine(documentVector, categoryVectors[i])
		if err != nil {
			return out, err
		}
		if score > best {
			best, index = score, i
		}
	}
	out.Score = best
	out.Reasons = append(out.Reasons, fmt.Sprintf("模型 %s：第一名「%s」，余弦相似度 %.6f（不是概率）", config.Model, categories[index].Name, best))
	if best >= config.Threshold {
		out.Reasons = append(out.Reasons, fmt.Sprintf("相似度达标：%.6f ≥ 最低余弦相似度 %.6f", best, config.Threshold))
		out.Directory = categories[index].Directory
		out.Source = "semantic"
		out.RuleID = categories[index].ID
		if out.Directory == noteDirectory(note.Path) {
			out.Reasons = append(out.Reasons, "分类已匹配当前目录，无需移动")
		}
	} else {
		out.Reasons = append(out.Reasons, fmt.Sprintf("未分类原因：相似度不足，%.6f < 最低余弦相似度 %.6f。建议补充笔记的主题与正文，或完善「%s」的分类描述；若现有分类都不合适，可新增分类。", best, config.Threshold, categories[index].Name))
		if best >= 0 {
			out.Reasons = append(out.Reasons, fmt.Sprintf("若确认应归入「%s」，可将最低余弦相似度调至 %.4f 或更低；降低门槛会增加误分类，建议优先手动指定目录。", categories[index].Name, math.Floor(best*10000)/10000))
		}
		out.Reasons = append(out.Reasons, "保留原目录；最高相似度达到配置门槛时归入最相似分类。门槛修改后自动保存，打开智能整理自动分析。")
	}
	return out, nil
}

func (s *Service) revalidateSemanticBatch(id, revision string, selected []Note, valid []bool, items []SemanticArchiveBatchItem, groups []prototypeGroup) ([]SemanticArchiveBatchItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(id)
	if err != nil {
		return nil, err
	}
	latest, err := s.semanticConfig(w)
	if err != nil {
		return nil, err
	}
	if latest.Revision != revision {
		return nil, errors.New("分类配置已变化，请重新分析")
	}
	if err = validateSemantic(w, latest); err != nil {
		return nil, err
	}
	if err = validatePrototypeGroups(w, groups); err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return items, nil
	}
	for i, ok := range valid {
		if !ok || items[i].Error != "" {
			continue
		}
		fresh, readErr := readArchiveNote(w, selected[i].Path)
		if errors.Is(readErr, os.ErrNotExist) {
			items[i].Suggestion = nil
			items[i].Error = "分析期间笔记已删除"
		} else if readErr != nil {
			items[i].Suggestion = nil
			items[i].Error = "分析期间无法读取笔记：" + readErr.Error()
		} else if fresh.ID != selected[i].ID || fresh.Hash != selected[i].Hash || fresh.Path != selected[i].Path {
			items[i].Suggestion = nil
			items[i].Error = "分析期间笔记已修改或移动，请重新分析"
		}
	}
	return items, nil
}

func readArchiveNote(w Workspace, rel string) (Note, error) {
	path, err := safePath(w.Path, rel)
	if err != nil {
		return Note{}, err
	}
	canonical := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if err = visibleNotePath(canonical); err != nil {
		return Note{}, err
	}
	if !strings.EqualFold(filepath.Ext(canonical), ".md") {
		return Note{}, errors.New("只能分析 Markdown 笔记")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Note{}, err
	}
	if !info.Mode().IsRegular() {
		return Note{}, errors.New("笔记路径不是普通文件")
	}
	if info.Size() > 32<<20 {
		return Note{}, errors.New("笔记超过 32 MB")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return Note{}, err
	}
	return parseNote(canonical, content, info.ModTime()), nil
}

// ApplyReviewedSemanticArchive only applies a preview created against the same
// semantic configuration revision. It still uses the standard archive journal,
// hash checks, backlink updates and recovery copy.
func (s *Service) ApplyReviewedSemanticArchive(revision string, input ArchiveInput) (ArchiveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(input.WorkspaceID)
	if err != nil {
		return ArchiveResult{}, err
	}
	config, err := s.semanticConfig(w)
	if err != nil {
		return ArchiveResult{}, err
	}
	if config.Revision != revision {
		return ArchiveResult{}, errors.New("分类配置已变化，请重新分析")
	}
	note, err := s.archiveNote(w, input.NoteID, input.Hash)
	if err != nil {
		return ArchiveResult{}, err
	}
	if isTaskArchiveNote(note) {
		return ArchiveResult{}, errors.New("已完成待办归档笔记不参与分类")
	}
	if input.SourcePath != "" && note.Path != input.SourcePath {
		return ArchiveResult{}, errors.New("笔记位置已变化，请重新分析")
	}
	return s.applyArchiveLocked(input)
}

func (s *Service) AutoArchive(id, noteID, hash string) (*ArchiveResult, error) {
	s.mu.Lock()
	w, e := s.archiveWorkspace(id)
	if e != nil {
		s.mu.Unlock()
		return nil, e
	}
	c, e := s.semanticConfig(w)
	if e != nil || !c.Automatic {
		s.mu.Unlock()
		return nil, e
	}
	n, e := s.archiveNote(w, noteID, hash)
	s.mu.Unlock()
	if e != nil {
		return nil, e
	}
	if isTaskArchiveNote(n) || n.AutomationIgnored {
		return nil, nil
	}
	if strings.TrimSpace(n.Title) == "" && strings.TrimSpace(archiveBody(n.Content)) == "" {
		return nil, nil
	}
	// An unchanged, explicitly selected example preserves the user's directory
	// choice. Editing it invalidates that version and resumes normal analysis.
	for _, category := range c.Categories {
		if !category.Enabled {
			continue
		}
		dir, err := archiveDirectory(w, category.Directory)
		if err != nil || dir != noteDirectory(n.Path) {
			continue
		}
		for _, ref := range category.Examples {
			if ref.NoteID == n.ID && ref.Hash == n.Hash && ref.Path == n.Path {
				return nil, nil
			}
		}
	}
	out, e := s.SuggestSemanticArchive(id, noteID, hash)
	if e != nil {
		return nil, e
	}
	if out.Source != "semantic" || out.Directory == noteDirectory(n.Path) {
		return nil, nil
	}
	// Recheck the configuration and switch state inside ApplyArchive's own lock.
	result, e := s.applySemanticArchive(c.Revision, ArchiveInput{WorkspaceID: id, NoteID: noteID, Hash: hash, Directory: out.Directory, Tags: out.Tags})
	return result, e
}

func (s *Service) applySemanticArchive(revision string, input ArchiveInput) (*ArchiveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.archiveWorkspace(input.WorkspaceID)
	if e != nil {
		return nil, e
	}
	c, e := s.semanticConfig(w)
	if e != nil {
		return nil, e
	}
	if !c.Automatic || c.Revision != revision {
		return nil, errors.New("自动归档配置已变化，请重试")
	}
	n, e := s.archiveNote(w, input.NoteID, input.Hash)
	if e != nil {
		return nil, e
	}
	if isTaskArchiveNote(n) || n.AutomationIgnored {
		return nil, nil
	}
	if input.Directory == noteDirectory(n.Path) {
		return nil, nil
	}
	r, e := s.applyArchiveLocked(input)
	return &r, e
}

func (s *Service) EmbeddingStatus() embedding.Status { return s.model.Status() }
func (s *Service) CheckEmbedding() (embedding.Status, error) {
	_, e := s.model.Embed([]string{"本地离线语义模型自检"})
	return s.model.Status(), e
}

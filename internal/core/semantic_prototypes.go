package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"smartnote/internal/embedding"
	"sort"
	"strings"
)

const prototypeSampleLimit = 20

type DirectoryLearningNote struct {
	SemanticArchiveRef
	Title string `json:"title"`
}
type DirectoryLearningPreview struct {
	Directory string                  `json:"directory"`
	Notes     []DirectoryLearningNote `json:"notes"`
	Total     int                     `json:"total"`
}

func (s *Service) PreviewDirectoryLearning(workspaceID, categoryID string) (DirectoryLearningPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := DirectoryLearningPreview{Notes: []DirectoryLearningNote{}}
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return out, err
	}
	config, err := s.semanticConfig(w)
	if err != nil {
		return out, err
	}
	var category *SemanticCategory
	for i := range config.Categories {
		if config.Categories[i].ID == categoryID {
			category = &config.Categories[i]
			break
		}
	}
	if category == nil {
		return out, errors.New("分类已不存在，请重新加载设置")
	}
	out.Directory, err = archiveDirectory(w, category.Directory)
	if err != nil {
		return out, err
	}
	notes, err := s.scan(w)
	if err != nil {
		return out, err
	}
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].Updated.Equal(notes[j].Updated) {
			return notes[i].Path < notes[j].Path
		}
		return notes[i].Updated.After(notes[j].Updated)
	})
	saved := map[string]bool{}
	for _, ref := range category.Examples {
		saved[ref.NoteID] = true
	}
	for _, n := range notes {
		if noteDirectory(n.Path) != out.Directory || isTaskArchiveNote(n) {
			continue
		}
		out.Total++
		if len(out.Notes) < 200 || saved[n.ID] {
			out.Notes = append(out.Notes, DirectoryLearningNote{SemanticArchiveRef: SemanticArchiveRef{NoteID: n.ID, Hash: n.Hash, Path: n.Path}, Title: n.Title})
		}
	}
	return out, nil
}

func validatePrototypeRefs(w Workspace, refs []SemanticArchiveRef) error {
	if len(refs) > prototypeSampleLimit {
		return errors.New("每个分类最多选择20篇目录学习样本")
	}
	seen := map[string]bool{}
	for _, r := range refs {
		if r.NoteID == "" || len(r.NoteID) > 512 || r.Hash == "" || len(r.Hash) > 128 || seen[r.NoteID] {
			return errors.New("目录学习样本标识、版本无效或重复")
		}
		seen[r.NoteID] = true
		if _, err := safePath(w.Path, r.Path); err != nil {
			return err
		}
		if err := visibleNotePath(r.Path); err != nil {
			return err
		}
		if !strings.EqualFold(filepath.Ext(r.Path), ".md") {
			return errors.New("学习样本必须是Markdown笔记")
		}
	}
	return nil
}

// Unchanged references may become stale after an edit. Keep settings editable;
// only newly confirmed sample selections must still match the files on disk.
func validateNewPrototypeExamples(w Workspace, old, next SemanticConfig) error {
	previous := map[string][]SemanticArchiveRef{}
	for _, c := range old.Categories {
		previous[c.ID] = c.Examples
	}
	for _, c := range next.Categories {
		if reflect.DeepEqual(previous[c.ID], c.Examples) {
			continue
		}
		dir, err := archiveDirectory(w, c.Directory)
		if err != nil {
			return err
		}
		for _, r := range c.Examples {
			n, err := readArchiveNote(w, r.Path)
			if err != nil {
				return fmt.Errorf("学习样本无法读取，请重新选择：%w", err)
			}
			if n.ID != r.NoteID || n.Hash != r.Hash || noteDirectory(n.Path) != dir || isTaskArchiveNote(n) {
				return errors.New("学习样本已修改、移动或不在分类目录下，请重新选择")
			}
		}
	}
	return nil
}

type prototypeGroup struct {
	notes      []Note
	configured int
}

func loadPrototypeGroups(w Workspace, categories []SemanticCategory) []prototypeGroup {
	groups := make([]prototypeGroup, len(categories))
	for i, c := range categories {
		groups[i].configured = len(c.Examples)
		dir, err := archiveDirectory(w, c.Directory)
		if err != nil {
			continue
		}
		for _, r := range c.Examples {
			n, err := readArchiveNote(w, r.Path)
			if err == nil && n.ID == r.NoteID && n.Hash == r.Hash && noteDirectory(n.Path) == dir && !isTaskArchiveNote(n) {
				groups[i].notes = append(groups[i].notes, n)
			}
		}
	}
	return groups
}

func validatePrototypeGroups(w Workspace, groups []prototypeGroup) error {
	for _, g := range groups {
		for _, n := range g.notes {
			fresh, err := readArchiveNote(w, n.Path)
			if err != nil || fresh.ID != n.ID || fresh.Hash != n.Hash {
				return errors.New("分析期间目录学习样本已变化，请重新分析或重新学习")
			}
		}
	}
	return nil
}

func prototypeText(n Note) string {
	body := []rune(archiveBody(n.Content))
	if len(body) > 6000 {
		body = body[:6000]
	}
	return n.Title + "\n" + string(body)
}

// Prototype vectors are derived data. The sample selection remains in the
// authoritative semantic configuration and can rebuild this SQLite cache.
func (s *Service) prototypeVector(workspace string, notes []Note, dimensions int) ([]float64, error) {
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	texts := make([]string, len(notes))
	parts := []string{embedding.Fingerprint()}
	for i, n := range notes {
		texts[i] = prototypeText(n)
		parts = append(parts, n.ID+":"+digest([]byte(texts[i])))
	}
	fingerprint := digest([]byte(strings.Join(parts, "\n")))
	var raw []byte
	var cached []float64
	if s.db.QueryRow("SELECT vector FROM prototype_vectors WHERE workspace=? AND fingerprint=?", workspace, fingerprint).Scan(&raw) == nil && json.Unmarshal(raw, &cached) == nil && len(cached) == dimensions {
		if _, err := cosine(cached, cached); err == nil {
			return cached, nil
		}
	}
	mean := make([]float64, dimensions)
	for start := 0; start < len(texts); start += 8 {
		end := start + 8
		if end > len(texts) {
			end = len(texts)
		}
		vectors, err := s.embedTexts(texts[start:end])
		if err != nil {
			return nil, err
		}
		if len(vectors) != end-start {
			return nil, errors.New("模型返回学习样本向量数量错误")
		}
		for _, v := range vectors {
			if len(v) != dimensions {
				return nil, errors.New("学习样本向量维度不一致")
			}
			if _, err := cosine(v, v); err != nil {
				return nil, err
			}
			norm := 0.0
			for _, x := range v {
				norm += x * x
			}
			norm = math.Sqrt(norm)
			for d, x := range v {
				mean[d] += x / norm
			}
		}
	}
	if _, err := cosine(mean, mean); err != nil {
		return nil, errors.New("学习样本语义相互抵消，请重新选择主题一致的笔记")
	}
	norm := 0.0
	for _, x := range mean {
		norm += x * x
	}
	norm = math.Sqrt(norm)
	for d := range mean {
		mean[d] /= norm
	}
	raw, _ = json.Marshal(mean)
	// Cache failures do not invalidate the independently verified model result.
	_, _ = s.db.Exec("INSERT INTO prototype_vectors(workspace,fingerprint,vector) VALUES(?,?,?) ON CONFLICT(workspace,fingerprint) DO UPDATE SET vector=excluded.vector", workspace, fingerprint, raw)
	_, _ = s.db.Exec("DELETE FROM prototype_vectors WHERE workspace=? AND rowid NOT IN (SELECT rowid FROM prototype_vectors WHERE workspace=? ORDER BY rowid DESC LIMIT 1024)", workspace, workspace)
	return mean, nil
}

func (s *Service) blendPrototypeVectors(w Workspace, categories []SemanticCategory, descriptions [][]float64, groups []prototypeGroup, exclude string) ([][]float64, []string, error) {
	combined := append([][]float64(nil), descriptions...)
	reasons := []string{}
	for i, c := range categories {
		g := groups[i]
		if g.configured == 0 {
			continue
		}
		samples := []Note{}
		for _, n := range g.notes {
			if n.ID != exclude {
				samples = append(samples, n)
			}
		}
		if len(samples) == 0 {
			reasons = append(reasons, fmt.Sprintf("「%s」没有可用的其他样本（已修改、移出、删除或为当前笔记），本次仅使用分类说明；可重新学习目录。", c.Name))
			continue
		}
		v, err := s.prototypeVector(w.ID, samples, len(descriptions[i]))
		if err != nil {
			return nil, nil, err
		}
		if _, err = cosine(descriptions[i], descriptions[i]); err != nil {
			return nil, nil, err
		}
		norm := 0.0
		for _, x := range descriptions[i] {
			norm += x * x
		}
		norm = math.Sqrt(norm)
		merged := make([]float64, len(v))
		for d := range v {
			merged[d] = .35*descriptions[i][d]/norm + .65*v[d]
		}
		combined[i] = merged
		reasons = append(reasons, fmt.Sprintf("「%s」采用%d篇已确认目录样本，与分类说明按65%%/35%%合成语义原型，再计算余弦相似度。", c.Name, len(samples)))
		if len(g.notes) < g.configured {
			reasons = append(reasons, fmt.Sprintf("「%s」有%d篇学习样本已变化，本次忽略；可重新学习更新样本。", c.Name, g.configured-len(g.notes)))
		}
	}
	return combined, reasons, nil
}

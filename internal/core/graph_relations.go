package core

import (
	"errors"
	"math"
	"path/filepath"
	"smartnote/internal/embedding"
	"sort"
	"strconv"
)

type GraphRelationInput struct {
	WorkspaceID string `json:"workspaceId"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	SourceHash  string `json:"sourceHash"`
	TargetHash  string `json:"targetHash"`
	Confirmed   bool   `json:"confirmed"`
}

type GraphSuggestions struct {
	Edges   []Edge `json:"edges"`
	Message string `json:"message"`
}

func relationKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	// Length-delimited IDs also handle externally supplied front matter IDs.
	return digest([]byte(strconv.Itoa(len(a)) + ":" + a + b))
}

func validateRelations(items []Relation) error {
	if len(items) > 50000 {
		return errors.New("人工关联最多保存50000条")
	}
	for _, r := range items {
		if r.Source == "" || r.Target == "" || r.Source == r.Target || len(r.Source) > 512 || len(r.Target) > 512 {
			return errors.New("人工关联的笔记标识无效")
		}
	}
	return nil
}

func (s *Service) graphRelations(w Workspace) ([]Relation, error) {
	items := []Relation{}
	if err := readJSON(filepath.Join(s.data(w), "relations.json"), &items); err != nil {
		return nil, err
	}
	return items, validateRelations(items)
}

// Only user confirmations are authoritative. Suggestions never alter source notes.
func (s *Service) SetGraphRelation(in GraphRelationInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	if w.ID != in.WorkspaceID {
		return errors.New("工作区已切换，请重新打开知识拓扑")
	}
	if in.Source == in.Target || in.Source == "" || in.Target == "" {
		return errors.New("请选择两篇不同的笔记")
	}
	notes, err := s.scan(w)
	if err != nil {
		return err
	}
	current := map[string]Note{}
	for _, n := range notes {
		current[n.ID] = n
	}
	a, okA := current[in.Source]
	b, okB := current[in.Target]
	if !okA || !okB {
		return errors.New("笔记已删除或不存在，请刷新拓扑")
	}
	if in.SourceHash == "" || in.TargetHash == "" || a.Hash != in.SourceHash || b.Hash != in.TargetHash {
		return errors.New("CONFLICT: 笔记内容已更新，请重新获取关联后操作")
	}
	items, err := s.graphRelations(w)
	if err != nil {
		return err
	}
	key := relationKey(in.Source, in.Target)
	next := []Relation{}
	for _, r := range items {
		if relationKey(r.Source, r.Target) != key {
			next = append(next, r)
		}
	}
	if in.Confirmed {
		next = append(next, Relation{Source: in.Source, Target: in.Target})
	}
	if err = validateRelations(next); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.data(w), "relations.json"), next)
}

// Reuse the search index, averaging a note's passages before comparing notes.
// The main service mutex is released for all model inference and comparisons.
func (s *Service) SuggestGraphRelations(workspaceID, noteID string, threshold float64) (GraphSuggestions, error) {
	out := GraphSuggestions{Edges: []Edge{}, Message: "基于全文段落平均向量的余弦相似度；每篇最多推荐3条，语义相似仅表示候选关联"}
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 || threshold > 1 {
		return out, errors.New("余弦相似度门槛应为0到1")
	}
	s.mu.Lock()
	w, err := s.active()
	if err != nil {
		s.mu.Unlock()
		return out, err
	}
	if w.ID != workspaceID {
		s.mu.Unlock()
		return out, errors.New("工作区已切换，请重新获取关联")
	}
	notes, err := s.scan(w)
	s.mu.Unlock()
	if err != nil {
		return out, err
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	focus := -1
	for i, n := range notes {
		if n.ID == noteID {
			focus = i
			break
		}
	}
	if noteID != "" && focus < 0 {
		return out, errors.New("当前笔记已不存在，请重新选择")
	}
	if len(notes) < 2 {
		out.Message = "至少需要两篇笔记才能推荐语义关联"
		return out, nil
	}
	if noteID == "" && len(notes) > 1500 {
		out.Message = "当前工作区超过1500篇笔记，请切换局部拓扑，按当前笔记获取语义关联；全局仍展示已有结构关系和人工关联"
		return out, nil
	}
	passages := searchPassages(notes)
	vectors, err := s.passageVectors(w.ID, notes, passages, embedding.Dimensions)
	if err != nil {
		return out, err
	}
	means := make([][]float64, len(notes))
	for i := range means {
		means[i] = make([]float64, embedding.Dimensions)
	}
	for i, p := range passages {
		for d, v := range vectors[i] {
			means[p.note][d] += v
		}
	}
	valid := make([]bool, len(notes))
	for i, v := range means {
		norm := 0.0
		for _, x := range v {
			norm += x * x
		}
		if norm > 0 {
			valid[i] = true
			norm = math.Sqrt(norm)
			for d := range v {
				v[d] /= norm
			}
		}
	}
	type match struct {
		index int
		score float64
	}
	seen := map[string]bool{}
	for i, a := range notes {
		if !valid[i] || (focus >= 0 && i != focus) {
			continue
		}
		best := []match{}
		for j := range notes {
			if i == j || !valid[j] {
				continue
			}
			score := 0.0
			for d, x := range means[i] {
				score += x * means[j][d]
			}
			score = math.Max(-1, math.Min(1, score))
			if score < threshold {
				continue
			}
			best = append(best, match{j, score})
			sort.SliceStable(best, func(x, y int) bool { return best[x].score > best[y].score })
			if len(best) > 3 {
				best = best[:3]
			}
		}
		for _, m := range best {
			b := notes[m.index]
			key := relationKey(a.ID, b.ID)
			if seen[key] {
				continue
			}
			seen[key] = true
			out.Edges = append(out.Edges, Edge{Source: a.ID, Target: b.ID, Type: "SEMANTIC_SIMILAR", Score: m.score, SourceHash: a.Hash, TargetHash: b.Hash})
		}
	}
	// Drop stale results and confirmations made while the model was running.
	s.mu.Lock()
	defer s.mu.Unlock()
	active, err := s.active()
	if err != nil {
		return out, err
	}
	if active.ID != w.ID {
		return GraphSuggestions{}, errors.New("工作区已切换，请重新获取关联")
	}
	fresh, err := s.scan(active)
	if err != nil {
		return out, err
	}
	current := map[string]Note{}
	previous := map[string]Note{}
	for _, n := range fresh {
		current[n.ID] = n
	}
	for _, n := range notes {
		previous[n.ID] = n
	}
	confirmed, err := s.graphRelations(w)
	if err != nil {
		return out, err
	}
	for _, r := range confirmed {
		seen[relationKey(r.Source, r.Target)] = false
	}
	kept := []Edge{}
	for _, e := range out.Edges {
		a, okA := current[e.Source]
		b, okB := current[e.Target]
		if okA && okB && a.Hash == e.SourceHash && b.Hash == e.TargetHash && a.Path == previous[e.Source].Path && b.Path == previous[e.Target].Path && seen[relationKey(e.Source, e.Target)] {
			kept = append(kept, e)
		}
	}
	out.Edges = kept
	sort.SliceStable(out.Edges, func(i, j int) bool { return out.Edges[i].Score > out.Edges[j].Score })
	return out, nil
}

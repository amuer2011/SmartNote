package core

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Graph annotations are metadata: editing them never changes Markdown or note identity.
type GraphAnnotations struct {
	Revision  string            `json:"revision"`
	Nodes     map[string]string `json:"nodes"`
	Relations map[string]string `json:"relations"`
}
type GraphAnnotationInput struct {
	WorkspaceID string `json:"workspaceId"`
	Revision    string `json:"revision"`
	NoteID      string `json:"noteId"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	Text        string `json:"text"`
}

func validateGraphAnnotations(a GraphAnnotations) error {
	if len(a.Nodes) > 10000 || len(a.Relations) > 50000 {
		return errors.New("拓扑备注数量超出限制")
	}
	for _, items := range []map[string]string{a.Nodes, a.Relations} {
		for id, text := range items {
			if id == "" || len(id) > 512 || !utf8.ValidString(text) || utf8.RuneCountInString(text) > 4000 {
				return errors.New("拓扑备注无效，每条最多4000字")
			}
		}
	}
	return nil
}
func loadGraphAnnotations(root string) (GraphAnnotations, error) {
	a := GraphAnnotations{Nodes: map[string]string{}, Relations: map[string]string{}}
	err := readJSON(filepath.Join(root, "graph-annotations.json"), &a)
	if err == nil {
		err = validateGraphAnnotations(a)
	}
	if a.Nodes == nil {
		a.Nodes = map[string]string{}
	}
	if a.Relations == nil {
		a.Relations = map[string]string{}
	}
	return a, err
}
func (s *Service) GetGraphAnnotations(workspaceID string) (GraphAnnotations, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(workspaceID)
	if err != nil {
		return GraphAnnotations{}, err
	}
	return loadGraphAnnotations(s.data(w))
}
func (s *Service) SaveGraphAnnotation(in GraphAnnotationInput) (GraphAnnotations, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.archiveWorkspace(in.WorkspaceID)
	if err != nil {
		return GraphAnnotations{}, err
	}
	a, err := loadGraphAnnotations(s.data(w))
	if err != nil {
		return a, err
	}
	if a.Revision != in.Revision {
		return a, errors.New("CONFLICT: 拓扑备注已更新，请重新打开后编辑")
	}
	notes, err := s.scan(w)
	if err != nil {
		return a, err
	}
	exists := map[string]bool{}
	for _, n := range notes {
		exists[n.ID] = true
	}
	items, key := a.Nodes, in.NoteID
	if in.NoteID != "" {
		if in.Source != "" || in.Target != "" || !exists[in.NoteID] {
			return a, errors.New("备注笔记不存在")
		}
	} else {
		if !exists[in.Source] || !exists[in.Target] || in.Source == in.Target {
			return a, errors.New("关联笔记不存在或无效")
		}
		related := false
		relations, e := s.graphRelations(w)
		if e != nil {
			return a, e
		}
		for _, r := range relations {
			if relationKey(r.Source, r.Target) == relationKey(in.Source, in.Target) {
				related = true
			}
		}
		for _, n := range notes {
			if n.ID != in.Source && n.ID != in.Target {
				continue
			}
			for _, ref := range markdownLinks(n.Content) {
				target := resolveNoteLink(n, ref, notes)
				if target != nil && ((n.ID == in.Source && target.ID == in.Target) || (n.ID == in.Target && target.ID == in.Source)) {
					related = true
				}
			}
		}
		if !related {
			return a, errors.New("该关联已不存在，请重新选择连线")
		}
		items, key = a.Relations, relationKey(in.Source, in.Target)
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		delete(items, key)
	} else {
		items[key] = text
	}
	if err = validateGraphAnnotations(a); err != nil {
		return a, err
	}
	a.Revision = uid()
	err = writeJSON(filepath.Join(s.data(w), "graph-annotations.json"), a)
	return a, err
}

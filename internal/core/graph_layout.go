package core

import (
	"errors"
	"math"
	"path/filepath"
)

type GraphPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type GraphLayout struct {
	Positions map[string]GraphPoint `json:"positions"`
	Zoom      float64               `json:"zoom"`
	Pan       GraphPoint            `json:"pan"`
}
type GraphLayouts map[string]GraphLayout

func validateGraphLayouts(layouts GraphLayouts) error {
	if len(layouts) > 128 {
		return errors.New("最多保存128个拓扑视图布局")
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= 1e7 }
	point := func(p GraphPoint) bool { return finite(p.X) && finite(p.Y) }
	for key, l := range layouts {
		if len(key) == 0 || len(key) > 512 || !finite(l.Zoom) || l.Zoom < .05 || l.Zoom > 3 || !point(l.Pan) || len(l.Positions) > 10000 {
			return errors.New("拓扑布局参数无效")
		}
		for id, p := range l.Positions {
			if len(id) == 0 || len(id) > 512 || !point(p) {
				return errors.New("拓扑节点位置无效")
			}
		}
	}
	return nil
}
func (s *Service) GetGraphLayouts(workspaceID string) (GraphLayouts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.graphWorkspace(workspaceID)
	if e != nil {
		return nil, e
	}
	l := GraphLayouts{}
	e = readJSON(filepath.Join(s.data(w), "graph-layouts.json"), &l)
	if e == nil {
		e = validateGraphLayouts(l)
	}
	return l, e
}
func (s *Service) SaveGraphLayout(workspaceID, key string, layout GraphLayout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.graphWorkspace(workspaceID)
	if e != nil {
		return e
	}
	l := GraphLayouts{}
	path := filepath.Join(s.data(w), "graph-layouts.json")
	if e = readJSON(path, &l); e != nil {
		return e
	}
	if l == nil {
		l = GraphLayouts{}
	}
	l[key] = layout
	if e = validateGraphLayouts(l); e != nil {
		return e
	}
	return writeJSON(path, l)
}

func (s *Service) graphWorkspace(id string) (Workspace, error) {
	for _, w := range s.settings.Workspaces {
		if w.ID == id {
			return w, nil
		}
	}
	return Workspace{}, errors.New("工作区不存在")
}

//go:build windows && amd64 && cgo

package core

import "testing"

func TestGraphRelationsBundledModel(t *testing.T) {
	s, w := fixture(t)
	a := mustNote(t, s, "数据库连接失败，排查SQL慢查询和数据库索引优化")
	b := mustNote(t, s, "SQL数据库的连接诊断、查询性能与索引调优")
	c := mustNote(t, s, "假期去海边旅行，预订酒店与安排景点行程")
	out, err := s.SuggestGraphRelations(w.ID, a.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Edges) != 2 || out.Edges[0].Target != b.ID {
		t.Fatalf("related note not first: %+v", out.Edges)
	}
	var related, other float64
	for _, e := range out.Edges {
		t.Logf("%s => %s %.4f", e.Source, e.Target, e.Score)
		if e.Target == b.ID {
			related = e.Score
		}
		if e.Target == c.ID {
			other = e.Score
		}
	}
	if related <= other {
		t.Fatalf("related %.4f <= unrelated %.4f", related, other)
	}
	filtered, err := s.SuggestGraphRelations(w.ID, a.ID, (related+other)/2)
	if err != nil || len(filtered.Edges) != 1 || filtered.Edges[0].Target != b.ID {
		t.Fatalf("threshold failed: %+v %v", filtered, err)
	}
}

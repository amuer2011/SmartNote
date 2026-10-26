//go:build windows && amd64 && cgo

package core

import "testing"

func TestSemanticSearchBundledModel(t *testing.T) {
	s, _ := fixture(t)
	n, e := s.CreateNote("这周末我计划去许昌胖东来", "生活/财务消费")
	if e != nil {
		t.Fatal(e)
	}
	n, e = s.SaveNote(SaveInput{ID: n.ID, Hash: n.Hash, Path: n.Path, Content: n.Content + "\n这周末我计划去许昌胖东来"})
	if e != nil {
		t.Fatal(e)
	}
	other := mustNote(t, s, "数据库慢查询排查")
	_, e = s.SaveNote(SaveInput{ID: other.ID, Hash: other.Hash, Path: other.Path, Content: other.Content + "\n分析SQL执行计划，添加索引优化数据库查询性能。"})
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range []string{"购物", "买东西", "超市", "出行", "消费", "数据库索引优化"} {
		r, e := s.SemanticSearch(q)
		if e != nil {
			t.Fatal(e)
		}
		for _, item := range r.Results {
			t.Logf("%s => %s %.4f %s", q, item.Note.Title, item.SemanticScore, item.MatchType)
		}
		want := n.ID
		if q == "数据库索引优化" {
			want = other.ID
		}
		if len(r.Results) == 0 || r.Results[0].Note.ID != want {
			t.Errorf("related shopping note not recalled for %s", q)
		}
	}
}

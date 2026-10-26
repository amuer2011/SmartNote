package core

import (
	"errors"
	"strings"
	"testing"
)

func TestSemanticSearchRelatedWithoutKeywordAndFallback(t *testing.T) {
	s, _ := fixture(t)
	n := mustNote(t, s, "周末购物计划")
	s.embedTexts = func(texts []string) ([][]float64, error) {
		out := [][]float64{}
		for _, text := range texts {
			if strings.HasPrefix(text, "为这个句子") || strings.Contains(text, n.Title) {
				out = append(out, []float64{1, 0})
			} else {
				out = append(out, []float64{0, 1})
			}
		}
		return out, nil
	}
	r, e := s.SemanticSearch("采购商品")
	if e != nil || r.Mode != "hybrid" || len(r.Results) != 1 || r.Results[0].Note.ID != n.ID || r.Results[0].MatchType != "semantic" {
		t.Fatal(r, e)
	}
	s.embedTexts = func([]string) ([][]float64, error) { return nil, errors.New("model failed") }
	r, e = s.SemanticSearch("购物")
	if e != nil || r.Mode != "keyword" || len(r.Results) != 1 || r.Results[0].MatchType != "keyword" || !strings.Contains(r.Message, "暂不可用") {
		t.Fatal(r, e)
	}
}
func TestSemanticSearchDoesNotHoldLockOrReturnStaleNotes(t *testing.T) {
	s, _ := fixture(t)
	n := mustNote(t, s, "并发搜索")
	started, release := make(chan struct{}), make(chan struct{})
	first := true
	s.embedTexts = func(texts []string) ([][]float64, error) {
		if first {
			first = false
			close(started)
			<-release
		}
		out := [][]float64{}
		for range texts {
			out = append(out, []float64{1, 0})
		}
		return out, nil
	}
	done := make(chan SearchResponse, 1)
	go func() { r, _ := s.SemanticSearch("任意语义"); done <- r }()
	<-started
	_, e := s.SaveNote(SaveInput{ID: n.ID, Hash: n.Hash, Path: n.Path, Content: n.Content + "\n新的正文"})
	close(release)
	if e != nil {
		t.Fatal(e)
	}
	if r := <-done; len(r.Results) != 0 {
		t.Fatal("stale note returned", r)
	}
}
func TestSearchPassagesLongLineAndBodyCoverage(t *testing.T) {
	content := "---\ntitle: 测试\n---\n" + strings.Repeat("长", 1200) + "\n最终段落"
	p := searchPassages([]Note{{Title: "测试", Content: content}})
	if len(p) < 4 {
		t.Fatal(p)
	}
	if !strings.Contains(p[len(p)-1].text, "最终段落") {
		t.Fatal("truncated body")
	}

}

func TestRankFusionKeepsKeywordRecallAndCosineSeparate(t *testing.T) {
	notes := []Note{{ID: "semantic"}, {ID: "both"}, {ID: "keyword"}}
	best := map[string]SearchResult{"semantic": {Note: notes[0], SemanticScore: .95}, "both": {Note: notes[1], SemanticScore: .6}, "keyword": {Note: notes[2], SemanticScore: .1}}
	lexical := []SearchResult{{Note: notes[2], Score: 999, Snippet: "exact", Line: 12}, {Note: notes[1], Score: 1}}
	results := fuseSearchResults(notes, best, lexical)
	if len(results) != 3 || results[0].Note.ID != "both" {
		t.Fatalf("two-channel match not ranked first: %+v", results)
	}
	for _, r := range results {
		if r.Note.ID == "keyword" && (r.MatchType != "keyword" || r.Line != 12 || r.Snippet != "exact") {
			t.Fatalf("keyword-only recall lost: %+v", r)
		}
		if r.Note.ID == "semantic" && r.SemanticScore != .95 {
			t.Fatal("cosine overwritten by fusion")
		}
	}
}

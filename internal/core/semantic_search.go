package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Mode    string         `json:"mode"`
	Message string         `json:"message"`
}
type searchPassage struct {
	note int
	line int
	text string
}

// Short overlapping passages avoid diluting a matching paragraph in a long note.
func searchPassages(notes []Note) []searchPassage {
	out := []searchPassage{}
	for ni, n := range notes {
		prefix := n.Title + "\n" + n.Path + "\n" + strings.Join(n.Tags, " ") + "\n"
		lines := strings.Split(strings.ReplaceAll(n.Content, "\r\n", "\n"), "\n")
		start := 0
		if len(lines) > 0 && lines[0] == "---" {
			for i := 1; i < len(lines); i++ {
				if lines[i] == "---" {
					start = i + 1
					break
				}
			}
		}
		added := false
		for row := start; row < len(lines); {
			first := row
			text := ""
			for row < len(lines) {
				runes := []rune(lines[row])
				if len(runes) > 400 {
					if text != "" {
						break
					}
					for offset := 0; offset < len(runes); offset += 320 {
						end := offset + 400
						if end > len(runes) {
							end = len(runes)
						}
						out = append(out, searchPassage{ni, row, prefix + string(runes[offset:end])})
						added = true
						if end == len(runes) {
							break
						}
					}
					row++
					break
				}
				if text != "" && len([]rune(text))+len(runes) > 400 {
					break
				}
				text += lines[row] + "\n"
				row++
			}
			if strings.TrimSpace(text) != "" {
				out = append(out, searchPassage{ni, first, prefix + text})
				added = true
			}
		}
		if !added {
			out = append(out, searchPassage{ni, 0, prefix})
		}
	}
	return out
}
func (s *Service) SemanticSearch(query string) (SearchResponse, error) {
	out := SearchResponse{Results: []SearchResult{}, Mode: "hybrid", Message: "内置模型语义检索 + 关键词匹配；语义召回门槛 0.30，相似度不是概率"}
	query = strings.TrimSpace(query)
	if query == "" {
		return out, nil
	}
	s.mu.Lock()
	w, e := s.active()
	if e != nil {
		s.mu.Unlock()
		return out, e
	}
	notes, e := s.scan(w)
	if e != nil {
		s.mu.Unlock()
		return out, e
	}
	lexical, e := s.keywordSearchLocked(w, notes, query)
	s.mu.Unlock()
	if e != nil {
		return out, e
	}
	if len(notes) == 0 {
		return out, nil
	}
	passages := searchPassages(notes)
	best := map[string]SearchResult{}
	var modelError error
	vectors, e := s.embedTexts([]string{"为这个句子生成表示以用于检索相关文章：" + query})
	if e != nil {
		modelError = e
	} else if len(vectors) != 1 {
		modelError = errors.New("模型返回查询向量数量错误")
	} else {
		_, modelError = cosine(vectors[0], vectors[0])
	}
	var batch [][]float64
	if modelError == nil {
		batch, modelError = s.passageVectors(w.ID, notes, passages, len(vectors[0]))
	}
	if modelError == nil {
		for i, v := range batch {
			score, err := cosine(vectors[0], v)
			if err != nil {
				modelError = err
				break
			}
			p := passages[i]
			n := notes[p.note]
			if previous, ok := best[n.ID]; ok && previous.SemanticScore >= score {
				continue
			}
			body := strings.Split(strings.ReplaceAll(n.Content, "\r\n", "\n"), "\n")
			line := p.line
			for line < len(body)-1 && strings.TrimSpace(body[line]) == "" {
				line++
			}
			snippet := []rune(strings.TrimSpace(strings.Join(body[line:], " ")))
			if len(snippet) > 180 {
				snippet = snippet[:180]
			}
			best[n.ID] = SearchResult{Note: n, Snippet: string(snippet), Line: line, Score: score, SemanticScore: score, MatchType: "semantic"}
		}
	}
	if modelError != nil {
		out.Mode = "keyword"
		out.Message = "语义模型暂不可用，当前仅显示关键词结果：" + modelError.Error()
		for _, r := range lexical {
			r.MatchType = "keyword"
			out.Results = append(out.Results, r)
		}
	} else {
		out.Results = fuseSearchResults(notes, best, lexical)
	}

	// Inference runs outside the service lock. Never return a different workspace's
	// results, or deleted/edited notes captured before inference.
	s.mu.Lock()
	defer s.mu.Unlock()
	active, err := s.active()
	if err != nil {
		return out, err
	}
	if active.ID != w.ID {
		return SearchResponse{}, fmt.Errorf("工作区已切换，请重新搜索")
	}
	current, err := s.scan(active)
	if err != nil {
		return out, err
	}
	fresh := map[string]Note{}
	for _, n := range current {
		fresh[n.ID] = n
	}
	kept := []SearchResult{}
	for _, r := range out.Results {
		n, ok := fresh[r.Note.ID]
		if ok && n.Hash == r.Note.Hash && n.Path == r.Note.Path {
			kept = append(kept, r)
		}
	}
	out.Results = kept
	return out, nil
}

// Reciprocal rank fusion avoids mixing unrelated keyword and cosine scales.
func fuseSearchResults(notes []Note, best map[string]SearchResult, lexical []SearchResult) []SearchResult {
	semantic := []SearchResult{}
	for _, note := range notes {
		if r, ok := best[note.ID]; ok && r.SemanticScore >= 0.30 {
			semantic = append(semantic, r)
		}
	}
	sort.SliceStable(semantic, func(i, j int) bool { return semantic[i].SemanticScore > semantic[j].SemanticScore })
	merged := map[string]SearchResult{}
	for i, r := range semantic {
		r.Score = 1 / float64(60+i+1)
		r.MatchType = "semantic"
		merged[r.Note.ID] = r
	}
	for i, keyword := range lexical {
		r, ok := merged[keyword.Note.ID]
		if ok {
			r.MatchType = "both"
		} else {
			r = keyword
			r.Score = 0
			r.SemanticScore = best[keyword.Note.ID].SemanticScore
			r.MatchType = "keyword"
		}
		r.Score += 1 / float64(60+i+1)
		if keyword.Snippet != "" {
			r.Snippet = keyword.Snippet
			r.Line = keyword.Line
		}
		merged[keyword.Note.ID] = r
	}
	results := []SearchResult{}
	for _, note := range notes {
		if r, ok := merged[note.ID]; ok {
			results = append(results, r)
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].SemanticScore > results[j].SemanticScore
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > 50 {
		results = results[:50]
	}
	return results
}

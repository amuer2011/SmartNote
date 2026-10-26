package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func countingEmbedding(counter *int) func([]string) ([][]float64, error) {
	return func(texts []string) ([][]float64, error) {
		out := [][]float64{}
		for _, text := range texts {
			if !strings.HasPrefix(text, "为这个句子") {
				*counter++
			}
			out = append(out, []float64{1, 0})
		}
		return out, nil
	}
}
func requireSearch(t *testing.T, s *Service) {
	t.Helper()
	out, err := s.SemanticSearch("语义测试")
	if err != nil || out.Mode != "hybrid" {
		t.Fatalf("%+v %v", out, err)
	}
}
func TestPassageVectorsPersistAndIncrementallyRebuild(t *testing.T) {
	s, w := fixture(t)
	mustNote(t, s, "第一篇")
	second := mustNote(t, s, "第二篇")
	computed := 0
	s.embedTexts = countingEmbedding(&computed)
	requireSearch(t, s)
	if computed != 2 {
		t.Fatalf("first build: %d", computed)
	}
	computed = 0
	requireSearch(t, s)
	if computed != 0 {
		t.Fatalf("unchanged recomputed: %d", computed)
	}
	reopened, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.embedTexts = countingEmbedding(&computed)
	requireSearch(t, reopened)
	if computed != 0 {
		t.Fatalf("restart recomputed: %d", computed)
	}
	saved, err := s.SaveNote(SaveInput{ID: second.ID, Path: second.Path, Hash: second.Hash, Content: second.Content + "\n更新内容"})
	if err != nil {
		t.Fatal(err)
	}
	requireSearch(t, s)
	if computed != 1 {
		t.Fatalf("edit recomputed %d passages", computed)
	}
	computed = 0
	moved, err := s.MoveNote(saved.ID, saved.Hash, "技术/第二篇.md")
	if err != nil {
		t.Fatal(err)
	}
	requireSearch(t, s)
	if computed != 1 {
		t.Fatalf("move recomputed %d passages", computed)
	}
	computed = 0
	if err = s.DeleteNote(moved.Note.ID, moved.Note.Hash); err != nil {
		t.Fatal(err)
	}
	requireSearch(t, s)
	if computed != 0 {
		t.Fatalf("delete recomputed %d", computed)
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM passage_vectors WHERE workspace=?", w.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stale vectors: %d %v", count, err)
	}
}
func TestPassageVectorsCorruptionModelAndWorkspaceIsolation(t *testing.T) {
	s, _ := fixture(t)
	n := mustNote(t, s, "笔记")
	computed := 0
	s.embedTexts = countingEmbedding(&computed)
	requireSearch(t, s)
	for _, damage := range []string{"UPDATE passage_vectors SET vector='broken'", "UPDATE passage_vectors SET vector='[0,0]'", "UPDATE passage_vectors SET vector='[1,0,0]'", "UPDATE passage_vectors SET model='older-model'"} {
		if _, err := s.db.Exec(damage); err != nil {
			t.Fatal(err)
		}
		computed = 0
		requireSearch(t, s)
		if computed != 1 {
			t.Fatalf("not rebuilt after %s: %d", damage, computed)
		}
	}
	_, err := s.AddWorkspace("另一个", filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatal(err)
	}
	other := mustNote(t, s, "笔记")
	_, err = s.SaveNote(SaveInput{ID: other.ID, Hash: other.Hash, Path: other.Path, Content: strings.ReplaceAll(n.Content, n.ID, other.ID)})
	if err != nil {
		t.Fatal(err)
	}
	computed = 0
	requireSearch(t, s)
	if computed != 1 {
		t.Fatalf("workspace leaked cached vector: %d", computed)
	}
}

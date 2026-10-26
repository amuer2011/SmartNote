package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func prototypeFixture(t *testing.T) (*Service, Workspace, SemanticConfig, Note, Note, *int) {
	t.Helper()
	s, w := fixture(t)
	c, _ := s.GetSemanticConfig(w.ID)
	c.Threshold = .75
	c.Categories = []SemanticCategory{{ID: "a", Name: "类别甲", Description: "甲的说明", Directory: "案例", Enabled: true}, {ID: "b", Name: "类别乙", Description: "乙的说明", Directory: "其他", Enabled: true}}
	c, err := s.SaveSemanticConfig(w.ID, c)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := s.CreateNote("典型案例", "案例")
	if err != nil {
		t.Fatal(err)
	}
	n := mustNote(t, s, "新记录")
	count := new(int)
	s.embedTexts = func(texts []string) ([][]float64, error) {
		out := make([][]float64, len(texts))
		for i, text := range texts {
			switch {
			case strings.HasPrefix(text, "类别甲\n"):
				out[i] = []float64{1, 0}
			case strings.HasPrefix(text, "类别乙\n"):
				out[i] = []float64{.6, .8}
			default:
				out[i] = []float64{0, 1}
				if strings.HasPrefix(text, "典型案例\n") {
					*count++
				}
			}
		}
		return out, nil
	}
	return s, w, c, sample, n, count
}
func prototypeRef(n Note) SemanticArchiveRef {
	return SemanticArchiveRef{NoteID: n.ID, Path: n.Path, Hash: n.Hash}
}

func TestDirectoryPrototypeChangesClassificationAcrossAllPaths(t *testing.T) {
	s, w, c, sample, n, count := prototypeFixture(t)
	before, err := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
	if err != nil || before.RuleID != "b" {
		t.Fatalf("baseline %+v %v", before, err)
	}
	c.Categories[0].Examples = []SemanticArchiveRef{prototypeRef(sample)}
	c, err = s.SaveSemanticConfig(w.ID, c)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		out, err := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
		if err != nil || out.RuleID != "a" || out.Score < .88 || !strings.Contains(strings.Join(out.Reasons, ""), "1篇已确认目录样本") {
			t.Fatalf("learned %+v %v", out, err)
		}
	}
	if *count != 1 {
		t.Fatalf("sample recomputed %d times", *count)
	}
	batch, err := s.SuggestSemanticArchiveBatch(w.ID, []SemanticArchiveRef{prototypeRef(n)})
	if err != nil || batch.Items[0].Suggestion == nil || batch.Items[0].Suggestion.RuleID != "a" {
		t.Fatalf("batch %+v %v", batch, err)
	}
	moved, err := s.AutoArchive(w.ID, n.ID, n.Hash)
	if err != nil || moved == nil || noteDirectory(moved.Note.Path) != "案例" {
		t.Fatalf("auto %+v %v", moved, err)
	}
	if workspaceFile(t, w.Path, sample.Path) != sample.Content {
		t.Fatal("learning modified sample")
	}
	backup := filepath.Join(t.TempDir(), "learned.zip")
	if err = s.Export(backup, true); err != nil {
		t.Fatal(err)
	}
	restored, err := s.Import(backup, filepath.Join(t.TempDir(), "restore"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := s.GetSemanticConfig(restored.ID)
	if err != nil || len(config.Categories[0].Examples) != 1 {
		t.Fatalf("backup lost sample references: %+v %v", config, err)
	}
	// The original SQLite cache is scoped to the old workspace; restore rebuilds it.
	if _, err = s.SuggestSemanticArchive(restored.ID, moved.Note.ID, moved.Note.Hash); err != nil {
		t.Fatal(err)
	}
	if *count != 2 {
		t.Fatalf("restored prototype was not rebuilt: %d", *count)
	}
}

func TestDirectoryLearningDirectFilesValidationAndStaleSamples(t *testing.T) {
	s, w, c, sample, n, _ := prototypeFixture(t)
	if _, err := s.CreateNote("子目录笔记", "案例/子目录"); err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewDirectoryLearning(w.ID, "a")
	if err != nil || p.Total != 1 || p.Notes[0].NoteID != sample.ID {
		t.Fatalf("mixed directories: %+v %v", p, err)
	}
	for _, refs := range [][]SemanticArchiveRef{{prototypeRef(n)}, {{NoteID: sample.ID, Hash: "old", Path: sample.Path}}, {prototypeRef(sample), prototypeRef(sample)}, {{NoteID: sample.ID, Hash: sample.Hash, Path: "../outside.md"}}} {
		bad := c
		bad.Categories = append([]SemanticCategory(nil), c.Categories...)
		bad.Categories[0].Examples = refs
		if _, err = s.SaveSemanticConfig(w.ID, bad); err == nil {
			t.Fatalf("invalid samples accepted: %+v", refs)
		}
	}
	c.Categories[0].Examples = []SemanticArchiveRef{prototypeRef(sample)}
	c, err = s.SaveSemanticConfig(w.ID, c)
	if err != nil {
		t.Fatal(err)
	}
	self, err := s.SuggestSemanticArchive(w.ID, sample.ID, sample.Hash)
	if err != nil || self.RuleID != "b" || !strings.Contains(strings.Join(self.Reasons, ""), "为当前笔记") {
		t.Fatalf("self match: %+v %v", self, err)
	}
	if result, err := s.AutoArchive(w.ID, sample.ID, sample.Hash); err != nil || result != nil {
		t.Fatalf("unchanged confirmed sample was auto-moved: %+v %v", result, err)
	}
	if _, err = s.SaveNote(SaveInput{ID: sample.ID, Path: sample.Path, Hash: sample.Hash, Content: sample.Content + "\n编辑后不再作为旧样本"}); err != nil {
		t.Fatal(err)
	}
	c.Threshold = .7
	c, err = s.SaveSemanticConfig(w.ID, c)
	if err != nil {
		t.Fatalf("stale sample blocked unrelated setting: %v", err)
	}
	out, err := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
	if err != nil || out.RuleID != "b" || !strings.Contains(strings.Join(out.Reasons, ""), "没有可用") {
		t.Fatalf("stale sample used: %+v %v", out, err)
	}
	c.Categories[0].Examples = nil
	if _, err = s.SaveSemanticConfig(w.ID, c); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryLearningIncludesOlderConfirmedSamples(t *testing.T) {
	s, w, c, sample, _, _ := prototypeFixture(t)
	c.Categories[0].Examples = []SemanticArchiveRef{prototypeRef(sample)}
	if _, err := s.SaveSemanticConfig(w.ID, c); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		if err := os.WriteFile(filepath.Join(w.Path, "案例", fmt.Sprintf("较新-%03d.md", i)), []byte("# 新笔记"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := s.PreviewDirectoryLearning(w.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range preview.Notes {
		if n.NoteID == sample.ID {
			found = true
		}
	}
	if !found || preview.Total != 206 {
		t.Fatalf("older saved sample omitted: total %d found %v", preview.Total, found)
	}
}

func TestDirectoryPrototypeInferenceDoesNotLockAndRejectsEditedSamples(t *testing.T) {
	s, w, c, sample, n, _ := prototypeFixture(t)
	c.Categories[0].Examples = []SemanticArchiveRef{prototypeRef(sample)}
	if _, err := s.SaveSemanticConfig(w.ID, c); err != nil {
		t.Fatal(err)
	}
	base := s.embedTexts
	started, release := make(chan struct{}), make(chan struct{})
	s.embedTexts = func(texts []string) ([][]float64, error) {
		if len(texts) == 1 && strings.HasPrefix(texts[0], sample.Title+"\n") {
			close(started)
			<-release
		}
		return base(texts)
	}
	done := make(chan error, 1)
	go func() { _, err := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash); done <- err }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		close(release)
		t.Fatal("inference did not start")
	}
	saved := make(chan error, 1)
	go func() {
		_, err := s.SaveNote(SaveInput{ID: sample.ID, Path: sample.Path, Hash: sample.Hash, Content: sample.Content + "\n并发编辑"})
		saved <- err
	}()
	select {
	case err := <-saved:
		if err != nil {
			close(release)
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		<-done
		t.Fatal("inference held service lock")
	}
	close(release)
	if err := <-done; err == nil || !strings.Contains(err.Error(), "样本已变化") {
		t.Fatalf("stale inference accepted: %v", err)
	}
}

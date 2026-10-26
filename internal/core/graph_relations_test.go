package core

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"smartnote/internal/embedding"
)

// graphTestAppend adds Markdown body text while preserving the note identity.
func graphTestAppend(t *testing.T, s *Service, n Note, body string) Note {
	t.Helper()
	updated, err := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + body})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

// graphTestMeta rewrites the empty front matter tags created by CreateNote.
func graphTestMeta(t *testing.T, s *Service, n Note, project string, tags []string) Note {
	t.Helper()
	replacement := "tags: [" + strings.Join(tags, ", ") + "]"
	if project != "" {
		replacement = "project: " + project + "\n" + replacement
	}
	content := strings.Replace(n.Content, "tags: []", replacement, 1)
	if content == n.Content {
		t.Fatalf("note %q has no empty tags metadata", n.Title)
	}
	updated, err := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func graphTestEdge(edges []Edge, kind, source, target string) (Edge, bool) {
	for _, e := range edges {
		if e.Type == kind && e.Source == source && e.Target == target {
			return e, true
		}
	}
	return Edge{}, false
}

func graphTestPair(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

// graphTestBetween counts relations between the same pair in either direction.
func graphTestBetween(edges []Edge, kind, a, b string) int {
	want := graphTestPair(a, b)
	count := 0
	for _, e := range edges {
		if e.Type == kind && graphTestPair(e.Source, e.Target) == want {
			count++
		}
	}
	return count
}

func graphTestVector(values ...float64) []float64 {
	v := make([]float64, embedding.Dimensions)
	copy(v, values)
	return v
}

// graphTestEmbeddings resolves each passage to a vector by its note title.
func graphTestEmbeddings(byTitle map[string][]float64) func([]string) ([][]float64, error) {
	return func(texts []string) ([][]float64, error) {
		out := make([][]float64, len(texts))
		for i, text := range texts {
			for title, vector := range byTitle {
				if strings.HasPrefix(text, title+"\n") {
					out[i] = vector
					break
				}
			}
			if out[i] == nil {
				return nil, fmt.Errorf("unexpected graph embedding input %q", text)
			}
		}
		return out, nil
	}
}

func TestGraphBuildsStructuredRelationsFromLinksProjectsAndTags(t *testing.T) {
	s, _ := fixture(t)
	source := mustNote(t, s, "图谱来源")
	target := mustNote(t, s, "图谱目标")
	wiki := mustNote(t, s, "图谱双链")
	reference := mustNote(t, s, "图谱引用")
	source = graphTestAppend(t, s, source, "[目标](图谱目标.md)\n[[图谱双链]]\n[引用][ref]\n[ref]: 图谱引用.md\n[重复](图谱目标.md)\n[缺失](不存在.md)\n")
	projectA := graphTestMeta(t, s, mustNote(t, s, "图谱项目甲"), "拓扑项目", nil)
	projectB := graphTestMeta(t, s, mustNote(t, s, "图谱项目乙"), "拓扑项目", nil)
	tagA := graphTestMeta(t, s, mustNote(t, s, "图谱标签甲"), "", []string{"拓扑标签"})
	tagB := graphTestMeta(t, s, mustNote(t, s, "图谱标签乙"), "", []string{"拓扑标签"})

	edges, err := s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Note{target, wiki, reference} {
		if _, ok := graphTestEdge(edges, "MARKDOWN_LINK", source.ID, want.ID); !ok {
			t.Fatalf("missing parsed link %s -> %s: %+v", source.Title, want.Title, edges)
		}
	}
	if links := func() int {
		count := 0
		for _, e := range edges {
			if e.Type == "MARKDOWN_LINK" && e.Source == source.ID {
				count++
			}
		}
		return count
	}(); links != 3 {
		t.Fatalf("duplicate or unresolved links created %d edges: %+v", links, edges)
	}
	for _, edge := range edges {
		if edge.Type == "SAME_PROJECT" || edge.Type == "SAME_TOPIC" {
			t.Fatalf("unconfirmed inferred relation displayed: %+v", edge)
		}
	}

	known := map[string]bool{}
	for _, n := range []Note{source, target, wiki, reference, projectA, projectB, tagA, tagB} {
		known[n.ID] = true
	}
	if len(edges) != 3 {
		t.Fatalf("unexpected structural edges: %+v", edges)
	}
	for _, e := range edges {
		if e.Source == e.Target {
			t.Fatalf("self relation: %+v", e)
		}
		if !known[e.Source] || !known[e.Target] {
			t.Fatalf("relation references an unknown note: %+v", e)
		}
	}
}

func TestGraphManualRelationsValidateConflictAndRemove(t *testing.T) {
	s, w := fixture(t)
	a := mustNote(t, s, "人工关联甲")
	b := mustNote(t, s, "人工关联乙")
	before := workspaceFile(t, w.Path, a.Path)

	valid := GraphRelationInput{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true}
	invalid := []GraphRelationInput{
		{WorkspaceID: "missing", Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true},
		{WorkspaceID: w.ID, Source: a.ID, Target: a.ID, SourceHash: a.Hash, TargetHash: a.Hash, Confirmed: true},
		{WorkspaceID: w.ID, Source: "", Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true},
		{WorkspaceID: w.ID, Source: a.ID, Target: "missing", SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true},
		{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, TargetHash: b.Hash, Confirmed: true},
		{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: "stale", Confirmed: true},
	}
	for i, in := range invalid {
		if err := s.SetGraphRelation(in); err == nil {
			t.Fatalf("invalid relation %d was accepted: %+v", i, in)
		}
	}
	edges, err := s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 0 {
		t.Fatalf("rejected relation was persisted: %+v", edges)
	}

	if err = s.SetGraphRelation(valid); err != nil {
		t.Fatal(err)
	}
	edges, err = s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 1 {
		t.Fatalf("confirmed relation missing: %+v", edges)
	}
	// The UI treats a relation as undirected; confirming the reverse must not duplicate it.
	if err = s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: b.ID, Target: a.ID, SourceHash: b.Hash, TargetHash: a.Hash, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	edges, err = s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 1 {
		t.Fatalf("reverse confirmation duplicated the relation: %+v", edges)
	}
	if got := workspaceFile(t, w.Path, a.Path); got != before {
		t.Fatal("confirming a relation modified note content")
	}

	// A confirmation fetched before an edit must not overwrite the newer relation state.
	staleHash := a.Hash
	a = graphTestAppend(t, s, a, "\n推理与确认期间的正文修改。\n")
	stale := GraphRelationInput{WorkspaceID: w.ID, Source: b.ID, Target: a.ID, SourceHash: b.Hash, TargetHash: staleHash, Confirmed: false}
	if err = s.SetGraphRelation(stale); err == nil {
		t.Fatal("stale hashes removed a relation")
	}
	edges, err = s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 1 {
		t.Fatalf("stale request changed the relation: %+v", edges)
	}

	remove := GraphRelationInput{WorkspaceID: w.ID, Source: b.ID, Target: a.ID, SourceHash: b.Hash, TargetHash: a.Hash, Confirmed: false}
	if err = s.SetGraphRelation(remove); err != nil {
		t.Fatal(err)
	}
	edges, err = s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 0 {
		t.Fatalf("removed relation still exists: %+v", edges)
	}
	if err = s.SetGraphRelation(remove); err != nil {
		t.Fatalf("removing an absent relation must be idempotent: %v", err)
	}
}

func TestGraphRelationsPersistAcrossRestartWorkspaceAndBackup(t *testing.T) {
	s, w := fixture(t)
	a := mustNote(t, s, "持久关联甲")
	b := mustNote(t, s, "持久关联乙")
	if err := s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true}); err != nil {
		t.Fatal(err)
	}

	other, err := s.AddWorkspace("关联隔离", filepath.Join(t.TempDir(), "other"))
	if err != nil {
		t.Fatal(err)
	}
	// A delayed confirmation for the previous workspace must be rejected.
	if err = s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true}); err == nil {
		t.Fatal("relation was written while another workspace was active")
	}
	c := mustNote(t, s, "隔离关联甲")
	d := mustNote(t, s, "隔离关联乙")
	if err = s.SetGraphRelation(GraphRelationInput{WorkspaceID: other.ID, Source: c.ID, Target: d.ID, SourceHash: c.Hash, TargetHash: d.Hash, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	edges, err := s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", c.ID, d.ID) != 1 {
		t.Fatalf("other workspace relation missing: %+v", edges)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 0 {
		t.Fatalf("relation leaked between workspaces: %+v", edges)
	}

	if err = s.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	edges, err = s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 1 || graphTestBetween(edges, "USER_CONFIRMED", c.ID, d.ID) != 0 {
		t.Fatalf("workspace relation set is wrong: %+v", edges)
	}

	// Read from a fresh service so the test exercises relations.json, not memory.
	fresh, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if err = fresh.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	edges, err = fresh.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 1 {
		t.Fatalf("restart lost the relation: %+v", edges)
	}
	if err = fresh.OpenWorkspace(other.ID); err != nil {
		t.Fatal(err)
	}
	edges, err = fresh.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", c.ID, d.ID) != 1 || graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 0 {
		t.Fatalf("restart mixed workspace relations: %+v", edges)
	}

	if err = fresh.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "graph-relations.zip")
	if err = fresh.Export(archive, true); err != nil {
		t.Fatal(err)
	}
	restored, err := fresh.Import(archive, filepath.Join(t.TempDir(), "restored"))
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID == "" {
		t.Fatal("restored workspace has no identity")
	}
	edges, err = fresh.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 1 || graphTestBetween(edges, "USER_CONFIRMED", c.ID, d.ID) != 0 {
		t.Fatalf("native backup lost or mixed manual relations: %+v", edges)
	}
}

func TestGraphRelationsIgnoreMissingNotesAndRejectInvalidStore(t *testing.T) {
	s, w := fixture(t)
	a := mustNote(t, s, "关系存储甲")
	b := mustNote(t, s, "关系存储乙")

	edges, err := s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 0 {
		t.Fatalf("empty store produced relations: %+v", edges)
	}

	path := filepath.Join(s.data(w), "relations.json")
	items := []Relation{{Source: a.ID, Target: b.ID}, {Source: a.ID, Target: b.ID}, {Source: "deleted-note", Target: b.ID}}
	if err = writeJSON(path, items); err != nil {
		t.Fatal(err)
	}
	edges, err = s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 1 {
		t.Fatalf("duplicate stored relations were not collapsed: %+v", edges)
	}
	for _, e := range edges {
		if e.Source == "deleted-note" || e.Target == "deleted-note" {
			t.Fatalf("relation to a missing note was exposed: %+v", e)
		}
	}

	// A corrupt store is reported and must not be overwritten by a new confirmation.
	if err = writeJSON(path, []Relation{{Source: a.ID, Target: a.ID}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Graph(); err == nil {
		t.Fatal("invalid relation store was accepted by Graph")
	}
	if err = s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true}); err == nil {
		t.Fatal("invalid relation store was overwritten")
	}
	persisted := []Relation{}
	if err = readJSON(path, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].Source != a.ID || persisted[0].Target != a.ID {
		t.Fatalf("rejected confirmation changed the corrupt store: %+v", persisted)
	}

	if err = writeJSON(path, []Relation{}); err != nil {
		t.Fatal(err)
	}
	edges, err = s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if graphTestBetween(edges, "USER_CONFIRMED", a.ID, b.ID) != 0 {
		t.Fatalf("cleared relation store still produced relations: %+v", edges)
	}
}

func TestSuggestGraphRelationsFocusHonorsThresholdLimitAndHashes(t *testing.T) {
	s, w := fixture(t)
	focus := mustNote(t, s, "图谱语义焦点")
	same := mustNote(t, s, "图谱语义同向")
	near := mustNote(t, s, "图谱语义接近")
	closeA := mustNote(t, s, "图谱语义近邻甲")
	closeB := mustNote(t, s, "图谱语义近邻乙")
	focusFile := workspaceFile(t, w.Path, focus.Path)
	s.embedTexts = graphTestEmbeddings(map[string][]float64{
		focus.Title:  graphTestVector(1),
		same.Title:   graphTestVector(1),
		near.Title:   graphTestVector(.8, .6),
		closeA.Title: graphTestVector(.6, .8),
		closeB.Title: graphTestVector(.5, .8660254037844386),
	})
	notes := map[string]Note{focus.ID: focus, same.ID: same, near.ID: near, closeA.ID: closeA, closeB.ID: closeB}

	got, err := s.SuggestGraphRelations(w.ID, focus.ID, .9)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Edges) != 1 || got.Edges[0].Target != same.ID || got.Edges[0].Score < .99 {
		t.Fatalf("threshold did not isolate the identical note: %+v", got.Edges)
	}
	for _, e := range got.Edges {
		if e.Type != "SEMANTIC_SIMILAR" || e.Source != focus.ID {
			t.Fatalf("unexpected suggestion kind or direction: %+v", e)
		}
		if e.SourceHash != notes[e.Source].Hash || e.TargetHash != notes[e.Target].Hash {
			t.Fatalf("suggestion is missing the hashes needed for confirmation: %+v", e)
		}
	}

	got, err = s.SuggestGraphRelations(w.ID, focus.ID, .79)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Edges) != 2 || got.Edges[0].Target != same.ID || got.Edges[1].Target != near.ID || got.Edges[0].Score < got.Edges[1].Score {
		t.Fatalf("near matches were not ordered by similarity: %+v", got.Edges)
	}

	// A focused note never returns more than three recommendations, even when
	// four candidates are above the threshold.
	got, err = s.SuggestGraphRelations(w.ID, focus.ID, .49)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Edges) != 3 {
		t.Fatalf("expected at most three focused suggestions, got %+v", got.Edges)
	}
	targets := map[string]bool{}
	for _, e := range got.Edges {
		if e.Source != focus.ID || e.Target == focus.ID {
			t.Fatalf("invalid focused suggestion: %+v", e)
		}
		if targets[e.Target] {
			t.Fatalf("duplicate focused suggestion: %+v", got.Edges)
		}
		targets[e.Target] = true
		if e.SourceHash != focus.Hash || e.TargetHash != notes[e.Target].Hash {
			t.Fatalf("focused suggestion has stale hashes: %+v", e)
		}
	}

	if _, err = s.SuggestGraphRelations(w.ID, focus.ID, 0); err != nil {
		t.Fatalf("lower threshold boundary was rejected: %v", err)
	}
	if _, err = s.SuggestGraphRelations(w.ID, focus.ID, 1); err != nil {
		t.Fatalf("upper threshold boundary was rejected: %v", err)
	}
	if _, err = s.SuggestGraphRelations(w.ID, "missing", .5); err == nil {
		t.Fatal("missing focus note was accepted")
	}
	if _, err = s.SuggestGraphRelations("missing", focus.ID, .5); err == nil {
		t.Fatal("wrong workspace was accepted")
	}
	for _, threshold := range []float64{-.01, 1.01, math.NaN(), math.Inf(1)} {
		if _, err = s.SuggestGraphRelations(w.ID, focus.ID, threshold); err == nil {
			t.Fatalf("invalid threshold %v was accepted", threshold)
		}
	}
	if got := workspaceFile(t, w.Path, focus.Path); got != focusFile {
		t.Fatal("graph suggestions modified source note content")
	}
}

func TestSuggestGraphRelationsNeedsTwoNotes(t *testing.T) {
	s, w := fixture(t)
	mustNote(t, s, "唯一笔记")
	s.embedTexts = func([]string) ([][]float64, error) {
		return nil, errors.New("embedding must not run for a single note")
	}

	got, err := s.SuggestGraphRelations(w.ID, "", .5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Edges) != 0 || got.Message == "" {
		t.Fatalf("single note should explain why there are no suggestions: %+v", got)
	}
}

func TestGraphSuggestionsReuseVectorsAndRecomputeEditedNote(t *testing.T) {
	s, w := fixture(t)
	a, b := mustNote(t, s, "缓存来源"), mustNote(t, s, "缓存目标")
	computed := 0
	s.embedTexts = func(texts []string) ([][]float64, error) {
		computed += len(texts)
		out := make([][]float64, len(texts))
		for i := range out {
			out[i] = graphTestVector(1)
		}
		return out, nil
	}
	for i := 0; i < 2; i++ {
		if _, err := s.SuggestGraphRelations(w.ID, a.ID, .65); err != nil {
			t.Fatal(err)
		}
	}
	if computed != 2 {
		t.Fatalf("unchanged notes recomputed: %d", computed)
	}
	b = graphTestAppend(t, s, b, "\n更新后的正文")
	if _, err := s.SuggestGraphRelations(w.ID, a.ID, .65); err != nil {
		t.Fatal(err)
	}
	if computed != 3 {
		t.Fatalf("edited note not incrementally recomputed: %d", computed)
	}
	// A failed fresh inference must report failure instead of reusing stale vectors.
	graphTestAppend(t, s, b, "\n再次修改")
	s.embedTexts = func([]string) ([][]float64, error) { return nil, errors.New("offline model failure") }
	if _, err := s.SuggestGraphRelations(w.ID, a.ID, .65); err == nil {
		t.Fatal("model failure hidden")
	}
}

func TestSuggestGraphRelationsGlobalDeduplicatesAndSkipsConfirmed(t *testing.T) {
	s, w := fixture(t)
	notes := []Note{
		mustNote(t, s, "图谱全局甲"),
		mustNote(t, s, "图谱全局乙"),
		mustNote(t, s, "图谱全局丙"),
		mustNote(t, s, "图谱全局丁"),
		mustNote(t, s, "图谱全局戊"),
	}
	vectors := map[string][]float64{}
	for _, n := range notes {
		vectors[n.Title] = graphTestVector(1)
	}
	s.embedTexts = graphTestEmbeddings(vectors)

	before, err := s.SuggestGraphRelations(w.ID, "", .9)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Edges) == 0 {
		t.Fatal("identical notes produced no global suggestions")
	}
	known := map[string]Note{}
	for _, n := range notes {
		known[n.ID] = n
	}
	seen := map[string]bool{}
	for _, e := range before.Edges {
		if e.Type != "SEMANTIC_SIMILAR" || e.Source == e.Target {
			t.Fatalf("invalid global suggestion: %+v", e)
		}
		if _, ok := known[e.Source]; !ok {
			t.Fatalf("suggestion has unknown source: %+v", e)
		}
		if _, ok := known[e.Target]; !ok {
			t.Fatalf("suggestion has unknown target: %+v", e)
		}
		if e.SourceHash != known[e.Source].Hash || e.TargetHash != known[e.Target].Hash {
			t.Fatalf("suggestion has stale hashes: %+v", e)
		}
		if e.Score < 0 || e.Score > 1 {
			t.Fatalf("similarity outside [0,1]: %+v", e)
		}
		key := graphTestPair(e.Source, e.Target)
		if seen[key] {
			t.Fatalf("global suggestion duplicated a pair: %+v", before.Edges)
		}
		seen[key] = true
	}

	first := before.Edges[0]
	if err = s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: first.Source, Target: first.Target, SourceHash: first.SourceHash, TargetHash: first.TargetHash, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	after, err := s.SuggestGraphRelations(w.ID, "", .9)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range after.Edges {
		if graphTestPair(e.Source, e.Target) == graphTestPair(first.Source, first.Target) {
			t.Fatalf("confirmed relation was suggested again: %+v", e)
		}
	}
	if len(after.Edges) == 0 {
		t.Fatal("confirming one candidate removed all unrelated suggestions")
	}
}

func TestSuggestGraphRelationsDropsConfirmationMadeDuringInference(t *testing.T) {
	s, w := fixture(t)
	focus := mustNote(t, s, "推理焦点")
	peer := mustNote(t, s, "推理同伴")
	other := mustNote(t, s, "推理其他")
	base := graphTestEmbeddings(map[string][]float64{
		focus.Title: graphTestVector(1),
		peer.Title:  graphTestVector(1),
		other.Title: graphTestVector(1),
	})
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	s.embedTexts = func(texts []string) ([][]float64, error) {
		once.Do(func() { close(started) })
		<-release
		return base(texts)
	}
	type result struct {
		edges []Edge
		err   error
	}
	done := make(chan result, 1)
	go func() {
		got, err := s.SuggestGraphRelations(w.ID, focus.ID, .5)
		done <- result{got.Edges, err}
	}()
	select {
	case <-started:
	case res := <-done:
		t.Fatalf("suggestion finished before inference: %+v", res)
	case <-time.After(15 * time.Second):
		close(release)
		<-done
		t.Fatal("suggestion never reached inference")
	}

	if err := s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: focus.ID, Target: peer.ID, SourceHash: focus.Hash, TargetHash: peer.Hash, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	close(release)
	res := <-done
	if res.err != nil {
		t.Fatal(res.err)
	}
	if graphTestBetween(res.edges, "SEMANTIC_SIMILAR", focus.ID, peer.ID) != 0 {
		t.Fatalf("confirmation made during inference was suggested back: %+v", res.edges)
	}
	if graphTestBetween(res.edges, "SEMANTIC_SIMILAR", focus.ID, other.ID) != 1 {
		t.Fatalf("unconfirmed candidate was lost: %+v", res.edges)
	}
}

func TestSuggestGraphRelationsRejectsWorkspaceSwitchDuringInference(t *testing.T) {
	s, w := fixture(t)
	focus := mustNote(t, s, "切换焦点")
	peer := mustNote(t, s, "切换同伴")
	other := mustNote(t, s, "切换其他")
	base := graphTestEmbeddings(map[string][]float64{
		focus.Title: graphTestVector(1),
		peer.Title:  graphTestVector(1),
		other.Title: graphTestVector(1),
	})
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	s.embedTexts = func(texts []string) ([][]float64, error) {
		once.Do(func() { close(started) })
		<-release
		return base(texts)
	}
	type result struct {
		edges []Edge
		err   error
	}
	done := make(chan result, 1)
	go func() {
		got, err := s.SuggestGraphRelations(w.ID, focus.ID, .5)
		done <- result{got.Edges, err}
	}()
	select {
	case <-started:
	case res := <-done:
		t.Fatalf("suggestion finished before inference: %+v", res)
	case <-time.After(15 * time.Second):
		close(release)
		<-done
		t.Fatal("suggestion never reached inference")
	}

	if _, err := s.AddWorkspace("切换后的工作区", filepath.Join(t.TempDir(), "switched")); err != nil {
		t.Fatal(err)
	}
	close(release)
	res := <-done
	if res.err == nil {
		t.Fatal("suggestions from the previous workspace were returned")
	}
	if len(res.edges) != 0 {
		t.Fatalf("stale workspace suggestions leaked: %+v", res.edges)
	}
}

func TestSuggestGraphRelationsDropsNotesEditedDuringInference(t *testing.T) {
	s, w := fixture(t)
	a := mustNote(t, s, "编辑推理甲")
	b := mustNote(t, s, "编辑推理乙")
	c := mustNote(t, s, "编辑推理丙")
	d := mustNote(t, s, "编辑推理丁")
	base := graphTestEmbeddings(map[string][]float64{
		a.Title: graphTestVector(1),
		b.Title: graphTestVector(1),
		c.Title: graphTestVector(1),
		d.Title: graphTestVector(1),
	})
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	s.embedTexts = func(texts []string) ([][]float64, error) {
		once.Do(func() { close(started) })
		<-release
		return base(texts)
	}
	type result struct {
		edges []Edge
		err   error
	}
	done := make(chan result, 1)
	go func() {
		got, err := s.SuggestGraphRelations(w.ID, "", .5)
		done <- result{got.Edges, err}
	}()
	select {
	case <-started:
	case res := <-done:
		t.Fatalf("suggestion finished before inference: %+v", res)
	case <-time.After(15 * time.Second):
		close(release)
		<-done
		t.Fatal("suggestion never reached inference")
	}

	c = graphTestAppend(t, s, c, "\n推理期间用户修改了这篇笔记。\n")
	close(release)
	res := <-done
	if res.err != nil {
		t.Fatal(res.err)
	}
	for _, e := range res.edges {
		if e.Source == c.ID || e.Target == c.ID {
			t.Fatalf("stale suggestion for an edited note survived: %+v", e)
		}
	}
	if graphTestBetween(res.edges, "SEMANTIC_SIMILAR", a.ID, b.ID) != 1 {
		t.Fatalf("unrelated fresh suggestion was dropped: %+v", res.edges)
	}
}

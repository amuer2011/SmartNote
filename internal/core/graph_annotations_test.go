package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphAnnotationsConflictMoveBackupAndSync(t *testing.T) {
	s, w := fixture(t)
	a, b := mustNote(t, s, "备注来源"), mustNote(t, s, "备注目标")
	before := workspaceFile(t, w.Path, a.Path)
	metadata, e := s.GetGraphAnnotations(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	metadata, e = s.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: w.ID, Revision: metadata.Revision, NoteID: a.ID, Text: "笔记说明"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: w.ID, NoteID: a.ID, Text: "覆盖"}); e == nil {
		t.Fatal("stale revision accepted")
	}
	if _, e = s.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: w.ID, Revision: metadata.Revision, Source: a.ID, Target: b.ID, Text: "不存在"}); e == nil {
		t.Fatal("nonexistent edge annotated")
	}
	if e = s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	metadata, e = s.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: w.ID, Revision: metadata.Revision, Source: a.ID, Target: b.ID, Text: "补充依据"})
	if e != nil {
		t.Fatal(e)
	}
	if workspaceFile(t, w.Path, a.Path) != before {
		t.Fatal("annotation changed Markdown")
	}
	edges, e := s.Graph()
	if e != nil || len(edges) != 1 || edges[0].Annotation != "补充依据" {
		t.Fatal(edges, e)
	}
	if _, e = s.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: w.ID, Revision: metadata.Revision, NoteID: a.ID, Text: strings.Repeat("字", 4001)}); e == nil {
		t.Fatal("oversized annotation accepted")
	}
	if _, e = s.MoveNote(a.ID, a.Hash, "知识/备注来源.md"); e != nil {
		t.Fatal(e)
	}
	files, e := s.syncSnapshot(w)
	if e != nil || len(files["metadata/graph-annotations.json"]) == 0 {
		t.Fatal("sync omitted annotations", e)
	}
	backup := filepath.Join(t.TempDir(), "graph.zip")
	if e = s.Export(backup, true); e != nil {
		t.Fatal(e)
	}
	restored, e := s.Import(backup, filepath.Join(t.TempDir(), "restored"))
	if e != nil {
		t.Fatal(e)
	}
	metadata, e = s.GetGraphAnnotations(restored.ID)
	if e != nil || metadata.Nodes[a.ID] != "笔记说明" || metadata.Relations[relationKey(a.ID, b.ID)] != "补充依据" {
		t.Fatal(metadata, e)
	}
	if _, e = s.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: w.ID, Revision: metadata.Revision, NoteID: a.ID, Text: "错误空间"}); e == nil {
		t.Fatal("wrong workspace accepted")
	}
	metadata, e = s.SaveGraphAnnotation(GraphAnnotationInput{WorkspaceID: restored.ID, Revision: metadata.Revision, NoteID: a.ID, Text: ""})
	if e != nil || metadata.Nodes[a.ID] != "" {
		t.Fatal("clearing annotation failed", metadata, e)
	}
}

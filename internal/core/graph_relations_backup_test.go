package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfirmedGraphRelationNativeBackupAndMove(t *testing.T) {
	s, w := fixture(t)
	a := mustNote(t, s, "关联来源")
	b := mustNote(t, s, "关联目标")
	if err := s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	moved, err := s.MoveNote(b.ID, b.Hash, "知识/关联目标.md")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Note.ID != b.ID {
		t.Fatal("move changed note identity")
	}
	backup := filepath.Join(t.TempDir(), "relations.zip")
	if err = s.Export(backup, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Import(backup, filepath.Join(t.TempDir(), "restored")); err != nil {
		t.Fatal(err)
	}
	edges, err := s.Graph()
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || edges[0].Type != "USER_CONFIRMED" || relationKey(edges[0].Source, edges[0].Target) != relationKey(a.ID, b.ID) {
		t.Fatalf("confirmation lost in backup: %+v", edges)
	}
}

func TestGraphRejectsCorruptRelationMetadataWithoutOverwrite(t *testing.T) {
	s, w := fixture(t)
	a, b := mustNote(t, s, "来源"), mustNote(t, s, "目标")
	path := filepath.Join(s.data(w), "relations.json")
	bad := []byte(`[{"source":"same","target":"same"}]`)
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Graph(); err == nil {
		t.Fatal("invalid graph metadata accepted")
	}
	if err := s.SetGraphRelation(GraphRelationInput{WorkspaceID: w.ID, Source: a.ID, Target: b.ID, SourceHash: a.Hash, TargetHash: b.Hash, Confirmed: true}); err == nil {
		t.Fatal("corrupt metadata overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(bad) {
		t.Fatal("corrupt metadata was changed")
	}
	if err = validateMetadata(s.data(w)); err == nil {
		t.Fatal("invalid relation backup metadata accepted")
	}
}

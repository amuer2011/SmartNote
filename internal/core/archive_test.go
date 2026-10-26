package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func archiveTestRule(id, dir string) ArchiveRule {
	return ArchiveRule{ID: id, Name: id, Enabled: true, Field: "title", Match: "any", Keywords: []string{"数据库"}, Directory: dir, Tags: []string{"数据库", "排查"}}
}
func TestArchiveRulesPersistenceIsolationAndConflicts(t *testing.T) {
	s, w := fixture(t)
	config, err := s.GetArchiveRules(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	config.Rules = []ArchiveRule{archiveTestRule("first", "工作/数据库"), archiveTestRule("second", "备用")}
	saved, err := s.SaveArchiveRules(w.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveArchiveRules(w.ID, config); err == nil {
		t.Fatal("stale config accepted")
	}
	invalid := saved
	invalid.Rules = []ArchiveRule{archiveTestRule("bad", "../outside")}
	if _, err = s.SaveArchiveRules(w.ID, invalid); err == nil {
		t.Fatal("traversal accepted")
	}
	got, err := s.GetArchiveRules(w.ID)
	if err != nil || !reflect.DeepEqual(got, saved) {
		t.Fatalf("rules changed after rejected writes: %+v %v", got, err)
	}
	second, err := s.AddWorkspace("other", filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetArchiveRules(w.ID); err == nil {
		t.Fatal("inactive workspace access accepted")
	}
	other, err := s.GetArchiveRules(second.ID)
	if err != nil || len(other.Rules) != 0 {
		t.Fatalf("workspace rules leaked: %+v %v", other, err)
	}
	if err = s.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	// Native backup is the authoritative migration path for rules.
	zipPath := filepath.Join(t.TempDir(), "native.zip")
	if err = s.Export(zipPath, true); err != nil {
		t.Fatal(err)
	}
	restored, err := s.Import(zipPath, filepath.Join(t.TempDir(), "restored"))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := s.GetArchiveRules(restored.ID)
	if err != nil || !reflect.DeepEqual(rules.Rules, saved.Rules) {
		t.Fatalf("backup lost rules: %+v %v", rules, err)
	}
}
func TestArchiveSuggestionPriorityEvidenceAndReadOnly(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "数据库排查")
	first := archiveTestRule("first", "工作/数据库")
	first.Match = "all"
	first.Keywords = []string{"数据库", "不存在"}
	second := archiveTestRule("second", "归档")
	third := archiveTestRule("third", "别处")
	_, err := s.SaveArchiveRules(w.ID, ArchiveRulesConfig{Rules: []ArchiveRule{first, second, third}})
	if err != nil {
		t.Fatal(err)
	}
	suggestion, err := s.SuggestArchive(w.ID, n.ID, n.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.RuleID != "second" || suggestion.Directory != "归档" || len(suggestion.Tags) != 2 || len(suggestion.Reasons) == 0 {
		t.Fatalf("wrong suggestion: %+v", suggestion)
	}
	if got := workspaceFile(t, w.Path, n.Path); got != n.Content {
		t.Fatal("suggestion modified note")
	}
	if _, err = s.SuggestArchive(w.ID, n.ID, "stale"); err == nil {
		t.Fatal("stale suggestions accepted")
	}
	config, _ := s.GetArchiveRules(w.ID)
	config.Rules = nil
	if _, err = s.SaveArchiveRules(w.ID, config); err != nil {
		t.Fatal(err)
	}
	peer, err := s.CreateNote("同项目参考", "项目/文档")
	if err != nil {
		t.Fatal(err)
	}
	peer, err = s.SaveNote(SaveInput{Path: peer.Path, Hash: peer.Hash, Content: strings.Replace(peer.Content, "tags: []", "project: SmartNote\ntags: [数据库]", 1)})
	if err != nil {
		t.Fatal(err)
	}
	n, err = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: strings.Replace(n.Content, "tags: []", "project: SmartNote\ntags: []", 1)})
	if err != nil {
		t.Fatal(err)
	}
	suggestion, err = s.SuggestArchive(w.ID, n.ID, n.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.Directory != "项目/文档" || suggestion.Source != "metadata" || !reflect.DeepEqual(suggestion.Tags, []string{"数据库"}) {
		t.Fatalf("metadata evidence ignored: %+v", suggestion)
	}
}
func TestArchiveApplyAndUndoPreserveContentIdentityAndLinks(t *testing.T) {
	s, w := fixture(t)
	source := mustNote(t, s, "数据库排查")
	other := mustNote(t, s, "相关")
	source, err := s.SaveNote(SaveInput{Path: source.Path, Hash: source.Hash, Content: source.Content + "[相关](相关.md)\n- [ ] 排查任务\n"})
	if err != nil {
		t.Fatal(err)
	}
	other, err = s.SaveNote(SaveInput{Path: other.Path, Hash: other.Hash, Content: other.Content + "[来源](数据库排查.md)\n[[数据库排查]]\n"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyArchive(ArchiveInput{WorkspaceID: w.ID, NoteID: source.ID, Hash: source.Hash, Directory: "工作/数据库", Tags: []string{"数据库", "数据库"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Note.Path != "工作/数据库/数据库排查.md" || result.Note.ID != source.ID || result.OperationID == "" || result.UpdatedLinks != 3 {
		t.Fatalf("archive failed: %+v", result)
	}
	if !reflect.DeepEqual(result.Note.Tags, []string{"数据库"}) || !strings.Contains(result.Note.Content, "../../Inbox/") || !strings.Contains(result.Note.Content, "- [ ] 排查任务") {
		t.Fatalf("archive content: %s", result.Note.Content)
	}
	if workspaceFile(t, result.RecoveryPath, "before.md") != source.Content {
		t.Fatal("original recovery copy missing")
	}
	if _, err = os.Stat(filepath.Join(w.Path, filepath.FromSlash(source.Path))); !os.IsNotExist(err) {
		t.Fatal("source not moved")
	}
	if !strings.Contains(snapshotNote(t, s, other.ID).Content, "[[工作/数据库/数据库排查]]") {
		t.Fatal("backlink not updated")
	}
	undone, err := s.UndoArchive(w.ID, result.OperationID, result.Note.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if undone.Note.Path != source.Path || undone.Note.ID != source.ID || len(undone.Note.Tags) != 0 || !strings.Contains(undone.Note.Content, "[相关](%E7%9B%B8%E5%85%B3.md)") {
		t.Fatalf("undo failed: %+v", undone)
	}
	if !strings.Contains(snapshotNote(t, s, other.ID).Content, "[[Inbox/数据库排查]]") {
		t.Fatal("undo did not update backlink")
	}
	if _, err = s.UndoArchive(w.ID, result.OperationID, undone.Note.Hash); err == nil {
		t.Fatal("double undo accepted")
	}
}
func TestArchiveRefusesCollisionStaleChangesAndUnsafeMetadata(t *testing.T) {
	s, w := fixture(t)
	n := mustNote(t, s, "数据库")
	other, err := s.CreateNote("数据库", "归档")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyArchive(ArchiveInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Directory: "归档", Tags: []string{"新增"}}); err == nil {
		t.Fatal("archive overwrote collision")
	}
	if workspaceFile(t, w.Path, n.Path) != n.Content || workspaceFile(t, w.Path, other.Path) != other.Content {
		t.Fatal("failed archive changed files")
	}
	result, err := s.ApplyArchive(ArchiveInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Directory: "Inbox", Tags: []string{"新增"}})
	if err != nil {
		t.Fatal(err)
	}
	external := result.Note.Content + "外部编辑\n"
	if err = os.WriteFile(filepath.Join(w.Path, filepath.FromSlash(n.Path)), []byte(external), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UndoArchive(w.ID, result.OperationID, result.Note.Hash); err == nil {
		t.Fatal("undo overwrote newer edit")
	}
	if _, err = s.ApplyArchive(ArchiveInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: result.Note.Hash, Directory: "其他"}); err == nil {
		t.Fatal("archive overwrote newer edit")
	}
	if workspaceFile(t, w.Path, n.Path) != external {
		t.Fatal("external content lost")
	}
	broken := parseNote("bad.md", []byte("---\ntags: [broken\n---\nbody"), n.Updated)
	if _, err = archiveTags(broken, []string{"tag"}); err == nil {
		t.Fatal("malformed YAML accepted")
	}
}
func TestArchiveTagsPreserveCRLFBodyCustomMetadataAndSyntheticID(t *testing.T) {
	s, w := fixture(t)
	body := "\r\n# 原文\r\n\r\n```go\r\nline\r\n```\r\n"
	text := "---\r\nid: ''\r\ncustom: {key: value}\r\n# 保留备注\r\ntags: [原标签]\r\n---\r\n" + body
	if err := os.WriteFile(filepath.Join(w.Path, "外部.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	n := snap.Notes[0]
	result, err := s.ApplyArchive(ArchiveInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Directory: ".", Tags: []string{"新标签"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Note.ID != n.ID || result.Note.SyntheticID || !strings.HasSuffix(result.Note.Content, body) || !strings.Contains(result.Note.Content, "# 保留备注") || !strings.Contains(result.Note.Content, "key: value") {
		t.Fatalf("metadata/body/identity damaged: %+v", result.Note)
	}
	if _, err = s.ApplyArchive(ArchiveInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: result.Note.Hash, Directory: ".", Tags: []string{"新标签"}}); err == nil {
		t.Fatal("no-op not rejected")
	}
}

func TestArchiveRulesRemainEditableWhenTargetBecomesAFile(t *testing.T) {
	s, w := fixture(t)
	config, err := s.SaveArchiveRules(w.ID, ArchiveRulesConfig{Rules: []ArchiveRule{archiveTestRule("rule", "目标")}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(w.Path, "目标"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.GetArchiveRules(w.ID)
	if err != nil || loaded.Revision != config.Revision {
		t.Fatalf("cannot edit broken target: %v", err)
	}
	loaded.Rules[0].Directory = "其他目录"
	if _, err = s.SaveArchiveRules(w.ID, loaded); err != nil {
		t.Fatal(err)
	}
	if workspaceFile(t, w.Path, "目标") != "keep" {
		t.Fatal("existing file was changed")
	}
}

package core

import (
	"errors"
	"path/filepath"
	"smartnote/internal/embedding"
	"strings"
	"testing"
)

func semanticFixture(t *testing.T, vectors [][]float64) (*Service, Workspace, SemanticConfig) {
	t.Helper()
	s, w := fixture(t)
	s.embedTexts = func(texts []string) ([][]float64, error) {
		if len(texts) != 3 {
			t.Error("wrong embedding input count")
		}
		return vectors, nil
	}
	c, _ := s.GetSemanticConfig(w.ID)
	c.Automatic = true
	c.Categories = []SemanticCategory{{ID: "db", Name: "数据库", Description: "SQL数据库", Directory: "技术/数据库", Enabled: true}, {ID: "life", Name: "生活", Description: "日常生活", Directory: "生活", Enabled: true}}
	c, e := s.SaveSemanticConfig(w.ID, c)
	if e != nil {
		t.Fatal(e)
	}
	return s, w, c
}
func TestSemanticAutoArchiveAndUndo(t *testing.T) {
	s, w, _ := semanticFixture(t, [][]float64{{1, 0}, {1, 0}, {0, 1}})
	n := mustNote(t, s, "数据库性能排查")
	n, e := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n数据库性能优化记录，分析慢查询与索引并记录本次排查过程和解决办法。"})
	if e != nil {
		t.Fatal(e)
	}
	out, e := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
	if e != nil || out.Source != "semantic" || out.Directory != "技术/数据库" {
		t.Fatalf("%+v %v", out, e)
	}
	if workspaceFile(t, w.Path, n.Path) != n.Content {
		t.Fatal("suggestion wrote note")
	}
	result, e := s.AutoArchive(w.ID, n.ID, n.Hash)
	if e != nil || result == nil || !strings.HasPrefix(result.Note.Path, "技术/数据库/") {
		t.Fatalf("%+v %v", result, e)
	}
	if _, e = s.UndoArchive(w.ID, result.OperationID, result.Note.Hash); e != nil {
		t.Fatal(e)
	}
}
func TestSemanticAmbiguityAndManualDirectory(t *testing.T) {
	s, w, _ := semanticFixture(t, [][]float64{{1, 0}, {1, 0}, {1, 0}})
	n := mustNote(t, s, "不明确")
	out, e := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
	if e != nil || out.Source != "semantic" || out.Directory != "技术/数据库" {
		t.Fatalf("%+v %v", out, e)
	}
	manual, e := s.CreateNote("明确位置", "自己指定")
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.AutoArchive(w.ID, manual.ID, manual.Hash)
	if e != nil || r == nil || r.Note.Path != "技术/数据库/明确位置.md" {
		t.Fatalf("title-only note was not classified: %+v %v", r, e)
	}
	if _, e = s.SuggestSemanticArchive(w.ID, n.ID, "stale"); e == nil {
		t.Fatal("stale hash accepted")
	}
}
func TestSemanticConfigCRUDAndBackup(t *testing.T) {
	s, w := fixture(t)
	c, e := s.GetSemanticConfig(w.ID)
	if e != nil || len(c.Categories) < 10 {
		t.Fatal(c, e)
	}
	c.Categories = []SemanticCategory{}
	saved, e := s.SaveSemanticConfig(w.ID, c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveSemanticConfig(w.ID, c); e == nil {
		t.Fatal("stale configuration accepted")
	}
	got, _ := s.GetSemanticConfig(w.ID)
	if len(got.Categories) != 0 {
		t.Fatal("deleted defaults restored")
	}
	bad := saved
	bad.Endpoint = "https://example.com"
	if _, e = s.SaveSemanticConfig(w.ID, bad); e == nil {
		t.Fatal("remote endpoint accepted")
	}
	bad = saved
	bad.Categories = []SemanticCategory{{ID: "bad", Name: "bad", Description: "bad", Directory: "../outside"}}
	if _, e = s.SaveSemanticConfig(w.ID, bad); e == nil {
		t.Fatal("traversal accepted")
	}
	zip := filepath.Join(t.TempDir(), "semantic.zip")
	if e = s.Export(zip, true); e != nil {
		t.Fatal(e)
	}
	restored, e := s.Import(zip, filepath.Join(t.TempDir(), "restore"))
	if e != nil {
		t.Fatal(e)
	}
	got, e = s.GetSemanticConfig(restored.ID)
	if e != nil || got.Revision != saved.Revision || len(got.Categories) != 0 {
		t.Fatal(got, e)
	}
}
func TestSemanticInvalidVectors(t *testing.T) {
	for _, pair := range [][2][]float64{{nil, nil}, {{0, 0}, {1, 0}}, {{1}, {1, 0}}} {
		if _, e := cosine(pair[0], pair[1]); e == nil {
			t.Fatal("bad vector accepted")
		}
	}
}

func TestSemanticConfigChangeDuringInference(t *testing.T) {
	s, w := fixture(t)
	started := make(chan struct{})
	release := make(chan struct{})
	s.embedTexts = func([]string) ([][]float64, error) {
		close(started)
		<-release
		return [][]float64{{1, 0}, {1, 0}, {0, 1}}, nil
	}
	c, _ := s.GetSemanticConfig(w.ID)
	c.Categories = c.Categories[:2]
	c, e := s.SaveSemanticConfig(w.ID, c)
	if e != nil {
		t.Fatal(e)
	}
	n := mustNote(t, s, "推理期间修改配置")
	done := make(chan error, 1)
	go func() { _, e := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash); done <- e }()
	<-started
	// Saving config must succeed while model inference is still blocked.
	c.Automatic = false
	_, e = s.SaveSemanticConfig(w.ID, c)
	close(release)
	if e != nil {
		t.Fatal(e)
	}
	if e = <-done; e == nil || !strings.Contains(e.Error(), "配置已变化") {
		t.Fatalf("stale inference accepted: %v", e)
	}
	if got := workspaceFile(t, w.Path, n.Path); got != n.Content {
		t.Fatal("stale inference modified file")
	}
}
func TestSemanticModelFailureKeepsNote(t *testing.T) {
	s, w := fixture(t)
	s.embedTexts = func([]string) ([][]float64, error) { return nil, errors.New("model load failed") }
	c, _ := s.GetSemanticConfig(w.ID)
	c.Automatic = true
	if _, e := s.SaveSemanticConfig(w.ID, c); e != nil {
		t.Fatal(e)
	}
	n := mustNote(t, s, "模型不存在")
	n, e := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n正文足够长，模型返回错误时必须保留笔记原有目录和正文内容不变。"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.AutoArchive(w.ID, n.ID, n.Hash); e == nil {
		t.Fatal("model error ignored")
	}
	if got := workspaceFile(t, w.Path, n.Path); got != n.Content {
		t.Fatal("model failure modified note")
	}
}

func TestSemanticLegacyOllamaConfigMigration(t *testing.T) {
	s, w := fixture(t)
	c := defaultSemantic()
	c.Endpoint = "http://127.0.0.1:11434"
	c.Model = "bge-m3"
	c.Revision = "legacy"
	c.Automatic = true
	c.Categories = c.Categories[:1]
	c.Categories[0].Name = "自定义分类"
	if e := writeJSON(filepath.Join(s.data(w), "semantic-archive.json"), c); e != nil {
		t.Fatal(e)
	}
	got, e := s.GetSemanticConfig(w.ID)
	if e != nil || got.Endpoint != "" || got.Model != embedding.ModelName || got.Revision != "legacy" || !got.Automatic || got.Categories[0].Name != "自定义分类" {
		t.Fatal(got, e)
	}
	if _, e = s.SaveSemanticConfig(w.ID, got); e != nil {
		t.Fatal(e)
	}
}

func TestAutomaticArchiveDefaultMigrationAndOptOut(t *testing.T) {
	s, w := fixture(t)
	c, e := s.GetSemanticConfig(w.ID)
	if e != nil || !c.Automatic || c.AutomaticVersion != 1 {
		t.Fatal("automatic archive must default on", c, e)
	}
	c.Automatic = false
	c, e = s.SaveSemanticConfig(w.ID, c)
	if e != nil {
		t.Fatal(e)
	}
	got, e := s.GetSemanticConfig(w.ID)
	if e != nil || got.Automatic {
		t.Fatal("explicit opt-out lost", got, e)
	}
	c.AutomaticVersion = 0
	c.Categories = c.Categories[:1]
	c.Categories[0].Name = "用户自定义待办"
	if e = writeJSON(filepath.Join(s.data(w), "semantic-archive.json"), c); e != nil {
		t.Fatal(e)
	}
	got, e = s.GetSemanticConfig(w.ID)
	if e != nil || !got.Automatic || got.Categories[0].Name != "用户自定义待办" {
		t.Fatal("legacy upgrade failed", got, e)
	}
}
func TestAutomaticArchiveRootAndOptOut(t *testing.T) {
	s, w, c := semanticFixture(t, [][]float64{{1, 0}, {1, 0}, {0, 1}})
	n, e := s.CreateNote("根目录自动整理", ".")
	if e != nil {
		t.Fatal(e)
	}
	n, e = s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n数据库SQL查询优化记录，分析索引以及执行计划，记录详细排查过程。"})
	if e != nil {
		t.Fatal(e)
	}
	result, e := s.AutoArchive(w.ID, n.ID, n.Hash)
	if e != nil || result == nil || !strings.HasPrefix(result.Note.Path, "技术/数据库/") {
		t.Fatal(result, e)
	}
	undone, e := s.UndoArchive(w.ID, result.OperationID, result.Note.Hash)
	if e != nil {
		t.Fatal(e)
	}
	c.Automatic = false
	if _, e = s.SaveSemanticConfig(w.ID, c); e != nil {
		t.Fatal(e)
	}
	if result, e = s.AutoArchive(w.ID, undone.Note.ID, undone.Note.Hash); e != nil || result != nil {
		t.Fatal("opt-out ignored", result, e)
	}
	if got := workspaceFile(t, w.Path, undone.Note.Path); got != undone.Note.Content {
		t.Fatal("disabled automatic archiving changed note")
	}
}

func TestAutomaticArchiveNestedDirectoryAndSameTarget(t *testing.T) {
	s, w, _ := semanticFixture(t, [][]float64{{1, 0}, {1, 0}, {0, 1}})
	n, e := s.CreateNote("跨分类整理", "工作/待办/历史")
	if e != nil {
		t.Fatal(e)
	}
	n, e = s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n数据库查询优化记录，分析索引以及执行计划，记录详细排查过程。"})
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.AutoArchive(w.ID, n.ID, n.Hash)
	if e != nil || r == nil || !strings.HasPrefix(r.Note.Path, "技术/数据库/") {
		t.Fatal(r, e)
	}
	again, e := s.AutoArchive(w.ID, r.Note.ID, r.Note.Hash)
	if e != nil || again != nil {
		t.Fatal("same target must not move again", again, e)
	}
}

func TestSemanticRejectionReasons(t *testing.T) {
	cases := []struct {
		name    string
		vectors [][]float64
		want    []string
	}{

		{"similarity", [][]float64{{1, 0}, {0.4, 0.916515139}, {0, 1}}, []string{"相似度不足", "最低余弦相似度调至", "新增分类"}},
		{"both", [][]float64{{1, 0}, {0.4, 0.916515139}, {0.39, 0.920814856}}, []string{"相似度不足", "保留原目录"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, w, _ := semanticFixture(t, tc.vectors)
			n := mustNote(t, s, "分类说明")
			r, e := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
			if e != nil || r.Source != "semantic-low" || r.Directory != "Inbox" {
				t.Fatal(r, e)
			}
			reasons := strings.Join(r.Reasons, "\n")
			for _, want := range tc.want {
				if !strings.Contains(reasons, want) {
					t.Fatalf("missing %s in %s", want, reasons)
				}
			}
		})
	}
}

func TestSemanticMostSimilarIgnoresRunnerUp(t *testing.T) {
	for _, vectors := range [][][]float64{{{1, 0}, {1, 0}, {1, 0}}, {{1, 0}, {0.8, 0.6}, {0.81, 0.586429876}}} {
		s, w, _ := semanticFixture(t, vectors)
		n := mustNote(t, s, "最高分分类")
		r, e := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
		if e != nil || r.Source != "semantic" {
			t.Fatal(r, e)
		}
		want := "技术/数据库"
		if vectors[2][0] > vectors[1][0] {
			want = "生活"
		}
		if r.Directory != want {
			t.Fatal(r)
		}
		if strings.Contains(strings.Join(r.Reasons, ""), "差值") {
			t.Fatal(r.Reasons)
		}
	}
}

func TestTitleOnlyTaskNoteReclassifiedAfterEdit(t *testing.T) {
	s, w, _ := semanticFixture(t, [][]float64{{1, 0}, {1, 0}, {0, 1}})
	n, e := s.CreateNote("10月21日我要出差", "任意/子目录")
	if e != nil {
		t.Fatal(e)
	}
	taskID := tasksFor(n)[0].ID
	result, e := s.AutoArchive(w.ID, n.ID, n.Hash)
	if e != nil || result == nil || noteDirectory(result.Note.Path) != "技术/数据库" {
		t.Fatalf("%+v %v", result, e)
	}
	if result.Note.Content != n.Content || tasksFor(result.Note)[0].ID != taskID {
		t.Fatal("classification altered source or task identity")
	}
	n = result.Note
	n, e = s.SaveNote(SaveInput{Path: n.Path, Hash: n.Hash, Content: n.Content + "\n生活日常出差行程"})
	if e != nil {
		t.Fatal(e)
	}
	s.embedTexts = func(texts []string) ([][]float64, error) { return [][]float64{{0, 1}, {1, 0}, {0, 1}}, nil }
	result, e = s.AutoArchive(w.ID, n.ID, n.Hash)
	if e != nil || result == nil || noteDirectory(result.Note.Path) != "生活" {
		t.Fatalf("edit was not reclassified: %+v %v", result, e)
	}
}

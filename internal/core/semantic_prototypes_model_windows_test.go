//go:build windows && amd64 && cgo

package core

import "testing"

func TestDirectoryPrototypeBundledModel(t *testing.T) {
	s, w := fixture(t)
	c, _ := s.GetSemanticConfig(w.ID)
	c.Threshold = .45
	c.Categories = []SemanticCategory{{ID: "personal", Name: "项目资料", Description: "项目A的工作记录与相关资料", Directory: "工作/项目A", Enabled: true}, {ID: "travel", Name: "旅行", Description: "旅游出行、住宿酒店、景点门票和假期行程", Directory: "生活/旅行", Enabled: true}}
	c, err := s.SaveSemanticConfig(w.ID, c)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := s.CreateNote("Kubernetes Pod容器启动异常，检查启动配置和错误日志", "工作/项目A")
	if err != nil {
		t.Fatal(err)
	}
	n := mustNote(t, s, "排查Pod容器启动失败，检查配置与错误日志")
	before, err := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
	if err != nil {
		t.Fatal(err)
	}
	c.Categories[0].Examples = []SemanticArchiveRef{prototypeRef(sample)}
	if _, err = s.SaveSemanticConfig(w.ID, c); err != nil {
		t.Fatal(err)
	}
	after, err := s.SuggestSemanticArchive(w.ID, n.ID, n.Hash)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("before %.4f (%s); learned %.4f (%s)", before.Score, before.Source, after.Score, after.Source)
	if after.RuleID != "personal" || after.Source != "semantic" || after.Score <= before.Score {
		t.Fatalf("learned prototype failed: before %+v after %+v", before, after)
	}
}

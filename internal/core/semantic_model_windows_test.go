//go:build windows && amd64 && cgo

package core

import "testing"

func TestSemanticBundledModelDefaultCategories(t *testing.T) {
	s, w := fixture(t)
	for _, c := range []struct{ title, body, dir string }{
		{"慢查询排查", "数据库SQL执行很慢，分析执行计划，添加索引并优化数据库查询性能。", "技术/数据库"},
		{"杭州旅行计划", "五一去杭州旅游，预订西湖附近的酒店，安排景点游览路线和高铁交通行程。", "生活/旅行出行"},
		{"研发例会纪要", "参会人员：张三、李四。会议讨论了需求进度。会议决议：本周五前提交方案。行动项：李四整理材料。", "工作/会议"},
		{"体检与健身", "体检报告显示血脂偏高，制定每周跑步和力量训练计划，控制饮食并补充营养。", "生活/健康运动"},
	} {
		n := mustNote(t, s, c.title)
		saved, e := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n" + c.body})
		if e != nil {
			t.Fatal(e)
		}
		out, e := s.SuggestSemanticArchive(w.ID, n.ID, saved.Hash)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("%s => %s (%s) %.3f %v", c.title, out.Directory, out.Source, out.Score, out.Reasons)
		if out.Directory != c.dir || out.Source != "semantic" {
			t.Errorf("expected %s, got %+v", c.dir, out)
		}
	}
}

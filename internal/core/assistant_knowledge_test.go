package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssistantHistoryPersistenceRevisionAndWorkspace(t *testing.T) {
	s, w := fixture(t)
	memory, err := s.SaveAssistantMemory(w.ID, AssistantMemory{Text: "项目：迁移数据库。回答先给结论。"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveAssistantMemory(w.ID, AssistantMemory{Text: "stale"}); err == nil {
		t.Fatal("stale memory replaced current data")
	}
	conversation := AssistantConversation{ID: uid(), Entries: []AssistantEntry{{Role: "user", Content: "继续讨论数据库认证", Sources: []LLMSource{}}, {Role: "assistant", Content: "请结合诊断笔记 [1]。", Sources: []LLMSource{{NoteID: "n", Hash: "h", Title: "诊断笔记", Path: "Inbox/诊断.md"}}}}}
	saved, err := s.SaveAssistantConversation(w.ID, conversation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveAssistantConversation(w.ID, conversation); err == nil {
		t.Fatal("stale conversation overwrote saved data")
	}
	// A fresh service reads both from disk, without any model request.
	fresh, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	read, err := fresh.GetAssistantMemory(w.ID)
	if err != nil || read != memory {
		t.Fatalf("memory not durable: %+v %v", read, err)
	}
	readChat, err := fresh.GetAssistantConversation(w.ID, saved.ID)
	if err != nil || readChat.Revision != saved.Revision || readChat.Entries[1].Sources[0].Hash != "h" {
		t.Fatalf("conversation not durable: %+v %v", readChat, err)
	}
	for _, id := range []string{"../llm-providers", "../../settings", saved.ID + "/..", "C:/test", strings.Repeat("a", 33)} {
		if _, err = s.GetAssistantConversation(w.ID, id); err == nil {
			t.Fatal("invalid conversation path accepted", id)
		}
	}
	if err = s.DeleteAssistantConversation(w.ID, saved.ID, "stale"); err == nil {
		t.Fatal("stale delete accepted")
	}
	w2, err := s.AddWorkspace("另一个工作区", filepath.Join(t.TempDir(), "other"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetAssistantMemory(w.ID); err == nil {
		t.Fatal("cross-workspace memory read")
	}
	if _, err = s.SaveAssistantConversation(w.ID, saved); err == nil {
		t.Fatal("cross-workspace history write")
	}
	list, err := s.ListAssistantConversations(w2.ID)
	if err != nil || len(list) != 0 {
		t.Fatal("workspace history leaked", list, err)
	}
	if err = s.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteAssistantConversation(w.ID, saved.ID, saved.Revision); err != nil {
		t.Fatal(err)
	}
	list, err = s.ListAssistantConversations(w.ID)
	if err != nil || len(list) != 0 {
		t.Fatal("deleted history remains", list, err)
	}
	cleared, err := s.SaveAssistantMemory(w.ID, AssistantMemory{Revision: memory.Revision, Text: ""})
	if err != nil || cleared.Text != "" {
		t.Fatal("memory clear failed", err)
	}
}

func TestAssistantNativeBackupAndHistoryCitations(t *testing.T) {
	s, w := fixture(t)
	testSecrets(s)
	n := mustNote(t, s, "事实资料")
	memory, err := s.SaveAssistantMemory(w.ID, AssistantMemory{Text: "长期项目背景"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.SaveAssistantConversation(w.ID, AssistantConversation{ID: uid(), Entries: []AssistantEntry{{Role: "user", Content: "请解释事实", Sources: []LLMSource{}}}})
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "knowledge.zip")
	if err = s.Export(backup, true); err != nil {
		t.Fatal(err)
	}
	restored, err := s.Import(backup, filepath.Join(t.TempDir(), "restored"))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.OpenWorkspace(restored.ID); err != nil {
		t.Fatal(err)
	}
	gotMemory, err := s.GetAssistantMemory(restored.ID)
	if err != nil || gotMemory != memory {
		t.Fatal("backup lost memory", err)
	}
	gotConversation, err := s.GetAssistantConversation(restored.ID, c.ID)
	if err != nil || gotConversation.Entries[0].Content != "请解释事实" {
		t.Fatal("backup lost conversation", err)
	}
	if err = s.OpenWorkspace(w.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveAssistantConversation(w.ID, AssistantConversation{ID: uid(), Entries: []AssistantEntry{{Role: "user", Content: "认证失败的排查", Sources: []LLMSource{}}, {Role: "assistant", Content: "曾参考资料 [9]，认证问题还需核实。", Sources: []LLMSource{{NoteID: n.ID, Hash: "old"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	historical, err := s.assistantKnowledge(w, snap.Notes, "search_conversations", assistantToolArgs{Query: "认证"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(historical)
	if !strings.Contains(string(encoded), "历史引用") || strings.Contains(string(encoded), "[9]") || !strings.Contains(string(encoded), "changedSinceConversation") {
		t.Fatal("unsafe historical excerpts", string(encoded))
	}
	var sent []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		sent = body.Messages
		fmt.Fprint(rw, `{"choices":[{"message":{"content":"已结合资料 [1]。"}}]}`)
	}))
	defer server.Close()
	p := chatProvider(t, s, server.URL, "openai")
	in := LLMChatInput{RequestID: uid(), WorkspaceID: w.ID, ProviderID: p.ID, Model: p.Model, Scope: "workspace", NoteID: n.ID, Hash: n.Hash, Messages: []LLMChatMessage{{Role: "assistant", Content: "之前提到资料 [2] 和失效资料 [1]。", Sources: []LLMSource{{NoteID: "deleted", Hash: "old"}, {NoteID: n.ID, Hash: n.Hash}}}, {Role: "user", Content: "继续解释"}}}
	out, err := s.ChatLLM(in)
	if err != nil || len(out.Sources) != 1 {
		t.Fatal(out, err)
	}
	history := fmt.Sprint(sent[len(sent)-2]["content"])
	if !strings.Contains(history, "资料 [1]") || !strings.Contains(history, "历史来源已变化") {
		t.Fatal("old citations were not remapped safely", history)
	}
}

func TestAssistantMetadataToolsAndRecurringDates(t *testing.T) {
	s, w := fixture(t)
	n, err := s.CreateNote("今天提醒我开会", "Work")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateNote("子目录资料", "Work/Child")
	if err != nil {
		t.Fatal(err)
	}
	n, err = s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n[关联](Child/子目录资料.md)\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddBookmark(BookmarkInput{WorkspaceID: w.ID, NoteID: n.ID, Hash: n.Hash, Line: 1, Column: 0, Label: "重点"}); err != nil {
		t.Fatal(err)
	}
	if err = writeJSON(filepath.Join(s.data(w), "attachments-manifest.json"), []Attachment{{ID: "a", Name: "方案.pdf", Mime: "application/pdf", RemoteURL: "https://private.example/secret-token"}}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil || len(snap.Tasks) != 1 {
		t.Fatal(snap.Tasks, err)
	}
	query := func(name string, arg assistantToolArgs) map[string]any {
		t.Helper()
		value, e := s.assistantKnowledge(w, snap.Notes, name, arg)
		if e != nil {
			t.Fatal(e)
		}
		return value.(map[string]any)
	}
	page := query("list_notes", assistantToolArgs{Directory: "Work"})
	if page["total"] != 1 || page["items"].([]map[string]any)[0]["noteId"] != n.ID {
		t.Fatal("directory recursion leaked", page)
	}
	page = query("list_notes", assistantToolArgs{Limit: 1})
	if page["total"] != 2 || page["hasMore"] != true {
		t.Fatal("paging failed", page)
	}
	page = query("list_bookmarks", assistantToolArgs{Query: "重点"})
	if page["total"] != 1 {
		t.Fatal(page)
	}
	page = query("get_note_links", assistantToolArgs{NoteID: other.ID})
	if page["total"] != 1 || page["items"].([]map[string]any)[0]["direction"] != "incoming" {
		t.Fatal("incoming link missing", page)
	}
	page = query("list_attachments", assistantToolArgs{})
	encoded, _ := json.Marshal(page)
	if strings.Contains(string(encoded), "secret-token") || strings.Contains(string(encoded), "private.example") {
		t.Fatal("attachment external URL exposed")
	}
	page = query("list_tasks", assistantToolArgs{})
	if page["total"] != 0 {
		t.Fatal("unconfirmed task treated as accepted", page)
	}
	page = query("list_tasks", assistantToolArgs{Status: "confirmation"})
	if page["total"] != 1 {
		t.Fatal(page)
	}
	task := snap.Tasks[0]
	if err = s.ResolveTaskIntent(TaskIntentDecision{WorkspaceID: w.ID, TaskID: task.ID, SourceRevision: task.SourceRevision, Accept: true}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(s.data(w), "tasks-metadata.json"))
	page = query("list_tasks", assistantToolArgs{})
	after, _ := os.ReadFile(filepath.Join(s.data(w), "tasks-metadata.json"))
	if page["total"] != 1 || string(before) != string(after) {
		t.Fatal("task query changed data", page)
	}
	context := query("get_workspace_context", assistantToolArgs{})
	encoded, _ = json.Marshal(context)
	if strings.Contains(string(encoded), w.Path) || context["similarityThreshold"] != 0.55 {
		t.Fatal("context leaked absolute path or lost config", string(encoded))
	}
	for _, tc := range []struct {
		r              Recurrence
		from, to, want string
	}{
		{Recurrence{Unit: "month", Interval: 1, Anchor: "2026-01-31", Day: 31}, "2026-02-01", "2026-02-28", "2026-02-28"},
		{Recurrence{Unit: "week", Interval: 2, Anchor: "2020-01-06"}, "2026-10-19", "2026-10-25", "2026-10-19"},
		{Recurrence{Unit: "day", Interval: 180, Anchor: "2026-01-01", Until: "2026-06-30"}, "2026-06-01", "2026-12-31", "2026-06-30"},
	} {
		dates := assistantTaskDates(Task{Recurrence: &tc.r}, tc.from, tc.to)
		if len(dates) != 1 || dates[0] != tc.want {
			t.Fatalf("recurrence %+v: %v", tc, dates)
		}
	}
}

func TestAssistantKnowledgeChatContextPagingAndScope(t *testing.T) {
	for _, provider := range []string{"openai", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			s, w := fixture(t)
			testSecrets(s)
			n := mustNote(t, s, "长篇知识笔记")
			n, err := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + strings.Repeat("正文", 10000) + "尾部关键知识"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.SaveAssistantMemory(w.ID, AssistantMemory{Text: "项目背景：数据库迁移。先讲结论。"}); err != nil {
				t.Fatal(err)
			}
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				requests++
				var body struct {
					Messages []map[string]any `json:"messages"`
					Tools    []map[string]any `json:"tools"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				if requests == 1 {
					if !strings.Contains(fmt.Sprint(body.Messages[0]["content"]), "回收站") || !strings.Contains(fmt.Sprint(body.Messages), "数据库迁移") || len(body.Tools) != 11 {
						t.Error("missing app capabilities, memory, or tools")
					}
					if strings.Contains(fmt.Sprint(body.Messages[0]["content"]), "项目背景：数据库迁移") {
						t.Error("user memory elevated to system")
					}
					toolReply(rw, provider, "read_note", map[string]any{"noteId": n.ID, "offset": 18000, "limit": 18000})
					return
				}
				if !strings.Contains(fmt.Sprint(body.Messages), "尾部关键知识") || !strings.Contains(fmt.Sprint(body.Messages), "nextOffset") {
					t.Error("long note tail not returned")
				}
				if provider == "openai" {
					fmt.Fprint(rw, `{"choices":[{"message":{"content":"笔记尾部包含关键知识 [1]。"}}]}`)
				} else {
					fmt.Fprint(rw, `{"message":{"content":"笔记尾部包含关键知识 [1]。"}}`)
				}
			}))
			defer server.Close()
			p := chatProvider(t, s, server.URL, provider)
			out, err := s.ChatLLM(LLMChatInput{RequestID: uid(), WorkspaceID: w.ID, ProviderID: p.ID, Model: p.Model, Scope: "workspace", NoteID: n.ID, Hash: n.Hash, Messages: []LLMChatMessage{{Role: "user", Content: "结合项目背景看完全文"}}})
			if err != nil || len(out.Sources) != 1 || requests != 2 {
				t.Fatalf("chat %+v %v", out, err)
			}
		})
	}
	// Newly added workspace tools obey the same note-only boundary.
	s, w := fixture(t)
	testSecrets(s)
	n := mustNote(t, s, "当前笔记")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			toolReply(rw, "openai", "list_tasks", map[string]string{})
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(fmt.Sprint(body["messages"]), "当前仅允许读取当前笔记") {
			t.Error("workspace scope bypass")
		}
		fmt.Fprint(rw, `{"choices":[{"message":{"content":"请切换工作区范围。"}}]}`)
	}))
	defer server.Close()
	p := chatProvider(t, s, server.URL, "openai")
	if _, err := s.ChatLLM(LLMChatInput{RequestID: uid(), WorkspaceID: w.ID, ProviderID: p.ID, Model: p.Model, Scope: "note", NoteID: n.ID, Hash: n.Hash, Messages: []LLMChatMessage{{Role: "user", Content: "查看待办"}}}); err != nil {
		t.Fatal(err)
	}
}

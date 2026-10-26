package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLLMProvidersMigrateModelsAndCredentialIsolation(t *testing.T) {
	s, _ := fixture(t)
	testSecrets(s)
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if r.Method == "GET" {
			fmt.Fprint(w, `{"data":[{"id":"model-b"},{"id":"model-a"},{"id":"model-b"}]}`)
		} else {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"OK"}}]}`)
		}
	}))
	defer server.Close()
	c, _ := s.GetLLMConfig()
	c.Endpoint = server.URL
	c.Model = "model-a"
	c.Enabled = true
	c, e := s.SaveLLMConfig(c, "legacy-private-key", false)
	if e != nil {
		t.Fatal(e)
	}
	state, e := s.GetLLMProviders()
	if e != nil || len(state.Providers) != 1 || !state.Providers[0].HasSecret {
		t.Fatalf("legacy %+v %v", state, e)
	}
	models, e := s.ListLLMModels(state.Providers[0], "", false)
	if e != nil || strings.Join(models, ",") != "model-a,model-b" || gotAuth != "Bearer legacy-private-key" || gotPath != "/v1/models" {
		t.Fatalf("models %v %s %s %v", models, gotAuth, gotPath, e)
	}
	if _, e = os.Stat(filepath.Join(s.root, "llm-providers.json")); !os.IsNotExist(e) {
		t.Fatal("list mutated settings")
	}
	second := LLMProvider{ID: "second", Name: "第二供应商", LLMConfig: LLMConfig{Provider: "openai", Endpoint: server.URL + "/second", Model: "model-b"}, Models: models, SelectedModels: []string{}}
	if _, e = s.TestLLMProvider(second, "", false); e != nil || gotAuth != "" {
		t.Fatalf("draft reused old credential %s %v", gotAuth, e)
	}
	state.Providers[0].Models = models
	state.Providers = append(state.Providers, second)
	state, e = s.SaveLLMProviders(state, map[string]string{"second": "second-private-key"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if !state.Providers[0].HasSecret || !state.Providers[1].HasSecret {
		t.Fatal("migration lost secrets")
	}
	b, _ := os.ReadFile(filepath.Join(s.root, "llm-providers.json"))
	if strings.Contains(string(b), "private-key") {
		t.Fatal("plaintext secret")
	}
	if _, e = s.TestLLM(); e != nil || gotAuth != "Bearer legacy-private-key" {
		t.Fatalf("legacy adapter %s %v", gotAuth, e)
	}
	if _, e = s.TestLLMProvider(state.Providers[1], "", false); e != nil || gotAuth != "Bearer second-private-key" {
		t.Fatalf("second %s %v", gotAuth, e)
	}
	oldSecond := state.Providers[1]
	oldKey := s.llmSecretKey(oldSecond.LLMConfig)
	state.Providers[1].Endpoint = server.URL + "/changed"
	state, e = s.SaveLLMProviders(state, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if state.Providers[1].HasSecret {
		t.Fatal("credential survived destination change")
	}
	if _, e = s.TestLLMProvider(state.Providers[1], "", false); e != nil || gotAuth != "" {
		t.Fatalf("forwarded credential %s %v", gotAuth, e)
	}
	if v, _ := s.secrets.Get(oldKey); v != "" {
		t.Fatal("old slot not removed")
	}
	if _, e = s.TestLLMProvider(oldSecond, "", false); e == nil {
		t.Fatal("stale provider tested")
	}
	stale := state
	state.Providers = []LLMProvider{}
	state, e = s.SaveLLMProviders(state, nil, nil)
	if e != nil || state.Enabled || state.DefaultID != "" {
		t.Fatalf("remove all %+v %v", state, e)
	}
	if _, e = s.SaveLLMProviders(stale, nil, nil); e == nil {
		t.Fatal("stale manifest saved")
	}
	if _, e = s.TestLLMProvider(oldSecond, "", false); e == nil {
		t.Fatal("removed provider tested")
	}
	second.Revision = ""
	second.ProviderID = ""
	if _, e = s.ListLLMModels(second, "new-key", false); e != nil || gotAuth != "Bearer new-key" {
		t.Fatalf("empty manifest draft %s %v", gotAuth, e)
	}
}

func toolReply(w http.ResponseWriter, provider, name string, args any) {
	arguments := args
	if provider == "openai" {
		b, _ := json.Marshal(args)
		arguments = string(b)
	}
	m := map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": "call-1", "type": "function", "function": map[string]any{"name": name, "arguments": arguments}}}}
	if provider == "openai" {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": m}}})
	} else {
		json.NewEncoder(w).Encode(map[string]any{"message": m})
	}
}
func chatProvider(t *testing.T, s *Service, url, provider string) LLMProvider {
	t.Helper()
	state, _ := s.GetLLMProviders()
	state.Enabled = true
	state.Providers[0].Endpoint = url
	state.Providers[0].Provider = provider
	state.Providers[0].Model = "chat-model"
	state.Providers[0].Models = []string{"chat-model", "other-model"}
	state, e := s.SaveLLMProviders(state, map[string]string{state.Providers[0].ID: "chat-secret"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return state.Providers[0]
}
func TestLLMAssistantSearchReadProposeAndConfirm(t *testing.T) {
	s, w := fixture(t)
	testSecrets(s)
	current := mustNote(t, s, "周末计划")
	target := mustNote(t, s, "水果采购")
	target, e := s.SaveNote(SaveInput{ID: target.ID, Path: target.Path, Hash: target.Hash, Content: target.Content + "\n采购水果：苹果和香蕉。\n"})
	if e != nil {
		t.Fatal(e)
	}
	var mu sync.Mutex
	calls := 0
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls++
		round := calls
		requests = append(requests, string(body))
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer chat-secret" {
			t.Error("wrong auth")
		}
		switch round {
		case 1:
			toolReply(rw, "openai", "search_notes", map[string]string{"query": "采购水果"})
		case 2:
			toolReply(rw, "openai", "read_note", map[string]string{"noteId": target.ID})
		case 3:
			toolReply(rw, "openai", "propose_note_edit", map[string]string{"noteId": target.ID, "oldText": "采购水果：苹果和香蕉。", "newText": "采购水果：苹果、香蕉和梨。", "reason": "补充采购清单"})
		default:
			fmt.Fprint(rw, `{"choices":[{"message":{"content":"找到采购清单 [2]。修改建议尚未保存，请确认。"}}]}`)
		}
	}))
	defer server.Close()
	p := chatProvider(t, s, server.URL, "openai")
	in := LLMChatInput{RequestID: uid(), WorkspaceID: w.ID, ProviderID: p.ID, Model: p.Model, NoteID: current.ID, Hash: current.Hash, Scope: "workspace", Messages: []LLMChatMessage{{Role: "user", Content: "检索采购清单，补充梨"}}}
	out, e := s.ChatLLM(in)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Proposals) != 1 || len(out.Sources) != 2 || out.Sources[1].NoteID != target.ID || len(out.Steps) != 3 {
		t.Fatalf("result %+v", out)
	}
	if readNoteFile(t, w, target.Path) != target.Content {
		t.Fatal("assistant wrote without confirmation")
	}
	if !strings.Contains(requests[3], "tool_call_id") || !strings.Contains(requests[3], "尚未保存") {
		t.Fatal("tool results were not returned")
	}
	edited := out.Proposals[0]
	edited.NewText += "人工校对 $&"
	edited.Content = strings.Replace(edited.Original, edited.OldText, edited.NewText, 1)
	saved, e := s.ApplyLLMEdit(edited)
	if e != nil || !strings.Contains(saved.Content, "香蕉和梨") || !strings.Contains(saved.Content, "人工校对 $&") {
		t.Fatalf("apply %+v %v", saved, e)
	}
	if _, e = s.ApplyLLMEdit(out.Proposals[0]); e == nil {
		t.Fatal("stale proposal overwritten")
	}
	if !strings.Contains(readNoteFile(t, w, target.Path), "香蕉和梨") {
		t.Fatal("confirmed edit missing")
	}
	in.Model = "outside-subset"
	state, _ := s.GetLLMProviders()
	state.Providers[0].SelectedModels = []string{"chat-model"}
	state, e = s.SaveLLMProviders(state, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ChatLLM(in); e == nil {
		t.Fatal("unselected model used")
	}
	state.Enabled = false
	_, e = s.SaveLLMProviders(state, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	in.Model = "chat-model"
	if _, e = s.ChatLLM(in); e == nil {
		t.Fatal("disabled chat requested")
	}
	if calls != 4 {
		t.Fatalf("unexpected calls %d", calls)
	}
}

func TestLLMAssistantOllamaScopeAndTamperedEdits(t *testing.T) {
	s, w := fixture(t)
	testSecrets(s)
	n := mustNote(t, s, "范围测试")
	n, e := s.SaveNote(SaveInput{ID: n.ID, Path: n.Path, Hash: n.Hash, Content: n.Content + "\n唯一原文\n"})
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/chat" {
			t.Error(r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if calls == 1 {
			toolReply(rw, "ollama", "search_notes", map[string]string{"query": "private"})
		} else {
			if !strings.Contains(string(b), "当前仅允许读取当前笔记") || !strings.Contains(string(b), "tool_name") {
				t.Errorf("scope response %s", b)
			}
			fmt.Fprint(rw, `{"message":{"content":"当前仅能读取这篇笔记 [1]。"}}`)
		}
	}))
	defer server.Close()
	p := chatProvider(t, s, server.URL, "ollama")
	out, e := s.ChatLLM(LLMChatInput{RequestID: uid(), WorkspaceID: w.ID, ProviderID: p.ID, Model: p.Model, NoteID: n.ID, Hash: n.Hash, Scope: "note", Messages: []LLMChatMessage{{Role: "user", Content: "检索"}}})
	if e != nil || len(out.Sources) != 1 {
		t.Fatalf("scope %+v %v", out, e)
	}
	proposal, e := makeLLMProposal(w.ID, n, "唯一原文", "新内容", "润色")
	if e != nil {
		t.Fatal(e)
	}
	proposal.Content += "tampered"
	if _, e = s.ApplyLLMEdit(proposal); e == nil {
		t.Fatal("tampered content applied")
	}
	if _, e = makeLLMProposal(w.ID, n, "id:", "other:", ""); e == nil {
		t.Fatal("metadata edit proposed")
	}
	if _, e = makeLLMProposal(w.ID, Note{Content: "body"}, "body", "---\nid: injected\n---\nbody", ""); e == nil {
		t.Fatal("metadata injection proposed")
	}
	proposal, e = makeLLMProposal("other-workspace", n, "唯一原文", "新内容", "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ApplyLLMEdit(proposal); e == nil {
		t.Fatal("cross-workspace edit applied")
	}
}

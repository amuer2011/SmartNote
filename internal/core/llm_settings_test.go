package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLLMDraftConnectionCredentialsAndPersistence(t *testing.T) {
	s, _ := fixture(t)
	testSecrets(s)
	var auth, path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"OK"}}]}`)
	}))
	defer server.Close()
	c, _ := s.GetLLMConfig()
	c.Endpoint, c.Model = server.URL, "saved-model"
	c, e := s.SaveLLMConfig(c, "saved-key", false)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(s.root, "llm.json"))
	draft := c
	draft.Model, draft.ReasoningEffort = "draft-model", "high"
	if _, e = s.TestLLMConfig(draft, "draft-key", false); e != nil {
		t.Fatal(e)
	}
	if auth != "Bearer draft-key" || path != "/v1/chat/completions" || body["model"] != "draft-model" || body["reasoning_effort"] != "high" || body["max_tokens"] != float64(16384) || body["max_completion_tokens"] != nil {
		t.Fatalf("unexpected draft request: %s %s %+v", auth, path, body)
	}
	for _, message := range body["messages"].([]any) {
		if !strings.Contains(message.(map[string]any)["content"].(string), "OK") && !strings.Contains(message.(map[string]any)["content"].(string), "无笔记内容") {
			t.Fatal("test sent unexpected content")
		}
	}
	after, _ := os.ReadFile(filepath.Join(s.root, "llm.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("draft test changed saved settings")
	}
	if _, e = s.llmRequest(uid(), c, "system", "private text"); e == nil {
		t.Fatal("disabled generation allowed")
	}
	if _, e = s.TestLLM(); e != nil || auth != "Bearer saved-key" {
		t.Fatalf("saved test: %s %v", auth, e)
	}
	if _, e = s.TestLLMConfig(c, "", true); e != nil || auth != "" {
		t.Fatalf("clear test: %s %v", auth, e)
	}
	draft.Endpoint = server.URL + "/other"
	if _, e = s.TestLLMConfig(draft, "", false); e != nil || auth != "" {
		t.Fatalf("key crossed destination: %s %v", auth, e)
	}
	draft = c
	draft.ReasoningEffort = "high"
	saved, e := s.SaveLLMConfig(draft, "", false)
	if e != nil {
		t.Fatal(e)
	}
	got, e := s.GetLLMConfig()
	if e != nil || got.ReasoningEffort != "high" || got.Enabled || !got.HasSecret {
		t.Fatalf("save %+v %v", got, e)
	}
	if _, e = s.TestLLMConfig(c, "", false); e == nil {
		t.Fatal("stale draft accepted")
	}
	saved.ReasoningEffort = "unsupported"
	if _, e = s.SaveLLMConfig(saved, "", false); e == nil {
		t.Fatal("invalid reasoning accepted")
	}
}

func TestLLMEndpointAndReasoningProtocols(t *testing.T) {
	s, _ := fixture(t)
	testSecrets(s)
	var gotPath string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"OK"}}],"message":{"content":"OK"}}`)
	}))
	defer server.Close()
	for _, tc := range []struct {
		provider, endpoint, effort, path string
		think                            any
	}{
		{"openai", "", "", "/v1/chat/completions", nil},
		{"openai", "/v1/", "high", "/v1/chat/completions", nil},
		{"openai", "/v1/chat/completions", "none", "/v1/chat/completions", nil},
		{"openai", "/custom/v2", "max", "/custom/v2/chat/completions", nil},
		{"ollama", "", "", "/api/chat", nil},
		{"ollama", "/api/", "on", "/api/chat", true},
		{"ollama", "/api/chat", "off", "/api/chat", false},
		{"ollama", "", "high", "/api/chat", "high"},
	} {
		t.Run(tc.provider+tc.endpoint+tc.effort, func(t *testing.T) {
			c, _ := s.GetLLMConfig()
			c.Model = "protocol-test-model" // Protocol coverage is independent of a named model's capability.
			c.Provider, c.Endpoint, c.ReasoningEffort = tc.provider, server.URL+tc.endpoint, tc.effort
			body = nil
			if _, e := s.TestLLMConfig(c, "", false); e != nil {
				t.Fatal(e)
			}
			if gotPath != tc.path || body["think"] != tc.think {
				t.Fatalf("request %s %+v", gotPath, body)
			}
			if tc.provider == "ollama" && body["reasoning_effort"] != nil {
				t.Fatal("wrong provider parameter")
			}
			if tc.effort == "" && (body["reasoning_effort"] != nil || body["think"] != nil) {
				t.Fatal("default forced reasoning")
			}
		})
	}
}

func TestLLMResponseFormatsAndSafeErrors(t *testing.T) {
	for _, tc := range []struct{ body, want, problem string }{
		{`{"choices":[{"message":{"content":"OK"}}]}`, "OK", ""},
		{`{"choices":[{"message":{"content":[{"type":"text","text":"A"},{"type":"thinking","text":"private"},{"type":"text","text":"B"}]}}]}`, "AB", ""},
		{"\xef\xbb\xbf{\"choices\":[{\"message\":{\"content\":\"OK\"}}]}", "OK", ""},
		{`<!doctype html><html>secret-payload</html>`, "", "HTML 网页"},
		{`data: {"secret":"secret-payload"}`, "", "事件流"},
		{`not-json secret-payload`, "", "不是有效的 JSON"},
		{`{"error":{"message":"secret-payload"}}`, "", "服务返回了错误"},
		{`{"output_text":"secret-payload"}`, "", "choices"},
		{`{"choices":[{"message":{"content":null,"reasoning_content":"secret-payload"}}]}`, "", "未返回正文"},
	} {
		got, e := decodeLLMResponse([]byte(tc.body), "openai")
		if got != tc.want || (tc.problem == "" && e != nil) || (tc.problem != "" && (e == nil || !strings.Contains(e.Error(), tc.problem))) {
			t.Fatalf("response %q: %q %v", tc.body, got, e)
		}
		if e != nil && strings.Contains(e.Error(), "secret-payload") {
			t.Fatal("raw server response leaked")
		}
	}
	// Reproduce a base URL returning a website with HTTP 200, without touching real notes.
	s, _ := fixture(t)
	testSecrets(s)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html>private server details</html>")
	}))
	defer server.Close()
	c, _ := s.GetLLMConfig()
	c.Endpoint = server.URL
	if _, e := s.TestLLMConfig(c, "", false); e == nil || !strings.Contains(e.Error(), "HTML 网页") {
		t.Fatalf("website not diagnosed: %v", e)
	}
}

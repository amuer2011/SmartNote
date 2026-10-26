package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReasoningModelValidation(t *testing.T) {
	for _, tc := range []struct {
		model, effort string
		valid         bool
	}{
		{"gpt-5", "minimal", true}, {"gpt-5", "none", false}, {"gpt-5.1", "none", true}, {"gpt-5.1", "xhigh", false},
		{"gpt-5.2", "xhigh", true}, {"gpt-5.2", "minimal", false}, {"gpt-5.2-2025-12-11", "xhigh", true},
		{"openai/gpt-5.2", "max", false}, {"gpt-5.4-mini", "xhigh", true}, {"gpt-6-luna", "max", true},
		{"gpt-6-astra", "none", false}, {"gpt-6.1-sol", "minimal", false}, {"gpt-6.1-sol", "low", true},
		{"gpt-4.1-mini", "high", false}, {"gpt-4.1-mini", "", true}, {"gpt-4o-2024-08-06", "low", false},
		{"gpt-5.2-custom", "minimal", true}, {"my-gpt-5.1", "xhigh", true}, {"gpt-6-luna-custom", "minimal", true},
	} {
		t.Run(tc.model+"/"+tc.effort, func(t *testing.T) {
			_, err := validateLLMConfig(LLMConfig{Provider: "openai", Endpoint: "https://api.example.com/v1", Model: tc.model, ReasoningEffort: tc.effort}, "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
		})
	}
}

func TestSavedReasoningCompatibility(t *testing.T) {
	s, _ := fixture(t)
	testSecrets(s)
	for _, legacy := range []bool{true, false} {
		c := LLMConfig{Provider: "openai", Endpoint: "https://api.example.com/v1", Model: "gpt-5.1", ReasoningEffort: "max", Revision: "saved-revision"}
		path := filepath.Join(s.root, "llm.json")
		var value any = c
		if !legacy {
			path = filepath.Join(s.root, "llm-providers.json")
			value = LLMProviders{DefaultID: "p", Providers: []LLMProvider{{ID: "p", LLMConfig: c}}}
		}
		b, _ := json.Marshal(value)
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		loaded, err := s.GetLLMConfig()
		if err != nil || loaded.ReasoningEffort != "" || loaded.Revision != c.Revision {
			t.Fatalf("legacy=%v: %+v %v", legacy, loaded, err)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(b) {
			t.Fatal("reading migrated settings modified the file")
		}
	}
}

func TestOllamaReasoningDiscoveryAndWireValues(t *testing.T) {
	s, _ := fixture(t)
	testSecrets(s)
	var auth, path, method, model string
	var response string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path, method = r.Header.Get("Authorization"), r.URL.Path, r.Method
		json.NewDecoder(r.Body).Decode(&body)
		model, _ = body["model"].(string)
		if strings.HasSuffix(path, "/show") {
			fmt.Fprint(w, response)
		} else {
			fmt.Fprint(w, `{"message":{"content":"OK"}}`)
		}
	}))
	defer server.Close()
	p := LLMProvider{ID: "new", Name: "Local", LLMConfig: LLMConfig{Provider: "ollama", Endpoint: server.URL + "/api/chat", Model: "custom-model", ReasoningEffort: "old-invalid"}}
	before, _ := os.ReadFile(filepath.Join(s.root, "llm-providers.json"))
	for _, tc := range []struct {
		json    string
		known   bool
		efforts []string
		bad     bool
	}{
		{`{"thinking":{"values":["low","medium","high"]}}`, true, []string{"low", "medium", "high"}, false},
		{`{"thinking":{"values":[false,true,true]}}`, true, []string{"off", "on"}, false},
		{`{"thinking":{"values":[true]}}`, true, []string{"on"}, false},
		{`{"thinking":{"values":[false]}}`, true, []string{}, false},
		{`{"thinking":{"values":["adaptive","on",true]}}`, true, []string{"level:adaptive", "level:on", "on"}, false},
		{`{}`, false, []string{}, false}, {`{"thinking":{}}`, false, []string{}, false},
		{`{"thinking":{"values":[1]}}`, false, nil, true}, {`{"thinking":{"values":["bad\nvalue"]}}`, false, nil, true},
		{`private-response`, false, nil, true},
	} {
		response = tc.json
		out, err := s.GetLLMReasoningCapability(p, "draft-key", false)
		if tc.bad {
			if err == nil || strings.Contains(err.Error(), "private-response") {
				t.Fatalf("unsafe result: %v", err)
			}
			continue
		}
		if err != nil || out.Known != tc.known || !reflect.DeepEqual(out.Efforts, tc.efforts) {
			t.Fatalf("%s: %+v %v", tc.json, out, err)
		}
		if path != "/api/show" || method != "POST" || model != "custom-model" || auth != "Bearer draft-key" || len(body) != 1 {
			t.Fatalf("metadata request %s %s %+v", method, path, body)
		}
	}
	after, _ := os.ReadFile(filepath.Join(s.root, "llm-providers.json"))
	if string(before) != string(after) {
		t.Fatal("discovery saved settings")
	}
	for _, tc := range []struct {
		effort string
		wire   any
	}{
		{"on", true}, {"off", false}, {"high", "high"}, {"level:adaptive", "adaptive"}, {"level:on", "on"},
	} {
		c := p.LLMConfig
		c.ReasoningEffort = tc.effort
		if _, err := validateLLMConfig(c, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := llmHTTP(context.Background(), c, "", []map[string]any{{"role": "user", "content": "OK"}}, nil); err != nil {
			t.Fatal(err)
		}
		if body["think"] != tc.wire {
			t.Fatalf("%s: %+v", tc.effort, body)
		}
	}
	for _, value := range []string{"level:", "level:bad\nvalue", "level: value", "level:" + strings.Repeat("x", 65)} {
		c := p.LLMConfig
		c.ReasoningEffort = value
		if _, err := validateLLMConfig(c, ""); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	// Persisted credentials are reused only at the saved address and revision.
	state, _ := s.GetLLMProviders()
	p.ReasoningEffort = ""
	state.DefaultID = p.ID
	state.Providers = []LLMProvider{p}
	state, err := s.SaveLLMProviders(state, map[string]string{p.ID: "saved-key"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p = state.Providers[0]
	response = `{"thinking":{"values":[true]}}`
	if _, err = s.GetLLMReasoningCapability(p, "", false); err != nil || auth != "Bearer saved-key" {
		t.Fatalf("saved credential: %s %v", auth, err)
	}
	p.Endpoint = server.URL + "/other"
	if _, err = s.GetLLMReasoningCapability(p, "", false); err != nil || auth != "" {
		t.Fatalf("credential crossed address: %s %v", auth, err)
	}
	p = state.Providers[0]
	p.Revision = "stale"
	if _, err = s.GetLLMReasoningCapability(p, "", false); err == nil {
		t.Fatal("stale profile queried")
	}
	p = state.Providers[0]
	p.Provider = "openai"
	if _, err = s.GetLLMReasoningCapability(p, "", false); err == nil {
		t.Fatal("remote generation protocol queried")
	}
}

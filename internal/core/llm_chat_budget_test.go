package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Real HTTP round trips check both wire protocols and one result per requested
// call, including duplicates and calls rejected by scope or execution budget.
func TestLLMChatToolBatchesAndFinalAnswer(t *testing.T) {
	for _, provider := range []string{"openai", "ollama"} {
		for _, scenario := range []string{"batch", "scope", "budget", "rounds"} {
			t.Run(provider+"/"+scenario, func(t *testing.T) {
				s, workspace := fixture(t)
				testSecrets(s)
				current := mustNote(t, s, "总结测试")
				targets := []Note{}
				for i := 0; i < 8; i++ {
					targets = append(targets, mustNote(t, s, fmt.Sprintf("资料%d", i)))
				}
				requests, previousBatch := 0, 0
				scope := "workspace"
				if scenario == "scope" {
					scope = "note"
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					var body struct {
						Messages []map[string]any `json:"messages"`
						Tools    json.RawMessage  `json:"tools"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if requests > 1 {
						start := len(body.Messages)
						for start > 0 && body.Messages[start-1]["role"] != "assistant" {
							start--
							if start == 0 {
								break
							}
						}
						results := body.Messages[start : start+previousBatch]
						for i, result := range results {
							if result["role"] != "tool" {
								t.Errorf("missing result %d: %+v", i, result)
							}
							if provider == "openai" && result["tool_call_id"] != fmt.Sprintf("call-%d-%d", requests-1, i) {
								t.Errorf("wrong result id: %+v", result)
							}
						}
					}
					final := requests > 1 && scenario != "rounds" || requests == 7
					if final {
						if scenario == "budget" || scenario == "rounds" {
							if len(body.Tools) != 0 || !strings.Contains(fmt.Sprint(body.Messages), "资料收集已结束") {
								t.Error("missing final synthesis without tools")
							}
						}
						message := map[string]any{"role": "assistant", "content": "笔记总结完成 [1]。"}
						if provider == "openai" {
							json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message}}})
						} else {
							json.NewEncoder(w).Encode(map[string]any{"message": message})
						}
						return
					}
					calls := []any{}
					addCall := func(name string, args any) {
						if provider == "openai" {
							encoded, _ := json.Marshal(args)
							args = string(encoded)
						}
						calls = append(calls, map[string]any{"id": fmt.Sprintf("call-%d-%d", requests, len(calls)), "type": "function", "function": map[string]any{"name": name, "arguments": args}})
					}
					if scenario == "budget" {
						for i := 0; i < 24; i++ {
							addCall("read_note", map[string]string{"noteId": fmt.Sprintf("missing-%d", i)})
						}
						addCall("read_note", map[string]string{"noteId": current.ID})
					} else if scenario == "rounds" {
						addCall("read_note", map[string]string{"noteId": current.ID})
					} else {
						for _, n := range targets {
							addCall("read_note", map[string]string{"noteId": n.ID})
						}
						addCall("read_note", map[string]string{"noteId": targets[0].ID})
					}
					previousBatch = len(calls)
					message := map[string]any{"role": "assistant", "content": "", "tool_calls": calls}
					if provider == "openai" {
						json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message}}})
					} else {
						json.NewEncoder(w).Encode(map[string]any{"message": message})
					}
				}))
				defer server.Close()
				p := chatProvider(t, s, server.URL, provider)
				out, err := s.ChatLLM(LLMChatInput{RequestID: uid(), WorkspaceID: workspace.ID, ProviderID: p.ID, Model: p.Model, NoteID: current.ID, Hash: current.Hash, Scope: scope, Messages: []LLMChatMessage{{Role: "user", Content: "总结笔记"}}})
				if err != nil || out.Text != "笔记总结完成 [1]。" {
					t.Fatalf("summary: %+v %v", out, err)
				}
				wantSteps, wantSources, wantRequests := 0, 1, 2
				if scenario == "batch" {
					wantSteps, wantSources = 8, 9
				}
				if scenario == "rounds" {
					wantSteps, wantRequests = 1, 7
				}
				if len(out.Steps) != wantSteps || len(out.Sources) != wantSources || requests != wantRequests || len(out.Proposals) != 0 {
					t.Fatalf("wrong execution counts: %+v requests=%d", out, requests)
				}
			})
		}
	}
}

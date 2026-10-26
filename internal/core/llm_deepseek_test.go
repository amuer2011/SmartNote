package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepSeekGatewayReasoningWire(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body = nil // Each request must be checked independently of the previous model/effort.
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["max_completion_tokens"] != nil || body["max_tokens"] != float64(16384) {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"param":"max_completion_tokens","message":"unsupported"}}`)
			return
		}
		if body["reasoning_effort"] == "none" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"param":"reasoning_effort","message":"use thinking instead"}}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"OK"}}]}`)
	}))
	defer server.Close()
	for _, model := range []string{"commandcode-go/deepseek-v4.1-flash", "deepseek-v4.1-flash-fast", "commandcode-go/deepseek-v4.1-flash-fast"} {
		t.Run(model, func(t *testing.T) {
			messages := []map[string]any{{"role": "assistant", "content": "Previous answer"}, {"role": "assistant", "content": nil, "reasoning_content": "tool reasoning", "tool_calls": []any{}}, {"role": "user", "content": "Summarize"}}
			for _, effort := range []string{"", "none", "low", "high", "max"} {
				c := LLMConfig{Provider: "openai", Endpoint: server.URL, Model: model, ReasoningEffort: effort}
				if _, err := validateLLMConfig(c, ""); err != nil {
					t.Fatal(err)
				}
				if _, err := llmHTTP(context.Background(), c, "", messages, nil); err != nil {
					t.Fatalf("%s: %v", effort, err)
				}
				if body["model"] != model {
					t.Fatal("gateway model ID changed")
				}
				if effort == "none" {
					if body["thinking"].(map[string]any)["type"] != "disabled" || body["reasoning_effort"] != nil {
						t.Fatalf("off: %+v", body)
					}
				} else if effort != "" {
					if body["thinking"].(map[string]any)["type"] != "enabled" || body["reasoning_effort"] != effort {
						t.Fatalf("effort: %+v", body)
					}
				} else if body["thinking"] != nil || body["reasoning_effort"] != nil {
					t.Fatal("model default overridden")
				}
				history := body["messages"].([]any)
				if history[0].(map[string]any)["reasoning_content"] != "" || history[1].(map[string]any)["reasoning_content"] != "tool reasoning" {
					t.Fatal("reasoning history lost")
				}
				if messages[0]["reasoning_content"] != nil {
					t.Fatal("caller messages mutated")
				}
			}
			for _, effort := range []string{"minimal", "medium", "xhigh"} {
				if _, err := validateLLMConfig(LLMConfig{Provider: "openai", Endpoint: server.URL, Model: model, ReasoningEffort: effort}, ""); err == nil {
					t.Fatalf("unsupported level %s accepted", effort)
				}
			}
		})
	}
}

func TestCompatibleGatewayTokenBudgetAndSafe400(t *testing.T) {
	for _, model := range []string{"gpt-5.2", "custom-model"} {
		var body map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&body)
			fmt.Fprint(w, `{"choices":[{"message":{"content":"OK"}}]}`)
		}))
		_, err := llmHTTP(context.Background(), LLMConfig{Provider: "openai", Endpoint: server.URL, Model: model, ReasoningEffort: "high"}, "", nil, nil)
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if model == "gpt-5.2" && body["max_completion_tokens"] != float64(16384) || model == "custom-model" && (body["max_tokens"] != float64(16384) || body["max_completion_tokens"] != nil) {
			t.Fatalf("%s: %+v", model, body)
		}
	}
	for _, param := range []string{"max_completion_tokens", "thinking", "tools", "messages", "reasoning_effort", "model"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			fmt.Fprintf(w, `{"error":{"param":%q,"message":"secret-key and PRIVATE-NOTE must never escape"}}`, param)
		}))
		_, err := llmHTTP(context.Background(), LLMConfig{Provider: "openai", Endpoint: server.URL, Model: "custom-model"}, "secret-key", nil, nil)
		server.Close()
		if err == nil || !strings.Contains(err.Error(), param) || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "PRIVATE-NOTE") {
			t.Fatalf("%s: %v", param, err)
		}
	}
}

// Route aliases are explicit: unknown variants must not inherit this wire format.
func TestDeepSeekFastAliasBoundary(t *testing.T) {
	for _, model := range []string{"deepseek-v4.1-flash-fast-custom", "commandcode-go/deepseek-v4.1-flash-faster", "commandcode-go/my-deepseek-v4.1-flash-fast"} {
		if _, known := modelReasoningRule("openai", model); known {
			t.Fatalf("unknown route matched: %s", model)
		}
	}
}

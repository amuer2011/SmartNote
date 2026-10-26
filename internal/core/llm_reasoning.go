package core

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Shared with the frontend so model switches and server validation use the same rules.
//
//go:embed llm_reasoning_models.json
var reasoningModelsJSON []byte

type reasoningModelRule struct {
	Provider   string   `json:"provider"`
	Models     []string `json:"models"`
	Dated      bool     `json:"dated"`
	Tags       bool     `json:"tags"`
	Namespaced bool     `json:"namespaced"`
	Wire       string   `json:"wire"`
	Efforts    []string `json:"efforts"`
}

var reasoningModels = func() []reasoningModelRule {
	var rules []reasoningModelRule
	if err := json.Unmarshal(reasoningModelsJSON, &rules); err != nil {
		panic(err)
	}
	return rules
}()
var reasoningSnapshot = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func modelReasoningRule(provider, model string) (reasoningModelRule, bool) {
	name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(model)), provider+"/")
	for _, rule := range reasoningModels {
		if rule.Provider != provider {
			continue
		}
		for _, id := range rule.Models {
			candidate := name
			if rule.Namespaced {
				candidate = name[strings.LastIndex(name, "/")+1:]
			}
			if candidate == id || rule.Dated && strings.HasPrefix(candidate, id+"-") && reasoningSnapshot.MatchString(strings.TrimPrefix(candidate, id+"-")) || rule.Tags && strings.HasPrefix(candidate, id+":") {
				return rule, true
			}
		}
	}
	return reasoningModelRule{}, false
}
func modelReasoningEfforts(provider, model string) ([]string, bool) {
	rule, known := modelReasoningRule(provider, model)
	return rule.Efforts, known
}

// Old releases offered protocol-wide choices. Read incompatible saved defaults
// as the model default without rewriting settings or changing credential revisions.
func normalizeSavedReasoning(c LLMConfig) LLMConfig {
	if efforts, known := modelReasoningEfforts(c.Provider, c.Model); c.Provider == "openai" && known && !containsModel(efforts, c.ReasoningEffort) {
		c.ReasoningEffort = ""
	}
	return c
}

type LLMReasoningCapability struct {
	Known   bool     `json:"known"`
	Efforts []string `json:"efforts"`
	Message string   `json:"message"`
}

func validOllamaLevel(value string) bool {
	if !strings.HasPrefix(value, "level:") {
		return false
	}
	level := strings.TrimPrefix(value, "level:")
	return len(level) > 0 && len(level) <= 64 && strings.TrimSpace(level) == level && !strings.ContainsFunc(level, unicode.IsControl)
}

// Read metadata only: /api/show neither loads a model nor generates billable tokens.
// Credentials and endpoint changes follow the same boundary as model-list requests.
func (s *Service) GetLLMReasoningCapability(p LLMProvider, secret string, clear bool) (LLMReasoningCapability, error) {
	out := LLMReasoningCapability{Efforts: []string{}}
	if p.Provider != "ollama" {
		return out, errors.New("仅 Ollama 支持查询模型思考元数据")
	}
	p.ReasoningEffort = "" // A stale selection must not prevent discovering the new model.
	c, value, e := s.providerDraft(p, secret, clear)
	if e != nil {
		return out, e
	}
	_, e = s.llmRun(uid(), c, true, &value, func(ctx context.Context, key string) (string, error) {
		u, err := llmURL(c, false)
		if err != nil {
			return "", err
		}
		u.Path = strings.TrimSuffix(u.Path, "/chat") + "/show"
		body, _ := json.Marshal(map[string]string{"model": c.Model})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		res, err := externalClient(8 * time.Second).Do(req)
		if err != nil {
			return "", errors.New("无法读取模型思考元数据")
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return "", externalHTTPError("读取模型思考元数据", res.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, 2<<20+1))
		if err != nil || len(b) > 2<<20 {
			return "", errors.New("模型元数据过大或读取失败")
		}
		var data struct {
			Thinking *struct {
				Values []any `json:"values"`
			} `json:"thinking"`
		}
		if json.Unmarshal(b, &data) != nil {
			return "", errors.New("模型思考元数据不是有效的 JSON")
		}
		if data.Thinking == nil || len(data.Thinking.Values) == 0 {
			return "", nil
		}
		efforts := []string{}
		if len(data.Thinking.Values) > 32 {
			return "", errors.New("模型思考档位过多")
		}
		for _, v := range data.Thinking.Values {
			effort := ""
			switch v := v.(type) {
			case bool:
				if v {
					effort = "on"
				} else {
					effort = "off"
				}
			case string:
				if !validOllamaLevel("level:" + v) {
					return "", errors.New("模型思考档位无效")
				}
				effort = "level:" + v
				if v == "low" || v == "medium" || v == "high" {
					effort = v
				}
			default:
				return "", errors.New("模型思考档位类型无效")
			}
			if !containsModel(efforts, effort) {
				efforts = append(efforts, effort)
			}
		}
		if len(efforts) == 1 && efforts[0] == "off" {
			efforts = []string{}
		}
		out = LLMReasoningCapability{Known: true, Efforts: efforts, Message: "选项来自此模型的 Ollama 元数据；模型默认不传思考参数。"}
		if len(efforts) == 0 {
			out.Message = "此模型的 Ollama 元数据表明不支持思考，使用模型默认。"
		}
		return "", nil
	})
	return out, e
}

package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LLMProvider struct {
	LLMConfig
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Models         []string `json:"models"`
	SelectedModels []string `json:"selectedModels"`
}
type LLMProviders struct {
	Enabled   bool          `json:"enabled"`
	DefaultID string        `json:"defaultId"`
	Revision  string        `json:"revision"`
	Providers []LLMProvider `json:"providers"`
}

// Legacy settings remain readable until the first successful multi-provider save.
// The credential is copied into a new versioned slot before the manifest commits.
func (s *Service) loadLLMProviders() (LLMProviders, error) {
	var state LLMProviders
	path := filepath.Join(s.root, "llm-providers.json")
	if _, e := os.Stat(path); e == nil {
		e = readJSON(path, &state)
		for i := range state.Providers {
			state.Providers[i].LLMConfig = normalizeSavedReasoning(state.Providers[i].LLMConfig)
			state.Providers[i].ProviderID = state.Providers[i].ID
			state.Providers[i].Enabled = state.Enabled
		}
		return state, e
	} else if !os.IsNotExist(e) {
		return state, e
	}
	c, e := s.loadLegacyLLM()
	c = normalizeSavedReasoning(c)
	if e != nil {
		return state, e
	}
	return LLMProviders{Enabled: c.Enabled, DefaultID: "legacy", Revision: c.Revision, Providers: []LLMProvider{{LLMConfig: c, ID: "legacy", Name: "默认供应商", Models: []string{c.Model}, SelectedModels: []string{}}}}, nil
}
func (s *Service) loadLLM() (LLMConfig, error) { return s.currentLLMConfig("") }

// Caller holds s.mu. An explicit provider never falls back to another provider.
func (s *Service) currentLLMConfig(id string) (LLMConfig, error) {
	state, e := s.loadLLMProviders()
	if e != nil {
		return LLMConfig{}, e
	}
	if id == "__draft__" {
		return LLMConfig{Revision: state.Revision}, nil
	}
	if id == "" {
		id = state.DefaultID
	}
	for _, p := range state.Providers {
		if p.ID == id {
			return p.LLMConfig, nil
		}
	}
	return LLMConfig{}, errors.New("请先添加并选择大模型供应商")
}
func (s *Service) llmSecretKey(c LLMConfig) string {
	if c.ProviderID == "" {
		return s.secretKey("llm:" + c.Revision)
	}
	return s.secretKey("llm-profile:" + c.ProviderID + ":" + c.Revision)
}
func (s *Service) GetLLMProviders() (LLMProviders, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, e := s.loadLLMProviders()
	if e != nil {
		return state, e
	}
	for i := range state.Providers {
		value, err := s.secrets.Get(s.llmSecretKey(state.Providers[i].LLMConfig))
		if err != nil {
			return state, errors.New("无法读取安全凭据")
		}
		state.Providers[i].HasSecret = value != ""
	}
	return state, nil
}
func cleanModels(values []string) ([]string, error) {
	if len(values) > 3000 {
		return nil, errors.New("模型数量不能超过 3000")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || len(v) > 200 {
			return nil, errors.New("模型名称不能为空且不能超过 200 字节")
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out, nil
}
func (s *Service) SaveLLMProviders(state LLMProviders, secrets map[string]string, clearIDs []string) (LLMProviders, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, e := s.loadLLMProviders()
	if e != nil {
		return state, e
	}
	if old.Revision != state.Revision {
		return state, errors.New("配置已变化，请重新加载设置")
	}
	if len(state.Providers) > 20 {
		return state, errors.New("最多配置 20 个供应商")
	}
	byID := map[string]LLMProvider{}
	for _, p := range old.Providers {
		byID[p.ID] = p
	}
	clear := map[string]bool{}
	for _, id := range clearIDs {
		clear[id] = true
	}
	seen := map[string]bool{}
	values := map[string]string{}
	for i, p := range state.Providers {
		if p.ID == "" || p.ID == "__draft__" || len(p.ID) > 100 || seen[p.ID] {
			return state, errors.New("供应商编号为空或重复")
		}
		seen[p.ID] = true
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" || len([]rune(p.Name)) > 80 {
			return state, errors.New("请填写不超过 80 字的供应商名称")
		}
		previous, exists := byID[p.ID]
		if exists && previous.Revision != p.Revision || !exists && p.Revision != "" {
			return state, errors.New("供应商已变化，请重新加载")
		}
		p.LLMConfig, e = validateLLMConfig(p.LLMConfig, secrets[p.ID])
		if e != nil {
			return state, e
		}
		p.Models, e = cleanModels(p.Models)
		if e != nil {
			return state, e
		}
		p.SelectedModels, e = cleanModels(p.SelectedModels)
		if e != nil {
			return state, e
		}
		for _, m := range append(append([]string{}, p.SelectedModels...), p.Model) {
			if !containsModel(p.Models, m) {
				p.Models = append(p.Models, m)
			}
		}
		if len(p.SelectedModels) > 0 && !containsModel(p.SelectedModels, p.Model) {
			p.Model = p.SelectedModels[0]
		}
		value := ""
		if exists && !clear[p.ID] && previous.Endpoint == p.Endpoint && previous.Provider == p.Provider {
			value, e = s.secrets.Get(s.llmSecretKey(previous.LLMConfig))
			if e != nil {
				return state, errors.New("无法读取安全凭据")
			}
		}
		if !clear[p.ID] && secrets[p.ID] != "" {
			value = secrets[p.ID]
		}
		p.ProviderID = p.ID
		p.Revision = uid()
		p.HasSecret = false
		p.Enabled = state.Enabled
		state.Providers[i] = p
		values[p.ID] = value
	}
	if len(state.Providers) == 0 {
		state.Enabled = false
		state.DefaultID = ""
	} else if !seen[state.DefaultID] {
		state.DefaultID = state.Providers[0].ID
	}
	staged := []string{}
	rollback := func() {
		for _, key := range staged {
			_ = s.secrets.Delete(key)
		}
	}
	for _, p := range state.Providers {
		if value := values[p.ID]; value != "" {
			key := s.llmSecretKey(p.LLMConfig)
			if e = s.secrets.Set(key, value); e != nil {
				rollback()
				return state, errors.New("无法保存操作系统安全凭据")
			}
			staged = append(staged, key)
		}
	}
	state.Revision = uid()
	if e = writeJSON(filepath.Join(s.root, "llm-providers.json"), state); e != nil {
		rollback()
		return state, e
	}
	for _, p := range old.Providers {
		_ = s.secrets.Delete(s.llmSecretKey(p.LLMConfig))
	}
	// Once migrated, remove the old settings to avoid stale credentials/configuration.
	_ = os.Remove(filepath.Join(s.root, "llm.json"))
	s.externalMu.Lock()
	for _, cancel := range s.llmCancels {
		cancel()
	}
	s.externalMu.Unlock()
	for i := range state.Providers {
		state.Providers[i].HasSecret = values[state.Providers[i].ID] != ""
	}
	return state, nil
}
func containsModel(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func (s *Service) saveLLMProfileConfig(c LLMConfig, secret string, clear bool) (LLMConfig, error) {
	state, e := s.GetLLMProviders()
	if e != nil {
		return c, e
	}
	found := false
	for i, p := range state.Providers {
		if p.ID == c.ProviderID {
			if p.Revision != c.Revision {
				return c, errors.New("配置已变化，请重新加载")
			}
			state.Providers[i].LLMConfig = c
			found = true
		}
	}
	if !found {
		return c, errors.New("供应商不存在")
	}
	state.Enabled = c.Enabled
	clears := []string{}
	if clear {
		clears = append(clears, c.ProviderID)
	}
	state, e = s.SaveLLMProviders(state, map[string]string{c.ProviderID: secret}, clears)
	if e != nil {
		return c, e
	}
	for _, p := range state.Providers {
		if p.ID == c.ProviderID {
			return p.LLMConfig, nil
		}
	}
	return c, errors.New("供应商不存在")
}
func (s *Service) providerDraft(p LLMProvider, secret string, clear bool) (LLMConfig, string, error) {
	c := p.LLMConfig
	if c.Model == "" {
		c.Model = "connection-test"
	}
	c, e := validateLLMConfig(c, secret)
	if e != nil {
		return c, "", e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, e := s.loadLLMProviders()
	if e != nil {
		return c, "", e
	}
	value := ""
	exists := false
	for _, old := range state.Providers {
		if old.ID == p.ID {
			exists = true
			if old.Revision != p.Revision {
				return c, "", errors.New("配置已变化，请重新加载")
			}
			c.ProviderID = old.ProviderID
			c.Revision = old.Revision
			if !clear && secret == "" && old.Endpoint == c.Endpoint && old.Provider == c.Provider {
				value, e = s.secrets.Get(s.llmSecretKey(old.LLMConfig))
			}
		}
	}
	if !exists {
		if p.Revision != "" {
			return c, "", errors.New("供应商已删除，请重新加载")
		}
		// Bind new drafts to the current manifest, even when the list is empty.
		c.ProviderID = "__draft__"
		c.Revision = state.Revision
	}
	if e != nil {
		return c, "", errors.New("无法读取安全凭据")
	}
	if !clear && secret != "" {
		value = secret
	}
	return c, value, nil
}
func (s *Service) TestLLMProvider(p LLMProvider, secret string, clear bool) (string, error) {
	if strings.TrimSpace(p.Model) == "" {
		return "", errors.New("请填写或选择模型名称")
	}
	c, value, e := s.providerDraft(p, secret, clear)
	if e != nil {
		return "", e
	}
	_, e = s.llmRequestMode(uid(), c, "请只回复 OK。", "连接测试，无笔记内容。", true, &value)
	if e != nil {
		return "", e
	}
	return "连接成功，模型已返回文本", nil
}

// All URL variants share the same destination and never retry with credentials on another host.
func llmURL(c LLMConfig, models bool) (*url.URL, error) {
	u, e := externalURL(c.Endpoint, c.Provider == "ollama")
	if e != nil {
		return nil, e
	}
	if c.Provider == "ollama" {
		u.Path = strings.TrimSuffix(u.Path, "/api/chat")
		u.Path = strings.TrimSuffix(u.Path, "/api/tags")
		u.Path = strings.TrimSuffix(u.Path, "/api")
		if models {
			u.Path += "/api/tags"
		} else {
			u.Path += "/api/chat"
		}
	} else {
		u.Path = strings.TrimSuffix(u.Path, "/chat/completions")
		u.Path = strings.TrimSuffix(u.Path, "/models")
		if u.Path == "" {
			u.Path = "/v1"
		}
		if models {
			u.Path += "/models"
		} else {
			u.Path += "/chat/completions"
		}
	}
	return u, nil
}
func (s *Service) ListLLMModels(p LLMProvider, secret string, clear bool) ([]string, error) {
	c, value, e := s.providerDraft(p, secret, clear)
	if e != nil {
		return nil, e
	}
	models := []string{}
	_, e = s.llmRun(uid(), c, true, &value, func(ctx context.Context, key string) (string, error) {
		u, err := llmURL(c, true)
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		res, err := externalClient(35 * time.Second).Do(req)
		if err != nil {
			return "", errors.New("无法获取模型列表，请检查服务地址和网络")
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return "", externalHTTPError("获取模型列表", res.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, 2<<20+1))
		if err != nil || len(b) > 2<<20 {
			return "", errors.New("模型列表过大或读取失败")
		}
		var out struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			Models []struct {
				Name  string `json:"name"`
				Model string `json:"model"`
			} `json:"models"`
		}
		if json.Unmarshal(b, &out) != nil {
			return "", errors.New("模型列表不是有效的 JSON，请检查 API 地址；也可手动添加模型")
		}
		if c.Provider == "ollama" {
			for _, m := range out.Models {
				v := m.Name
				if v == "" {
					v = m.Model
				}
				models = append(models, v)
			}
		} else {
			for _, m := range out.Data {
				models = append(models, m.ID)
			}
		}
		if len(models) == 0 {
			return "", errors.New("服务未返回可用模型，可手动添加模型")
		}
		models, err = cleanModels(models)
		return "", err
	})
	return models, e
}

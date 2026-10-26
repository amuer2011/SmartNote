package main

import "smartnote/internal/core"

func (a *App) GetSyncConfig(id string) (core.SyncConfig, error) { return a.service.GetSyncConfig(id) }
func (a *App) SaveSyncConfig(id string, c core.SyncConfig, secret string, clear bool) (core.SyncConfig, error) {
	return a.service.SaveSyncConfig(id, c, secret, clear)
}
func (a *App) TestSync(id string) (string, error)           { return a.service.TestSync(id) }
func (a *App) StartSync(id string) (core.SyncStatus, error) { return a.service.StartSync(id) }
func (a *App) CancelSync(id string)                         { a.service.CancelSync(id) }
func (a *App) GetSyncStatus(id string) core.SyncStatus      { return a.service.SyncStatus(id) }
func (a *App) ListSyncConflicts(id string) ([]core.SyncConflict, error) {
	return a.service.ListSyncConflicts(id)
}
func (a *App) ReadSyncConflict(id, conflictID string) (core.SyncConflictPreview, error) {
	return a.service.ReadSyncConflict(id, conflictID)
}
func (a *App) ResolveSyncConflict(in core.SyncResolution) error {
	return a.service.ResolveSyncConflict(in)
}
func (a *App) GetLLMConfig() (core.LLMConfig, error) { return a.service.GetLLMConfig() }
func (a *App) SaveLLMConfig(c core.LLMConfig, secret string, clear bool) (core.LLMConfig, error) {
	return a.service.SaveLLMConfig(c, secret, clear)
}
func (a *App) TestLLM() (string, error) { return a.service.TestLLM() }
func (a *App) TestLLMConfig(c core.LLMConfig, secret string, clear bool) (string, error) {
	return a.service.TestLLMConfig(c, secret, clear)
}
func (a *App) GenerateLLM(in core.LLMInput) (core.LLMResult, error) { return a.service.GenerateLLM(in) }
func (a *App) CancelLLM(id string)                                  { a.service.CancelLLM(id) }
func (a *App) GetLLMProviders() (core.LLMProviders, error)          { return a.service.GetLLMProviders() }
func (a *App) SaveLLMProviders(state core.LLMProviders, secrets map[string]string, clearIDs []string) (core.LLMProviders, error) {
	return a.service.SaveLLMProviders(state, secrets, clearIDs)
}
func (a *App) ListLLMModels(p core.LLMProvider, secret string, clear bool) ([]string, error) {
	return a.service.ListLLMModels(p, secret, clear)
}
func (a *App) GetLLMReasoningCapability(p core.LLMProvider, secret string, clear bool) (core.LLMReasoningCapability, error) {
	return a.service.GetLLMReasoningCapability(p, secret, clear)
}
func (a *App) TestLLMProvider(p core.LLMProvider, secret string, clear bool) (string, error) {
	return a.service.TestLLMProvider(p, secret, clear)
}
func (a *App) ChatLLM(in core.LLMChatInput) (core.LLMChatResult, error) { return a.service.ChatLLM(in) }
func (a *App) ApplyLLMEdit(p core.LLMEditProposal) (core.Note, error) {
	return a.service.ApplyLLMEdit(p)
}

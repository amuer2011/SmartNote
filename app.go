package main

import (
	"context"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"path/filepath"
	"smartnote/internal/core"
	"smartnote/internal/embedding"
	"time"
)

type App struct {
	ctx         context.Context
	service     *core.Service
	dataDir     string
	watchCancel context.CancelFunc
	watchDone   chan struct{}
	syncDone    chan struct{}
}

func NewApp() (*App, error) {
	base, err := selectDataDirectory(dataDirectoryCandidates())
	if err != nil {
		return nil, err
	}
	s, err := core.New(base)
	if err != nil {
		return nil, err
	}
	return &App{service: s, dataDir: base}, nil
}

func (a *App) DataDirectory() string                        { return a.dataDir }
func (a *App) SetUIPreferences(ui core.UIPreferences) error { return a.service.SetUIPreferences(ui) }
func (a *App) ListWorkspaceFiles(id string) (core.WorkspaceFiles, error) {
	return a.service.ListWorkspaceFiles(id)
}
func (a *App) GetArchiveRules(workspaceID string) (core.ArchiveRulesConfig, error) {
	return a.service.GetArchiveRules(workspaceID)
}
func (a *App) SaveArchiveRules(workspaceID string, config core.ArchiveRulesConfig) (core.ArchiveRulesConfig, error) {
	return a.service.SaveArchiveRules(workspaceID, config)
}
func (a *App) SuggestArchive(workspaceID, noteID, hash string) (core.ArchiveSuggestion, error) {
	return a.service.SuggestArchive(workspaceID, noteID, hash)
}
func (a *App) ApplyArchive(input core.ArchiveInput) (core.ArchiveResult, error) {
	return a.service.ApplyArchive(input)
}
func (a *App) UndoArchive(workspaceID, operationID, hash string) (core.ArchiveResult, error) {
	return a.service.UndoArchive(workspaceID, operationID, hash)
}
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	watchCtx, cancel := context.WithCancel(ctx)
	a.watchCancel = cancel
	a.watchDone = make(chan struct{})
	go a.watchWorkspaces(watchCtx)
	a.syncDone = make(chan struct{})
	go func() { defer close(a.syncDone); a.service.RunSyncScheduler(watchCtx) }()
}
func (a *App) shutdown(ctx context.Context) {
	if a.watchCancel != nil {
		a.watchCancel()
		<-a.watchDone
		<-a.syncDone
	}
	_ = a.service.Close()
}
func (a *App) GetGraphLayouts(id string) (core.GraphLayouts, error) {
	return a.service.GetGraphLayouts(id)
}
func (a *App) SaveGraphLayout(id, key string, layout core.GraphLayout) error {
	return a.service.SaveGraphLayout(id, key, layout)
}

func (a *App) Snapshot() (core.Snapshot, error) {
	snapshot, err := a.service.Snapshot()
	if err == nil {
		a.service.QueueIndex(snapshot)
	}
	return snapshot, err
}
func (a *App) IndexStatus() core.IndexStatus { return a.service.IndexStatus() }
func (a *App) StartIndex(rebuild bool) (core.IndexStatus, error) {
	return a.service.StartIndex(rebuild)
}
func (a *App) CancelIndex() core.IndexStatus { return a.service.CancelIndex() }
func (a *App) SaveNote(in core.SaveInput) (core.Note, error) {
	n, e := a.service.SaveNote(in)
	if e == nil {
		a.service.RequestAutoSync()
	}
	return n, e
}
func (a *App) CreateNote(title, dir string) (core.Note, error) {
	return a.service.CreateNote(title, dir)
}
func (a *App) DeleteNote(id, hash string) error      { return a.service.DeleteNote(id, hash) }
func (a *App) OpenWorkspace(id string) error         { return a.service.OpenWorkspace(id) }
func (a *App) RemoveWorkspace(id string) error       { return a.service.RemoveWorkspace(id) }
func (a *App) RenameWorkspace(id, name string) error { return a.service.RenameWorkspace(id, name) }
func (a *App) ChooseWorkspace() (core.Workspace, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择或创建 Markdown 工作区", CanCreateDirectories: true})
	if err != nil {
		return core.Workspace{}, err
	}
	if path == "" {
		return core.Workspace{}, errors.New("已取消")
	}
	return a.service.AddWorkspace(filepath.Base(path), path)
}
func (a *App) SetPreferences(theme, picgo string) error {
	return a.service.SetPreferences(theme, picgo)
}
func (a *App) SetTask(id string, done, confirm bool) error {
	return a.service.SetTask(id, done, confirm)
}
func (a *App) UpdateTaskDetails(input core.TaskDetails) error {
	return a.service.UpdateTaskDetails(input)
}
func (a *App) MoveNote(id, hash, target string) (core.MoveResult, error) {
	return a.service.MoveNote(id, hash, target)
}
func (a *App) MoveDirectory(source, target string) (core.EntryResult, error) {
	return a.service.MoveDirectory(source, target)
}
func (a *App) CopyEntry(source string) (core.EntryResult, error) {
	return a.service.CopyEntry(source)
}
func (a *App) PasteEntry(source, directory string) (core.EntryResult, error) {
	return a.service.PasteEntry(source, directory)
}
func (a *App) ClipboardHasFiles() bool { return clipboardHasFiles() }
func (a *App) DeleteDirectory(path string) (core.EntryResult, error) {
	return a.service.DeleteDirectory(path)
}
func (a *App) CreateDirectory(path string) error                { return a.service.CreateDirectory(path) }
func (a *App) EntryLocation(path string) (string, error)        { return a.service.EntryLocation(path) }
func (a *App) Search(query string) ([]core.SearchResult, error) { return a.service.Search(query) }
func (a *App) Graph() ([]core.Edge, error)                      { return a.service.Graph() }
func (a *App) PreviewDirectoryLearning(workspaceID, categoryID string) (core.DirectoryLearningPreview, error) {
	return a.service.PreviewDirectoryLearning(workspaceID, categoryID)
}
func (a *App) SuggestGraphRelations(workspaceID, noteID string, threshold float64) (core.GraphSuggestions, error) {
	return a.service.SuggestGraphRelations(workspaceID, noteID, threshold)
}
func (a *App) SetGraphRelation(input core.GraphRelationInput) error {
	return a.service.SetGraphRelation(input)
}
func (a *App) GetGraphAnnotations(id string) (core.GraphAnnotations, error) {
	return a.service.GetGraphAnnotations(id)
}
func (a *App) SaveGraphAnnotation(input core.GraphAnnotationInput) (core.GraphAnnotations, error) {
	return a.service.SaveGraphAnnotation(input)
}
func (a *App) AttachmentUsage(id string) ([]core.AttachmentUsage, error) {
	return a.service.AttachmentUsage(id)
}
func (a *App) DeleteAttachments(input core.DeleteAttachmentsInput) (core.DeleteAttachmentsResult, error) {
	return a.service.DeleteAttachments(input)
}
func (a *App) ImportAttachment(name, encoded string, paste bool) (core.Attachment, error) {
	return a.service.ImportAttachment(name, encoded, paste)
}
func (a *App) AttachmentData(id string) (string, error) { return a.service.AttachmentData(id) }
func (a *App) SaveAttachment(id, name string) error {
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "附件另存为", DefaultFilename: name})
	if err != nil || dest == "" {
		return err
	}
	return a.service.CopyAttachment(id, dest)
}
func (a *App) Export(native bool) (string, error) {
	name := "SmartNote-Markdown.zip"
	if native {
		name = "SmartNote-Backup.zip"
	}
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "导出工作区", DefaultFilename: name, Filters: []runtime.FileFilter{{DisplayName: "ZIP 归档", Pattern: "*.zip"}}})
	if err != nil || dest == "" {
		return "", err
	}
	return dest, a.service.Export(dest, native)
}
func (a *App) ChooseImportFolder() (string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择已准备好的 Markdown / Obsidian 笔记文件夹"})
	if err == nil && path == "" {
		return "", errors.New("已取消")
	}
	return path, err
}
func (a *App) ImportFolder(input core.FolderImportInput) (core.FolderImportResult, error) {
	return a.service.ImportFolder(input)
}

func (a *App) Import() (core.Workspace, error) {
	source, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "导入标准 Markdown ZIP 或 SmartNote 原生备份", Filters: []runtime.FileFilter{{DisplayName: "ZIP 归档", Pattern: "*.zip"}}})
	if err != nil {
		return core.Workspace{}, err
	}
	if source == "" {
		return core.Workspace{}, errors.New("已取消")
	}
	parent, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择导入位置（将在其下新建工作区）", CanCreateDirectories: true})
	if err != nil {
		return core.Workspace{}, err
	}
	if parent == "" {
		return core.Workspace{}, errors.New("已取消")
	}
	dest := filepath.Join(parent, "SmartNote-导入-"+time.Now().Format("20060102-150405"))
	return a.service.Import(source, dest)
}

func (a *App) GetSemanticConfig(id string) (core.SemanticConfig, error) {
	return a.service.GetSemanticConfig(id)
}
func (a *App) SaveSemanticConfig(id string, c core.SemanticConfig) (core.SemanticConfig, error) {
	return a.service.SaveSemanticConfig(id, c)
}
func (a *App) SuggestSemanticArchive(id, noteID, hash string) (core.ArchiveSuggestion, error) {
	return a.service.SuggestSemanticArchive(id, noteID, hash)
}
func (a *App) SuggestSemanticArchiveBatch(id string, refs []core.SemanticArchiveRef) (core.SemanticArchiveBatch, error) {
	return a.service.SuggestSemanticArchiveBatch(id, refs)
}
func (a *App) ApplyReviewedSemanticArchive(revision string, input core.ArchiveInput) (core.ArchiveResult, error) {
	return a.service.ApplyReviewedSemanticArchive(revision, input)
}
func (a *App) AutoArchive(id, noteID, hash string) (*core.ArchiveResult, error) {
	return a.service.AutoArchive(id, noteID, hash)
}

func (a *App) EmbeddingStatus() embedding.Status         { return a.service.EmbeddingStatus() }
func (a *App) CheckEmbedding() (embedding.Status, error) { return a.service.CheckEmbedding() }

func (a *App) GetBookmarks(id string) ([]core.Bookmark, error) { return a.service.GetBookmarks(id) }
func (a *App) AddBookmark(input core.BookmarkInput) (core.Bookmark, error) {
	return a.service.AddBookmark(input)
}
func (a *App) DeleteBookmark(workspaceID, id string) error {
	return a.service.DeleteBookmark(workspaceID, id)
}

func (a *App) SemanticSearch(query string) (core.SearchResponse, error) {
	return a.service.SemanticSearch(query)
}

func (a *App) GetReminderConfig(workspaceID string) (core.ReminderConfig, error) {
	return a.service.GetReminderConfig(workspaceID)
}
func (a *App) SaveReminderConfig(workspaceID string, c core.ReminderConfig) (core.ReminderConfig, error) {
	return a.service.SaveReminderConfig(workspaceID, c)
}
func (a *App) DueReminders(workspaceID string) ([]core.Task, error) {
	return a.service.DueReminders(workspaceID)
}
func (a *App) ActOnReminder(input core.ReminderAction) error { return a.service.ActOnReminder(input) }

func (a *App) GetLunarYear(year int) (core.LunarYear, error) { return a.service.GetLunarYear(year) }
func (a *App) GetHolidayYear(year int) (core.HolidayYear, error) {
	return a.service.GetHolidayYear(year)
}

func (a *App) SetTaskOccurrence(workspaceID, id, date string, done bool) error {
	return a.service.SetTaskOccurrence(workspaceID, id, date, done)
}
func (a *App) SetRecurringEnabled(workspaceID string, ids []string, enabled bool) error {
	return a.service.SetRecurringEnabled(workspaceID, ids, enabled)
}

func (a *App) GetTaskArchive(workspaceID string) ([]core.TaskArchiveRecord, error) {
	return a.service.GetTaskArchive(workspaceID)
}

func (a *App) ResolveTaskIntent(input core.TaskIntentDecision) error {
	return a.service.ResolveTaskIntent(input)
}
func (a *App) IgnoreAllTaskIntents(input core.TaskIntentBatch) (int, error) {
	return a.service.IgnoreAllTaskIntents(input)
}
func (a *App) SetNoteAutomationIgnored(workspaceID, noteID, hash string, ignored bool) error {
	return a.service.SetNoteAutomationIgnored(workspaceID, noteID, hash, ignored)
}
func (a *App) SetAttachmentPreferences(picgo string, preferences core.AttachmentPreferences) error {
	return a.service.SetAttachmentPreferences(picgo, preferences)
}

func (a *App) ListTrash(workspaceID string) ([]core.TrashEntry, error) {
	return a.service.ListTrash(workspaceID)
}
func (a *App) ReadTrashFile(workspaceID, source, path string) (string, error) {
	return a.service.ReadTrashFile(workspaceID, source, path)
}
func (a *App) DeleteTrashEntry(workspaceID, source, path string) error {
	return a.service.DeleteTrashEntry(workspaceID, source, path)
}
func (a *App) EmptyTrash(workspaceID string) error { return a.service.EmptyTrash(workspaceID) }

func (a *App) PreviewAttachment(workspaceID, id string) (core.AttachmentPreview, error) {
	return a.service.PreviewAttachment(workspaceID, id)
}

func (a *App) GetAssistantMemory(workspaceID string) (core.AssistantMemory, error) {
	return a.service.GetAssistantMemory(workspaceID)
}
func (a *App) SaveAssistantMemory(workspaceID string, in core.AssistantMemory) (core.AssistantMemory, error) {
	return a.service.SaveAssistantMemory(workspaceID, in)
}
func (a *App) ListAssistantConversations(workspaceID string) ([]core.AssistantConversationInfo, error) {
	return a.service.ListAssistantConversations(workspaceID)
}
func (a *App) GetAssistantConversation(workspaceID, id string) (core.AssistantConversation, error) {
	return a.service.GetAssistantConversation(workspaceID, id)
}
func (a *App) SaveAssistantConversation(workspaceID string, in core.AssistantConversation) (core.AssistantConversation, error) {
	return a.service.SaveAssistantConversation(workspaceID, in)
}
func (a *App) DeleteAssistantConversation(workspaceID, id, revision string) error {
	return a.service.DeleteAssistantConversation(workspaceID, id, revision)
}

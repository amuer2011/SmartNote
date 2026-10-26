export interface AttachmentPreview {kind:string;content:string;message:string}
export interface TrashEntry {source:string;path:string;name:string;directory:boolean;size:number;modified:string}
export interface TaskArchiveRecord {id:string;taskId:string;noteId:string;noteTitle:string;title:string;notePath:string;dueAt:string;completedAt:string;reopenedAt?:string;imported?:boolean;sourceRevision?:string;line:number;priority:string}
export interface HolidayYear {year:number;holiday:Record<string,{holiday:boolean;name:string;date:string;wage:number}>;type:Record<string,{type:number;name:string;week:number}>}
export interface LunarYear {year:number;version:number;days:Record<string,{month:string;day:string;term:string}>}
export interface ReminderConfig {revision:string;enabled:boolean;time:string;leadDays:number}
export interface IndexStatus {workspaceId:string;state:string;completed:number;total:number;notes:number;message:string;updated:string}
export interface Bookmark {kind?:'note'|'position';id:string;noteId:string;label:string;line:number;column:number;text:string;before:string;after:string;created:string}
export interface BookmarkInput {kind?:'note'|'position';workspaceId:string;noteId:string;hash:string;line:number;column:number;label:string}
export interface FolderImportInput {source:string;organize:boolean;templateWorkspaceId:string}
export interface FolderImportResult {workspace:Workspace;notes:number;attachments:number;skipped:number;warningCount:number;warnings:string[]}
export interface Workspace { id: string; name: string; path: string }
export interface Note { id: string; path: string; title: string; content: string; hash: string; tags: string[]; project: string; updated: string; taskArchived?: boolean; automationIgnored?:boolean }
export interface Recurrence {until?:string;unit:string;interval:number;anchor:string;day?:number;source?:string}
export interface Task { sourceRevision?:string;needsConfirmation?:boolean;intentDecision?:string; recurrence?:Recurrence;completedDates?:string[]; inferred?:boolean; id: string; noteId: string; noteTitle: string; title: string; line: number; done: boolean; candidate: boolean; dueAt: string; priority: string; project: string; tags: string[] }
export interface Attachment { id: string; name: string; mime: string; size: number; sha256: string; remoteUrl?: string; remoteOnly?:boolean; created: string }
export interface LayoutPreferences {sidebarWidth:number;listWidth:number;inspectorWidth:number;splitRatio:number;hiddenPanels:string[];rightTabOrder?:string[];hiddenRightTabs?:string[]}
export interface UIPreferences { layout?:LayoutPreferences; shortcuts?: Record<string,string>; editorFontFamily?: string; editorFontSize?: number; fontSize: number; fontFamily?: string; collapsedWorkspaces: string[]; entryOrder: Record<string, Record<string, string[]>> }
export interface EntryResult { path: string; recoveryPath: string; updatedLinks: number }
export interface WorkspaceFiles { notes: Note[]; directories: string[] }
export interface AttachmentPreferences {picGoEnabled:boolean;keepOriginal:boolean;deduplicate:boolean}
export interface Settings { attachments?:AttachmentPreferences; workspaces: Workspace[]; activeId: string; picGoUrl: string; theme: string; ui?: UIPreferences }
export interface Snapshot { settings: Settings; notes: Note[]; tasks: Task[]; attachments: Attachment[]; directories?: string[] }
export interface SearchResponse {results:SearchResult[];mode:string;message:string}
export interface SearchResult {semanticScore?:number;matchType?:string; note: Note; snippet: string; line: number; score: number }
export interface GraphAnnotations {revision:string;nodes:Record<string,string>;relations:Record<string,string>}
export interface GraphAnnotationInput {workspaceId:string;revision:string;noteId?:string;source?:string;target?:string;text:string}
export interface AttachmentUsage {id:string;notes:number;trash:number}
export interface DeleteAttachmentsResult {deleted:string[];skipped:Record<string,string>;warnings:string[]}
export interface Edge { annotation?:string; source: string; target: string; type: string; score?:number;sourceHash?:string;targetHash?:string }
export interface GraphSuggestions {edges:Edge[];message:string}
export interface GraphRelationInput {workspaceId:string;source:string;target:string;sourceHash:string;targetHash:string;confirmed:boolean}
export interface SemanticArchiveRef {noteId:string;hash:string;path:string}
export interface DirectoryLearningPreview {directory:string;notes:(SemanticArchiveRef&{title:string})[];total:number}
export interface SemanticCategory {id:string;name:string;description:string;directory:string;enabled:boolean;examples?:SemanticArchiveRef[]}
export interface EmbeddingStatus {state:string;model:string;dimensions:number;message:string;cached:number}
export interface SemanticConfig {revision:string;endpoint?:string;model:string;automatic:boolean;automaticVersion?:number;threshold:number;categories:SemanticCategory[]}
export interface ArchiveRule {id:string;name:string;enabled:boolean;field:'title'|'content'|'project'|'tag';match:'any'|'all';keywords:string[];directory:string;tags:string[]}
export interface ArchiveRulesConfig {revision:string;rules:ArchiveRule[]}
export interface ArchiveSuggestion {score:number;workspaceId:string;noteId:string;hash:string;directory:string;tags:string[];reasons:string[];source:string;ruleId:string}
export interface SemanticArchiveBatchItem {noteId:string;suggestion?:ArchiveSuggestion;error?:string}
export interface SemanticArchiveBatch {revision:string;items:SemanticArchiveBatchItem[]}
export interface ArchiveInput {workspaceId:string;noteId:string;hash:string;sourcePath?:string;directory:string;tags:string[]}
export interface ArchiveResult {note:Note;updatedLinks:number;recoveryPath:string;operationId:string}
export interface GraphLayout {positions:Record<string,{x:number;y:number}>;zoom:number;pan:{x:number;y:number}}
export interface SyncConfig {enabled:boolean;mode:'bidirectional'|'backup';endpoint:string;username:string;remoteId:string;intervalMinutes:number;revision:string;hasSecret:boolean}
export interface SyncStatus {workspaceId:string;state:string;message:string;completed:number;total:number;conflicts:number;lastSuccess:string;updated:string}
export interface SyncConflict {remotePath?:string;id:string;path:string;localHash:string;remoteHash:string;created:string;localDeleted:boolean;remoteDeleted:boolean}
export interface SyncConflictPreview {conflict:SyncConflict;text:boolean;local:string;remote:string}
export interface LLMConfig {providerId?:string;enabled:boolean;provider:'openai'|'ollama';endpoint:string;model:string;reasoningEffort:string;hasSecret:boolean;revision:string}
export interface LLMProvider extends LLMConfig {id:string;name:string;models:string[];selectedModels:string[]}
export interface LLMProviders {enabled:boolean;defaultId:string;revision:string;providers:LLMProvider[]}
export interface LLMReasoningCapability {known:boolean;efforts:string[];message:string}
export interface AssistantAttachment {id:string;name:string;mime:string;data:string}
export interface LLMChatMessage {role:'user'|'assistant';content:string;sources?:LLMSource[];attachments?:AssistantAttachment[]}
export interface AssistantMemory {revision:string;text:string}
export interface AssistantEntry {role:'user'|'assistant';content:string;sources:LLMSource[];service?:string;notice?:boolean;attachments?:AssistantAttachment[]}
export interface AssistantConversation {id:string;revision:string;title:string;updated:string;entries:AssistantEntry[]}
export interface AssistantConversationInfo {id:string;title:string;updated:string;messages:number}
export interface LLMChatInput {requestId:string;workspaceId:string;providerId:string;model:string;reasoningEffort:string;messages:LLMChatMessage[];noteId:string;hash:string;scope:'note'|'workspace';selection:string}
export interface LLMEditProposal {id:string;workspaceId:string;noteId:string;title:string;path:string;hash:string;original:string;content:string;oldText:string;newText:string;reason:string}
export interface LLMChatResult {text:string;sources:LLMSource[];proposals:LLMEditProposal[];steps:string[]}
export interface LLMSource {noteId:string;hash:string;title:string;path:string;line:number}
export interface LLMInput {requestId:string;workspaceId:string;noteId?:string;hash?:string;from?:number;to?:number;text?:string;action:'polish'|'structure'|'summary';query?:string;sources?:LLMSource[]}
export interface LLMResult {text:string;original:string;sources:LLMSource[]}
type Backend = {
 GetSyncConfig(id:string):Promise<SyncConfig>
 SaveSyncConfig(id:string,config:SyncConfig,secret:string,clear:boolean):Promise<SyncConfig>
 TestSync(id:string):Promise<string>
 StartSync(id:string):Promise<SyncStatus>
 CancelSync(id:string):Promise<void>
 GetSyncStatus(id:string):Promise<SyncStatus>
 ListSyncConflicts(id:string):Promise<SyncConflict[]>
 ReadSyncConflict(id:string,conflictId:string):Promise<SyncConflictPreview>
 ResolveSyncConflict(input:{workspaceId:string;id:string;choice:'local'|'remote'|'merge';merged:string}):Promise<void>
 GetLLMConfig():Promise<LLMConfig>
 GetLLMProviders():Promise<LLMProviders>
 SaveLLMProviders(state:LLMProviders,secrets:Record<string,string>,clearIds:string[]):Promise<LLMProviders>
 ListLLMModels(provider:LLMProvider,secret:string,clear:boolean):Promise<string[]>
 GetLLMReasoningCapability(provider:LLMProvider,secret:string,clear:boolean):Promise<LLMReasoningCapability>
 TestLLMProvider(provider:LLMProvider,secret:string,clear:boolean):Promise<string>
 GetAssistantMemory(workspaceId:string):Promise<AssistantMemory>
 SaveAssistantMemory(workspaceId:string,memory:AssistantMemory):Promise<AssistantMemory>
 ListAssistantConversations(workspaceId:string):Promise<AssistantConversationInfo[]>
 GetAssistantConversation(workspaceId:string,id:string):Promise<AssistantConversation>
 SaveAssistantConversation(workspaceId:string,conversation:AssistantConversation):Promise<AssistantConversation>
 DeleteAssistantConversation(workspaceId:string,id:string,revision:string):Promise<void>
 ChatLLM(input:LLMChatInput):Promise<LLMChatResult>
 ApplyLLMEdit(proposal:LLMEditProposal):Promise<Note>
 SaveLLMConfig(config:LLMConfig,secret:string,clearSecret:boolean):Promise<LLMConfig>
 TestLLM():Promise<string>
 TestLLMConfig(config:LLMConfig,secret:string,clearSecret:boolean):Promise<string>
 GenerateLLM(input:LLMInput):Promise<LLMResult>
 CancelLLM(id:string):Promise<void>
 PreviewDirectoryLearning(workspaceId:string,categoryId:string):Promise<DirectoryLearningPreview>
 GetGraphAnnotations(id:string):Promise<GraphAnnotations>
 SaveGraphAnnotation(input:GraphAnnotationInput):Promise<GraphAnnotations>
 AttachmentUsage(id:string):Promise<AttachmentUsage[]>
 DeleteAttachments(input:{workspaceId:string;ids:string[];force:boolean}):Promise<DeleteAttachmentsResult>
 SuggestGraphRelations(workspaceId:string,noteId:string,threshold:number):Promise<GraphSuggestions>
 SetGraphRelation(input:GraphRelationInput):Promise<void>
 ResolveTaskIntent(input:{workspaceId:string;taskId:string;sourceRevision:string;accept:boolean;until?:string;ignoreNote?:boolean}):Promise<void>
 IgnoreAllTaskIntents(input:{workspaceId:string;tasks:{taskId:string;sourceRevision:string}[]}):Promise<number>
 GetGraphLayouts(id:string):Promise<Record<string,GraphLayout>>
 SaveGraphLayout(id:string,key:string,layout:GraphLayout):Promise<void>
 GetTaskArchive(workspaceID:string):Promise<TaskArchiveRecord[]>
 SetTaskOccurrence(workspaceID:string,id:string,date:string,done:boolean):Promise<void>
 SetRecurringEnabled(workspaceID:string,ids:string[],enabled:boolean):Promise<void>
 GetHolidayYear(year:number):Promise<HolidayYear>
 GetLunarYear(year:number):Promise<LunarYear>
 GetReminderConfig(id:string):Promise<ReminderConfig>
 SaveReminderConfig(id:string,config:ReminderConfig):Promise<ReminderConfig>
 DueReminders(id:string):Promise<Task[]>
 ActOnReminder(input:{workspaceId:string;taskId:string;dueAt:string;action:string;minutes:number}):Promise<void>
 IndexStatus():Promise<IndexStatus>
 StartIndex(rebuild:boolean):Promise<IndexStatus>
 CancelIndex():Promise<IndexStatus>
 EmbeddingStatus():Promise<EmbeddingStatus>
 CheckEmbedding():Promise<EmbeddingStatus>
 GetBookmarks(id:string):Promise<Bookmark[]>
 AddBookmark(input:BookmarkInput):Promise<Bookmark>
 DeleteBookmark(workspaceID:string,id:string):Promise<void>
 GetSemanticConfig(workspaceId:string):Promise<SemanticConfig>
 SaveSemanticConfig(workspaceId:string,config:SemanticConfig):Promise<SemanticConfig>
 SuggestSemanticArchive(workspaceId:string,noteId:string,hash:string):Promise<ArchiveSuggestion>
 SuggestSemanticArchiveBatch(workspaceId:string,refs:{noteId:string;hash:string;path:string}[]):Promise<SemanticArchiveBatch>
 ApplyReviewedSemanticArchive(revision:string,input:ArchiveInput):Promise<ArchiveResult>
 AutoArchive(workspaceId:string,noteId:string,hash:string):Promise<ArchiveResult|null>
 GetArchiveRules(workspaceId:string):Promise<ArchiveRulesConfig>
 SaveArchiveRules(workspaceId:string,config:ArchiveRulesConfig):Promise<ArchiveRulesConfig>
 SuggestArchive(workspaceId:string,noteId:string,hash:string):Promise<ArchiveSuggestion>
 ApplyArchive(input:ArchiveInput):Promise<ArchiveResult>
 UndoArchive(workspaceId:string,operationId:string,hash:string):Promise<ArchiveResult>
 MoveDirectory(source:string,target:string): Promise<EntryResult>
 CopyEntry(source:string): Promise<EntryResult>
 PasteEntry(source:string,directory:string):Promise<EntryResult>
 ClipboardHasFiles():Promise<boolean>
 DeleteDirectory(path:string): Promise<EntryResult>
 CreateDirectory(path:string): Promise<void>
 EntryLocation(path:string): Promise<string>
 RevealEntry(path:string): Promise<void>
 SetUIPreferences(ui: UIPreferences): Promise<void>
 ListTrash(workspaceId:string):Promise<TrashEntry[]>
 ReadTrashFile(workspaceId:string,source:string,path:string):Promise<string>
 DeleteTrashEntry(workspaceId:string,source:string,path:string):Promise<void>
 EmptyTrash(workspaceId:string):Promise<void>
 ListWorkspaceFiles(id: string): Promise<WorkspaceFiles>
 DataDirectory(): Promise<string>
 Snapshot(): Promise<Snapshot>; SaveNote(input: { id: string; path: string; content: string; hash: string }): Promise<Note>
 CreateNote(title: string, dir: string): Promise<Note>; DeleteNote(id: string, hash: string): Promise<void>
 MoveNote(id: string, hash: string, target: string): Promise<{ note: Note; updatedLinks: number; recoveryPath: string }>
 UpdateTaskDetails(input: {id: string; dueAt: string; priority: string}): Promise<void>
 ChooseImportFolder():Promise<string>; ImportFolder(input:FolderImportInput):Promise<FolderImportResult>
 ChooseWorkspace(): Promise<Workspace>; OpenWorkspace(id: string): Promise<void>; RemoveWorkspace(id: string): Promise<void>; RenameWorkspace(id: string, name: string): Promise<void>
 SetAttachmentPreferences(picgo:string,preferences:AttachmentPreferences):Promise<void>
 SetNoteAutomationIgnored(workspaceId:string,noteId:string,hash:string,ignored:boolean):Promise<void>
 SetPreferences(theme: string, picgo: string): Promise<void>; SetTask(id: string, done: boolean, confirm: boolean): Promise<void>
 SemanticSearch(query:string):Promise<SearchResponse>
 Search(query: string): Promise<SearchResult[]>; Graph(): Promise<Edge[]>
 ImportAttachment(name: string, base64: string, paste: boolean): Promise<Attachment>; PreviewAttachment(workspaceId:string,id:string):Promise<AttachmentPreview>
 AttachmentData(id: string): Promise<string>
 SaveAttachment(id: string, name: string): Promise<void>; Export(native: boolean): Promise<string>; Import(): Promise<Workspace>
}
declare global { interface Window { go?: { main: { App: Backend } }; runtime?: { EventsOn(name: string, callback: () => void): () => void; BrowserOpenURL(url: string): void } } }
export const desktop = !!window.go?.main.App
export const api = new Proxy({} as Backend, { get: (_, key) => {
 if (typeof key !== 'string' || key.startsWith('__')) return undefined
 return (...args: unknown[]) => {
 if (!window.go?.main.App) return Promise.reject(new Error('请在 SmartNote 桌面应用中使用文件操作。浏览器预览仅展示界面。'))
 const method = window.go.main.App[key as keyof Backend] as (...a: unknown[]) => Promise<unknown>
 return method(...args)
} } })

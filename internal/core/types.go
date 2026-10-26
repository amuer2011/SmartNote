package core

import "time"

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}
type Settings struct {
	Attachments AttachmentPreferences `json:"attachments"`
	UI          UIPreferences         `json:"ui"`
	Workspaces  []Workspace           `json:"workspaces"`
	ActiveID    string                `json:"activeId"`
	PicGoURL    string                `json:"picGoUrl"`
	Theme       string                `json:"theme"`
}
type AttachmentPreferences struct {
	PicGoEnabled bool `json:"picGoEnabled"`
	KeepOriginal bool `json:"keepOriginal"`
	Deduplicate  bool `json:"deduplicate"`
}
type UIPreferences struct {
	Layout              LayoutPreferences              `json:"layout"`
	Shortcuts           map[string]string              `json:"shortcuts,omitempty"`
	EditorFontFamily    string                         `json:"editorFontFamily"`
	EditorFontSize      int                            `json:"editorFontSize"`
	FontFamily          string                         `json:"fontFamily"`
	FontSize            int                            `json:"fontSize"`
	CollapsedWorkspaces []string                       `json:"collapsedWorkspaces"`
	EntryOrder          map[string]map[string][]string `json:"entryOrder"`
}
type LayoutPreferences struct {
	RightTabOrder   []string `json:"rightTabOrder,omitempty"`
	HiddenRightTabs []string `json:"hiddenRightTabs,omitempty"`
	SidebarWidth    int      `json:"sidebarWidth"`
	ListWidth       int      `json:"listWidth"`
	InspectorWidth  int      `json:"inspectorWidth"`
	SplitRatio      int      `json:"splitRatio"`
	HiddenPanels    []string `json:"hiddenPanels"`
}
type WorkspaceFiles struct {
	Notes       []Note   `json:"notes"`
	Directories []string `json:"directories"`
}
type Note struct {
	AutomationIgnored bool      `json:"automationIgnored,omitempty"`
	TaskArchived      bool      `json:"taskArchived,omitempty"`
	SyntheticID       bool      `json:"-"`
	ID                string    `json:"id"`
	Path              string    `json:"path"`
	Title             string    `json:"title"`
	Content           string    `json:"content"`
	Hash              string    `json:"hash"`
	Tags              []string  `json:"tags"`
	Project           string    `json:"project"`
	Updated           time.Time `json:"updated"`
	Created           time.Time `json:"created"`
}
type Task struct {
	IgnoreNoteAutomation bool                `json:"ignoreNoteAutomation,omitempty"`
	IntentDecision       string              `json:"intentDecision,omitempty"`
	NeedsConfirmation    bool                `json:"needsConfirmation,omitempty"`
	SourceRevision       string              `json:"sourceRevision,omitempty"`
	History              []TaskArchiveRecord `json:"history,omitempty"`
	Recurrence           *Recurrence         `json:"recurrence,omitempty"`
	CompletedDates       []string            `json:"completedDates,omitempty"`
	Deleted              bool                `json:"deleted,omitempty"`
	Inferred             bool                `json:"inferred"`
	ID                   string              `json:"id"`
	NoteID               string              `json:"noteId"`
	NoteTitle            string              `json:"noteTitle"`
	Title                string              `json:"title"`
	Line                 int                 `json:"line"`
	Done                 bool                `json:"done"`
	Candidate            bool                `json:"candidate"`
	DueAt                string              `json:"dueAt"`
	Priority             string              `json:"priority"`
	Project              string              `json:"project"`
	Tags                 []string            `json:"tags"`
}

type TaskDetails struct {
	ID       string `json:"id"`
	DueAt    string `json:"dueAt"`
	Priority string `json:"priority"`
}
type Attachment struct {
	Deleted    bool      `json:"deleted,omitempty"`
	RemoteOnly bool      `json:"remoteOnly,omitempty"`
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Mime       string    `json:"mime"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"`
	RemoteURL  string    `json:"remoteUrl,omitempty"`
	Created    time.Time `json:"created"`
}
type Snapshot struct {
	Directories []string     `json:"directories"`
	Settings    Settings     `json:"settings"`
	Notes       []Note       `json:"notes"`
	Tasks       []Task       `json:"tasks"`
	Attachments []Attachment `json:"attachments"`
}
type SaveInput struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash"`
}
type SearchResult struct {
	SemanticScore float64 `json:"semanticScore"`
	MatchType     string  `json:"matchType"`
	Note          Note    `json:"note"`
	Snippet       string  `json:"snippet"`
	Line          int     `json:"line"`
	Score         float64 `json:"score"`
}
type Edge struct {
	Annotation string  `json:"annotation,omitempty"`
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Type       string  `json:"type"`
	Score      float64 `json:"score"`
	SourceHash string  `json:"sourceHash,omitempty"`
	TargetHash string  `json:"targetHash,omitempty"`
}
type Relation struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

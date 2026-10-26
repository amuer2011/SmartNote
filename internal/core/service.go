package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"smartnote/internal/embedding"
	"sort"
	"strings"
	"sync"
	"time"
)

type Service struct {
	externalMu      sync.Mutex
	externalClosed  bool
	externalWG      sync.WaitGroup
	externalCancel  context.CancelFunc
	externalContext context.Context
	secrets         secretStore
	syncStates      map[string]SyncStatus
	syncCancels     map[string]context.CancelFunc
	syncPending     map[string]time.Time
	llmCancels      map[string]context.CancelFunc
	indexMu         sync.Mutex
	indexCancel     context.CancelFunc
	indexDone       chan struct{}
	indexWG         sync.WaitGroup
	indexGeneration uint64
	indexState      IndexStatus
	indexAttempt    map[string]string
	indexClosed     bool
	mu              sync.Mutex
	root            string
	settings        Settings
	db              *sql.DB
	model           *embedding.Engine
	embedTexts      func([]string) ([][]float64, error)
}

func New(root string) (*Service, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	s := &Service{root: root, settings: Settings{Workspaces: []Workspace{}, Theme: "light", Attachments: AttachmentPreferences{PicGoEnabled: true, KeepOriginal: true, Deduplicate: true}}}
	s.externalContext, s.externalCancel = context.WithCancel(context.Background())
	s.secrets = systemSecrets{}
	s.syncStates = map[string]SyncStatus{}
	s.syncCancels = map[string]context.CancelFunc{}
	s.syncPending = map[string]time.Time{}
	s.llmCancels = map[string]context.CancelFunc{}
	if err := readJSON(filepath.Join(root, "settings.json"), &s.settings); err != nil {
		return nil, fmt.Errorf("读取设置: %w", err)
	}
	if err := s.recoverSyncs(); err != nil {
		return nil, err
	}
	if err := s.recoverMoves(); err != nil {
		return nil, err
	}
	if err := s.recoverDirectories(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "index.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s.db = db
	s.model = embedding.New(filepath.Join(root, "models"))
	s.embedTexts = s.model.Embed
	_, err = db.Exec(`PRAGMA journal_mode=WAL; CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(workspace UNINDEXED, id UNINDEXED, title, content, tokenize='trigram'); CREATE TABLE IF NOT EXISTS note_fingerprints(workspace TEXT NOT NULL, id TEXT NOT NULL, hash TEXT NOT NULL, PRIMARY KEY(workspace,id)); CREATE TABLE IF NOT EXISTS passage_vectors(workspace TEXT NOT NULL, note TEXT NOT NULL, model TEXT NOT NULL, fingerprint TEXT NOT NULL, vector BLOB NOT NULL, PRIMARY KEY(workspace,note,model,fingerprint));`)
	if err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS prototype_vectors(workspace TEXT NOT NULL, fingerprint TEXT NOT NULL, vector BLOB NOT NULL, PRIMARY KEY(workspace,fingerprint))`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Service) Close() error {
	s.externalMu.Lock()
	s.externalClosed = true
	s.externalCancel()
	s.externalMu.Unlock()
	s.externalWG.Wait()
	s.indexMu.Lock()
	s.indexClosed = true
	if s.indexCancel != nil {
		s.indexCancel()
	}
	s.indexMu.Unlock()
	s.indexWG.Wait()

	modelErr := s.model.Close()
	dbErr := s.db.Close()
	return errors.Join(modelErr, dbErr)
}
func (s *Service) active() (Workspace, error) {
	for _, w := range s.settings.Workspaces {
		if w.ID == s.settings.ActiveID {
			return w, nil
		}
	}
	return Workspace{}, errors.New("请先创建或打开工作区")
}
func (s *Service) data(w Workspace) string { return filepath.Join(s.root, "workspaces", w.ID) }
func (s *Service) persist() error {
	return writeJSON(filepath.Join(s.root, "settings.json"), s.settings)
}
func (s *Service) Settings() Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.settings }
func (s *Service) SetPreferences(theme, picgo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if theme != "light" && theme != "dark" {
		return errors.New("无效主题")
	}
	s.settings.Theme = theme
	s.settings.PicGoURL = strings.TrimRight(picgo, "/")
	return s.persist()
}
func (s *Service) AddWorkspace(name, path string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return Workspace{}, err
	}
	if err = os.MkdirAll(abs, 0755); err != nil {
		return Workspace{}, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return Workspace{}, err
	}
	if err = probeWorkspaceWrite(abs); err != nil {
		return Workspace{}, fmt.Errorf("工作区无法写入（%s），请检查目录权限或程序的 Windows 完整性级别：%w", abs, err)
	}
	for _, w := range s.settings.Workspaces {
		if strings.EqualFold(w.Path, abs) {
			s.settings.ActiveID = w.ID
			return w, s.persist()
		}
	}
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(abs)
	}
	w := Workspace{ID: uid(), Name: name, Path: abs}
	if err = os.MkdirAll(s.data(w), 0700); err != nil {
		return Workspace{}, err
	}
	s.settings.Workspaces = append(s.settings.Workspaces, w)
	s.settings.ActiveID = w.ID
	return w, s.persist()
}
func (s *Service) OpenWorkspace(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.settings.Workspaces {
		if w.ID == id {
			if err := probeWorkspaceWrite(w.Path); err != nil {
				return fmt.Errorf("工作区无法写入（%s）：%w", w.Path, err)
			}
			s.settings.ActiveID = id
			return s.persist()
		}
	}
	return errors.New("工作区不存在")
}
func (s *Service) RemoveWorkspace(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.settings.Workspaces {
		if w.ID == id {
			s.CancelSync(id)
			s.settings.Workspaces = append(s.settings.Workspaces[:i], s.settings.Workspaces[i+1:]...)
			if s.settings.ActiveID == id {
				s.settings.ActiveID = ""
				if len(s.settings.Workspaces) > 0 {
					s.settings.ActiveID = s.settings.Workspaces[0].ID
				}
			}
			return s.persist()
		}
	}
	return errors.New("工作区不存在")
}
func (s *Service) RenameWorkspace(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(name) == "" {
		return errors.New("名称不能为空")
	}
	for i := range s.settings.Workspaces {
		if s.settings.Workspaces[i].ID == id {
			s.settings.Workspaces[i].Name = name
			return s.persist()
		}
	}
	return errors.New("工作区不存在")
}

func (s *Service) scan(w Workspace) ([]Note, error) {
	notes := []Note{}
	metadata := map[string]Task{}
	if err := readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &metadata); err != nil {
		return nil, err
	}
	ids := map[string]string{}
	err := filepath.WalkDir(w.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != w.Path && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(w.Path, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > 32<<20 {
			return fmt.Errorf("笔记超过 32 MB: %s", rel)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		n := parseNote(rel, b, info.ModTime())
		n.AutomationIgnored = metadata[n.ID+"_intent"].IgnoreNoteAutomation
		if prev, ok := ids[n.ID]; ok {
			return fmt.Errorf("重复笔记 ID: %s 与 %s", prev, rel)
		}
		ids[n.ID] = rel
		notes = append(notes, n)
		return nil
	})
	sort.Slice(notes, func(i, j int) bool { return notes[i].Updated.After(notes[j].Updated) })
	return notes, err
}
func (s *Service) Snapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Snapshot{Settings: s.settings, Notes: []Note{}, Tasks: []Task{}, Attachments: []Attachment{}}
	w, err := s.active()
	if err != nil {
		return out, nil
	}
	notes, err := s.scan(w)
	if err != nil {
		return out, err
	}
	out.Notes = notes
	out.Directories, err = workspaceDirectories(w.Path)
	if err != nil {
		return out, err
	}
	confirmed := map[string]Task{}
	if err = readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &confirmed); err != nil {
		return out, err
	}
	historyChanged := false
	for _, n := range notes {
		for _, t := range tasksFor(n) {
			if stored, ok := confirmed[t.ID]; ok {
				t = mergeTask(t, stored)
			}
			before := len(t.History)
			if t.Done {
				t = taskHistory(t, t.DueAt, true, n, true)
			}
			for _, date := range t.CompletedDates {
				t = taskHistory(t, date, true, n, true)
			}
			if len(t.History) != before {
				confirmed[t.ID] = t
				historyChanged = true
			}
			if !t.Deleted && t.IntentDecision != "rejected" {
				t.NeedsConfirmation = needsTaskConfirmation(t)
				if t.NeedsConfirmation {
					t.Candidate = true
				}
				out.Tasks = append(out.Tasks, t)
			}
		}
	}
	if historyChanged {
		if err = writeJSON(filepath.Join(s.data(w), "tasks-metadata.json"), confirmed); err != nil {
			return out, err
		}
	}
	if err = s.projectTaskArchive(w, archiveRecords(confirmed)); err != nil {
		return out, err
	}
	// Archive notes created during this refresh must immediately appear in navigation.
	if len(archiveRecords(confirmed)) > 0 {
		out.Notes, err = s.scan(w)
		if err != nil {
			return out, err
		}
		archived := map[string]bool{}
		for _, note := range out.Notes {
			if isTaskArchiveNote(note) {
				archived[note.ID] = true
			}
		}
		visibleTasks := out.Tasks[:0]
		for _, task := range out.Tasks {
			if !archived[task.NoteID] {
				visibleTasks = append(visibleTasks, task)
			}
		}
		out.Tasks = visibleTasks
		out.Directories, err = workspaceDirectories(w.Path)
		if err != nil {
			return out, err
		}
	}
	err = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &out.Attachments)
	visible := []Attachment{}
	for _, a := range out.Attachments {
		if !a.Deleted {
			visible = append(visible, a)
		}
	}
	out.Attachments = visible
	return out, err
}
func (s *Service) SaveNote(in SaveInput) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return Note{}, err
	}
	return s.saveNoteLocked(w, in)
}
func (s *Service) saveNoteLocked(w Workspace, in SaveInput) (Note, error) {
	previousRevision := ""
	if path, pathErr := safePath(w.Path, in.Path); pathErr == nil {
		if b, readErr := os.ReadFile(path); readErr == nil {
			previousRevision = noteTaskRevision(parseNote(in.Path, b, time.Now()))
		}
	}
	n, e := s.save(w, in)
	if e != nil {
		return n, e
	}
	if previousRevision != noteTaskRevision(n) {
		if e = s.recalculateNoteTasks(w, n); e != nil {
			return n, fmt.Errorf("笔记已保存，待办重新计算失败：%w", e)
		}
	}
	// Completing checkboxes while editing can move this note during recalculation.
	if notes, scanErr := s.scan(w); scanErr == nil {
		for _, fresh := range notes {
			if fresh.ID == n.ID {
				return fresh, nil
			}
		}
	}
	return n, nil
}
func (s *Service) save(w Workspace, in SaveInput) (Note, error) {
	if !strings.EqualFold(filepath.Ext(in.Path), ".md") {
		return Note{}, errors.New("笔记必须为 .md 文件")
	}
	path, err := safePath(w.Path, in.Path)
	if err != nil {
		return Note{}, err
	}
	if err = visibleNotePath(in.Path); err != nil {
		return Note{}, err
	}
	old, err := os.ReadFile(path)
	if err == nil {
		if in.Hash == "" || digest(old) != in.Hash {
			return Note{}, errors.New("CONFLICT: 文件已被外部修改。请保留草稿，重新载入后合并")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Note{}, err
	} else if in.Hash != "" {
		return Note{}, errors.New("CONFLICT: 原文件已被外部删除")
	}
	if len(in.Content) > 32<<20 {
		return Note{}, errors.New("笔记超过 32 MB")
	}
	if old != nil {
		previous := parseNote(in.Path, old, time.Now())
		title := explicitTitle(in.Content)
		if title != "" && title != explicitTitle(string(old)) {
			filename := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-").Replace(title)
			target := filepath.ToSlash(filepath.Join(filepath.Dir(in.Path), filename+".md"))
			if !strings.EqualFold(target, filepath.ToSlash(in.Path)) {
				moved, moveErr := s.moveNoteLocked(previous.ID, in.Hash, target, &in.Content)
				return moved.Note, moveErr
			}
		}
	}
	if err = atomicWrite(path, []byte(in.Content)); err != nil {
		return Note{}, err
	}
	return parseNote(in.Path, []byte(in.Content), time.Now()), nil
}
func (s *Service) CreateNote(title, dir string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return Note{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "未命名笔记"
	}
	filename := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-").Replace(title)
	if dir == "" {
		dir = "Inbox"
	}
	path := filepath.Join(dir, filename+".md")
	full, err := safePath(w.Path, path)
	if err != nil {
		return Note{}, err
	}
	if _, err = os.Stat(full); err == nil {
		path = filepath.Join(dir, filename+"-"+uid()[:8]+".md")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Note{}, err
	}
	meta := fmt.Sprintf("---\nid: %s\ntitle: %q\ntags: []\ncreated: %s\n---\n\n# %s\n\n", uid(), title, time.Now().Format(time.RFC3339), title)
	return s.save(w, SaveInput{Path: filepath.ToSlash(path), Content: meta})
}
func (s *Service) DeleteNote(id, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	notes, err := s.scan(w)
	if err != nil {
		return err
	}
	for _, n := range notes {
		if n.ID == id {
			if n.Hash != hash {
				return errors.New("CONFLICT: 笔记已变化，请刷新后重试")
			}
			path, err := safePath(w.Path, n.Path)
			if err != nil {
				return err
			}
			trash := filepath.Join(s.data(w), "trash", uid(), n.Path)
			if err = os.MkdirAll(filepath.Dir(trash), 0700); err != nil {
				return err
			}
			if err = atomicWrite(trash, []byte(n.Content)); err != nil {
				return err
			}
			return os.Remove(path)
		}
	}
	return errors.New("笔记不存在")
}
func (s *Service) SetTask(id string, done, confirm bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return err
	}
	notes, err := s.scan(w)
	if err != nil {
		return err
	}
	for _, n := range notes {
		for _, t := range tasksFor(n) {
			if t.ID != id {
				continue
			}
			if t.Candidate || t.Inferred {
				meta := map[string]Task{}
				p := filepath.Join(s.data(w), "tasks-metadata.json")
				if err = readJSON(p, &meta); err != nil {
					return err
				}
				if _, ok := meta[id]; !ok && !confirm && !t.Inferred {
					return errors.New("请先确认候选任务")
				}
				if stored, ok := meta[id]; ok {
					t = mergeTask(t, stored)
				}
				if t.Deleted {
					return errors.New("待办已移除")
				}
				date := t.DueAt
				if t.Recurrence != nil {
					t, err = completeOccurrence(t, t.DueAt, done)
					if err != nil {
						return err
					}
				} else {
					t.Done = done
				}
				t.Candidate = false
				if t.Inferred {
					t.IntentDecision = "accepted"
				}
				t = taskHistory(t, date, done, n, false)
				meta[id] = t
				return s.commitTaskMetadata(w, meta)
			}
			lines := strings.Split(strings.ReplaceAll(n.Content, "\r\n", "\n"), "\n")
			metadata := map[string]Task{}
			metadataPath := filepath.Join(s.data(w), "tasks-metadata.json")
			if err = readJSON(metadataPath, &metadata); err != nil {
				return err
			}
			if stored, ok := metadata[id]; ok {
				t = mergeTask(t, stored)
			}
			state := "[ ]"
			if done {
				state = "[x]"
			}
			line := lines[t.Line]
			m := checkbox.FindStringSubmatchIndex(line)
			lines[t.Line] = line[:m[2]-1] + state + line[m[3]+1:]
			if !taskMarker.MatchString(lines[t.Line]) {
				lines[t.Line] += " <!-- task:" + t.ID + " -->"
			}
			text := strings.Join(lines, "\n")
			if strings.Contains(n.Content, "\r\n") {
				text = strings.ReplaceAll(text, "\n", "\r\n")
			}
			_, err = s.save(w, SaveInput{Path: n.Path, Hash: n.Hash, Content: text})
			if err != nil {
				return err
			}
			n.Content = text
			t.Done = done
			t = taskHistory(t, t.DueAt, done, n, false)
			metadata[id] = t
			return s.commitTaskMetadata(w, metadata)
		}
	}
	// A repeated completion request after the original has moved is idempotent.
	if done && !confirm {
		metadata := map[string]Task{}
		if e := readJSON(filepath.Join(s.data(w), "tasks-metadata.json"), &metadata); e != nil {
			return e
		}
		if stored, ok := metadata[id]; ok && stored.Done {
			for _, note := range notes {
				if note.ID == stored.NoteID && isTaskArchiveNote(note) {
					return nil
				}
			}
		}
	}
	return errors.New("任务已变化或已归档，请刷新")
}
func (s *Service) Search(query string) ([]SearchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return nil, err
	}
	notes, err := s.scan(w)
	if err != nil {
		return nil, err
	}
	return s.keywordSearchLocked(w, notes, query)
}
func (s *Service) keywordSearchLocked(w Workspace, notes []Note, query string) ([]SearchResult, error) {
	var err error
	query = strings.TrimSpace(query)
	results := []SearchResult{}
	if query == "" {
		return results, nil
	}
	if err = s.indexNotes(w.ID, notes); err != nil {
		return nil, err
	}
	ranked := map[string]float64{}
	if len([]rune(query)) >= 3 {
		rows, e := s.db.Query("SELECT id,bm25(notes_fts) FROM notes_fts WHERE notes_fts MATCH ? AND workspace=?", "\""+strings.ReplaceAll(query, "\"", "\"\"")+"\"", w.ID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id string
			var score float64
			if e = rows.Scan(&id, &score); e != nil {
				rows.Close()
				return nil, e
			}
			ranked[id] = -score
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	terms := strings.Fields(strings.ToLower(query))
	for _, n := range notes {
		score := ranked[n.ID]
		hay := strings.ToLower(n.Content + " " + n.Path + " " + strings.Join(n.Tags, " "))
		all := true
		for _, t := range terms {
			if !strings.Contains(hay, t) {
				all = false
				break
			}
			score += 1
			if strings.Contains(strings.ToLower(n.Title), t) {
				score += 4
			}
		}
		if !all && ranked[n.ID] == 0 {
			continue
		}
		lines := strings.Split(n.Content, "\n")
		line := 0
		snippet := ""
		for i, l := range lines {
			if strings.Contains(strings.ToLower(l), terms[0]) {
				line = i
				snippet = l
				break
			}
		}
		if snippet == "" {
			snippet = n.Title
		}
		results = append(results, SearchResult{Note: n, Snippet: snippet, Line: line, Score: score})
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results, nil
}
func (s *Service) Graph() ([]Edge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.active()
	if err != nil {
		return nil, err
	}
	notes, err := s.scan(w)
	if err != nil {
		return nil, err
	}
	edges := []Edge{}
	annotations, err := loadGraphAnnotations(s.data(w))
	if err != nil {
		return nil, err
	}
	for _, a := range notes {
		seen := map[string]bool{}
		for _, ref := range markdownLinks(a.Content) {
			if b := resolveNoteLink(a, ref, notes); b != nil && b.ID != a.ID && !seen[b.ID] {
				seen[b.ID] = true
				edges = append(edges, Edge{Source: a.ID, Target: b.ID, Type: "MARKDOWN_LINK", Annotation: annotations.Relations[relationKey(a.ID, b.ID)]})
			}
		}
	}
	confirmed, err := s.graphRelations(w)
	if err != nil {
		return nil, err
	}
	available := map[string]bool{}
	for _, n := range notes {
		available[n.ID] = true
	}
	seen := map[string]bool{}
	for _, r := range confirmed {
		key := relationKey(r.Source, r.Target)
		if available[r.Source] && available[r.Target] && !seen[key] {
			edges = append(edges, Edge{Source: r.Source, Target: r.Target, Type: "USER_CONFIRMED", Annotation: annotations.Relations[key]})
			seen[key] = true
		}
	}
	return edges, nil
}

func (s *Service) ActiveWorkspacePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.active()
	if e != nil {
		return ""
	}
	return w.Path
}

package core

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type SyncConfig struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	Endpoint        string `json:"endpoint"`
	Username        string `json:"username"`
	RemoteID        string `json:"remoteId"`
	IntervalMinutes int    `json:"intervalMinutes"`
	Revision        string `json:"revision"`
	HasSecret       bool   `json:"hasSecret"`
}
type SyncStatus struct {
	WorkspaceID string `json:"workspaceId"`
	State       string `json:"state"`
	Message     string `json:"message"`
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
	Conflicts   int    `json:"conflicts"`
	LastSuccess string `json:"lastSuccess"`
	Updated     string `json:"updated"`
}
type syncFile struct {
	Hash     string `json:"hash,omitempty"`
	Deleted  bool   `json:"deleted,omitempty"`
	Modified string `json:"modified"`
}
type syncManifest struct {
	Version     int                 `json:"version"`
	WorkspaceID string              `json:"workspaceId"`
	Files       map[string]syncFile `json:"files"`
}
type SyncConflict struct {
	RemotePath    string `json:"remotePath,omitempty"`
	ID            string `json:"id"`
	Path          string `json:"path"`
	LocalHash     string `json:"localHash"`
	RemoteHash    string `json:"remoteHash"`
	Created       string `json:"created"`
	LocalDeleted  bool   `json:"localDeleted"`
	RemoteDeleted bool   `json:"remoteDeleted"`
}
type SyncConflictPreview struct {
	Conflict SyncConflict `json:"conflict"`
	Text     bool         `json:"text"`
	Local    string       `json:"local"`
	Remote   string       `json:"remote"`
}
type syncLocalState struct {
	Target      string            `json:"target"`
	Baseline    map[string]string `json:"baseline"`
	Conflicts   []SyncConflict    `json:"conflicts"`
	LastSuccess string            `json:"lastSuccess"`
}

var syncMetadata = []string{"attachments-manifest.json", "tasks-metadata.json", "relations.json", "graph-layouts.json", "graph-annotations.json", "bookmarks.json", "task-reminders.json", "archive-rules.json", "semantic-archive.json"}
var syncRemoteID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

func validDigest(h string) bool {
	b, e := hex.DecodeString(h)
	return e == nil && len(b) == 32 && h == strings.ToLower(h)
}
func (s *Service) syncPath(w Workspace, name string) string {
	return filepath.Join(s.data(w), "sync", name)
}
func syncTarget(c SyncConfig) string {
	return digest([]byte(c.Endpoint + "\n" + c.RemoteID + "\n" + c.Username + "\n" + c.Mode))
}
func (s *Service) loadSync(w Workspace) (SyncConfig, error) {
	c := SyncConfig{Mode: "bidirectional", RemoteID: w.ID, IntervalMinutes: 5}
	e := readJSON(s.syncPath(w, "config.json"), &c)
	return c, e
}
func (s *Service) workspaceByID(id string) (Workspace, error) {
	for _, w := range s.settings.Workspaces {
		if w.ID == id {
			return w, nil
		}
	}
	return Workspace{}, errors.New("工作区不存在")
}
func (s *Service) GetSyncConfig(id string) (SyncConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspaceByID(id)
	if e != nil {
		return SyncConfig{}, e
	}
	c, e := s.loadSync(w)
	if e != nil {
		return c, e
	}
	secret, e := s.secrets.Get(s.secretKey("sync:" + id + ":" + c.Revision))
	c.HasSecret = secret != ""
	return c, e
}
func (s *Service) SaveSyncConfig(id string, c SyncConfig, secret string, clear bool) (SyncConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspaceByID(id)
	if e != nil {
		return c, e
	}
	old, e := s.loadSync(w)
	if e != nil {
		return c, e
	}
	s.externalMu.Lock()
	running := s.syncCancels[id] != nil
	s.externalMu.Unlock()
	if running {
		return c, errors.New("请等待同步结束或先取消同步")
	}
	if old.Revision != c.Revision {
		return c, errors.New("同步配置已变化，请重新打开设置")
	}
	u, e := externalURL(c.Endpoint, false)
	if e != nil {
		return c, e
	}
	c.Endpoint = u.String()
	c.Username = strings.TrimSpace(c.Username)
	if !syncRemoteID.MatchString(c.RemoteID) || c.IntervalMinutes < 1 || c.IntervalMinutes > 1440 || (c.Mode != "bidirectional" && c.Mode != "backup") || len(secret) > 16000 || len(c.Username) > 300 {
		return c, errors.New("请检查工作区同步标识、模式和同步间隔（1–1440 分钟）")
	}
	value := ""
	if !clear {
		value, e = s.secrets.Get(s.secretKey("sync:" + id + ":" + old.Revision))
		if e != nil {
			return c, e
		}
	}
	if c.Endpoint != old.Endpoint || c.Username != old.Username {
		value = ""
	}
	if secret != "" {
		value = secret
	}
	c.Revision = uid()
	c.HasSecret = false
	key := s.secretKey("sync:" + id + ":" + c.Revision)
	if value != "" {
		if e = s.secrets.Set(key, value); e != nil {
			return c, errors.New("无法写入操作系统安全凭据存储")
		}
	}
	if e = writeJSON(s.syncPath(w, "config.json"), c); e != nil {
		_ = s.secrets.Delete(key)
		return c, e
	}
	_ = s.secrets.Delete(s.secretKey("sync:" + id + ":" + old.Revision))
	s.externalMu.Lock()
	delete(s.syncStates, id)
	s.externalMu.Unlock()
	c.HasSecret = value != ""
	return c, nil
}
func (s *Service) loadSyncState(w Workspace, c SyncConfig) (syncLocalState, error) {
	st := syncLocalState{}
	e := readJSON(s.syncPath(w, "state.json"), &st)
	if e != nil {
		return st, e
	}
	if st.Target != syncTarget(c) {
		st = syncLocalState{Target: syncTarget(c)}
	}
	if st.Baseline == nil {
		st.Baseline = map[string]string{}
	}
	if st.Conflicts == nil {
		st.Conflicts = []SyncConflict{}
	}
	return st, nil
}
func syncFileHash(f syncFile) string {
	if f.Deleted {
		return ""
	}
	return f.Hash
}
func (s *Service) syncFilePath(w Workspace, key string) (string, error) {
	if strings.HasPrefix(key, "notes/") {
		rel := strings.TrimPrefix(key, "notes/")
		if e := visibleNotePath(rel); e != nil {
			return "", e
		}
		return safePath(w.Path, rel)
	}
	if strings.HasPrefix(key, "assets/") {
		id := strings.TrimPrefix(key, "assets/")
		if strings.Contains(id, "/") || id == "" {
			return "", errors.New("非法附件路径")
		}
		return safePath(filepath.Join(s.data(w), "attachments"), id)
	}
	for _, name := range syncMetadata {
		if key == "metadata/"+name {
			return safePath(s.data(w), name)
		}
	}
	return "", errors.New("同步清单包含不允许的路径")
}
func (s *Service) syncSnapshot(w Workspace) (map[string][]byte, error) {
	files := map[string][]byte{}
	var total int64
	add := func(key, path string) error {
		if _, exists := files[key]; exists {
			return nil
		}
		info, e := os.Lstat(path)
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("同步文件不是普通文件")
		}
		if info.Size() > maxAttachment {
			return fmt.Errorf("同步文件超过100MB：%s", key)
		}
		total += info.Size()
		if total > 2<<30 {
			return errors.New("工作区同步内容超过2GB")
		}
		b, e := os.ReadFile(path)
		if e == nil {
			files[key] = b
		}
		return e
	}
	e := filepath.WalkDir(w.Path, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if path == w.Path {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(w.Path, path)
		if e != nil {
			return e
		}
		key := "notes/" + filepath.ToSlash(rel)
		if _, e = s.syncFilePath(w, key); e != nil {
			return e
		}
		return add(key, path)
	})
	if e != nil {
		return nil, e
	}
	for _, name := range syncMetadata {
		if e = add("metadata/"+name, filepath.Join(s.data(w), name)); e != nil {
			return nil, e
		}
	}
	items := []Attachment{}
	if e = readJSON(filepath.Join(s.data(w), "attachments-manifest.json"), &items); e != nil {
		return nil, e
	}
	deletedIDs := map[string]bool{}
	for _, a := range items {
		if a.Deleted {
			deletedIDs[a.ID] = true
			continue
		}
		if a.RemoteOnly {
			if !validAttachmentRemote(a) {
				return nil, errors.New("附件图床链接无效")
			}
			continue
		}
		path, e := s.syncFilePath(w, "assets/"+a.ID)
		if e != nil {
			return nil, e
		}
		if e = add("assets/"+a.ID, path); e != nil {
			return nil, e
		}
		if b, ok := files["assets/"+a.ID]; !ok || digest(b) != a.SHA256 {
			return nil, errors.New("附件缺失或校验失败")
		}
	}
	assetFiles, assetErr := os.ReadDir(filepath.Join(s.data(w), "attachments"))
	if assetErr != nil && !errors.Is(assetErr, os.ErrNotExist) {
		return nil, assetErr
	}
	for _, entry := range assetFiles {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || deletedIDs[entry.Name()] {
			continue
		}
		path, e := s.syncFilePath(w, "assets/"+entry.Name())
		if e != nil {
			return nil, e
		}
		if e = add("assets/"+entry.Name(), path); e != nil {
			return nil, e
		}
	}
	if len(files) > 20000 {
		return nil, errors.New("同步文件超过20000个")
	}
	return files, nil
}
func (s *Service) SyncStatus(id string) SyncStatus {
	s.externalMu.Lock()
	st, ok := s.syncStates[id]
	s.externalMu.Unlock()
	if ok {
		return st
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspaceByID(id)
	if e != nil {
		return SyncStatus{WorkspaceID: id, State: "idle"}
	}
	c, _ := s.loadSync(w)
	local, _ := s.loadSyncState(w, c)
	return SyncStatus{WorkspaceID: id, State: "idle", Conflicts: len(local.Conflicts), LastSuccess: local.LastSuccess}
}
func (s *Service) updateSyncStatus(id, state, message string, done, total, conflicts int) {
	s.externalMu.Lock()
	defer s.externalMu.Unlock()
	st := s.syncStates[id]
	st.WorkspaceID = id
	st.State = state
	st.Message = message
	st.Completed = done
	st.Total = total
	st.Conflicts = conflicts
	st.Updated = time.Now().Format(time.RFC3339)
	if state == "synced" || state == "conflict" {
		st.LastSuccess = st.Updated
	}
	s.syncStates[id] = st
}
func (s *Service) StartSync(id string) (SyncStatus, error) {
	s.mu.Lock()
	w, e := s.workspaceByID(id)
	c, ce := s.loadSync(w)
	s.mu.Unlock()
	if e != nil {
		return SyncStatus{}, e
	}
	if ce != nil {
		return SyncStatus{}, ce
	}
	if !c.Enabled {
		return SyncStatus{}, errors.New("请先启用并保存 WebDAV 配置")
	}
	initial := s.SyncStatus(id)
	s.externalMu.Lock()
	if s.externalClosed || s.syncCancels[id] != nil {
		s.externalMu.Unlock()
		return SyncStatus{}, errors.New("同步正在执行或应用正在退出")
	}
	ctx, cancel := context.WithTimeout(s.externalContext, 20*time.Minute)
	s.syncCancels[id] = cancel
	s.syncStates[id] = initial
	s.externalWG.Add(1)
	s.externalMu.Unlock()
	s.updateSyncStatus(id, "syncing", "正在比较本地与远端", 0, 0, 0)
	go func() {
		defer func() {
			cancel()
			s.externalMu.Lock()
			delete(s.syncCancels, id)
			s.externalMu.Unlock()
			s.externalWG.Done()
		}()
		e := s.runSync(ctx, w, c)
		if e != nil {
			s.updateSyncStatus(id, "error", e.Error(), 0, 0, s.SyncStatus(id).Conflicts)
		}
	}()
	return s.SyncStatus(id), nil
}
func (s *Service) CancelSync(id string) {
	s.externalMu.Lock()
	defer s.externalMu.Unlock()
	if c := s.syncCancels[id]; c != nil {
		c()
	}
}

func (s *Service) runSync(ctx context.Context, w Workspace, c SyncConfig) error {
	s.mu.Lock()
	current, e := s.loadSync(w)
	if e != nil || current.Revision != c.Revision {
		s.mu.Unlock()
		return errors.New("同步配置已变化")
	}
	local, e := s.syncSnapshot(w)
	if e != nil {
		s.mu.Unlock()
		return e
	}
	st, e := s.loadSyncState(w, c)
	s.mu.Unlock()
	if e != nil {
		return e
	}
	secret, e := s.secrets.Get(s.secretKey("sync:" + w.ID + ":" + c.Revision))
	if e != nil {
		return errors.New("无法读取 WebDAV 安全凭据")
	}
	dav, e := newDAV(c, secret)
	if e != nil {
		return e
	}
	if e = dav.mkdir(ctx); e != nil {
		return e
	}
	m, etag, e := dav.manifest(ctx)
	if e != nil {
		return e
	}
	if e = dav.checkConditionalWrites(ctx); e != nil {
		return e
	}
	if m.Version != 1 || m.WorkspaceID != c.RemoteID || m.Files == nil || len(m.Files) > 100000 {
		return errors.New("远端工作区标识或清单版本无效")
	}
	folded := map[string]bool{}
	for key, f := range m.Files {
		if key != filepath.ToSlash(filepath.Clean(filepath.FromSlash(key))) || strings.Contains(key, "\\") {
			return errors.New("远端路径不是标准相对路径")
		}
		if _, e = s.syncFilePath(w, key); e != nil {
			return e
		}
		fold := strings.ToLower(key)
		if folded[fold] {
			return errors.New("远端包含大小写冲突的文件名")
		}
		folded[fold] = true
		if !f.Deleted && !validDigest(f.Hash) {
			return errors.New("远端文件哈希无效")
		}
	}
	keys := map[string]bool{}
	for k := range local {
		keys[k] = true
	}
	for k := range m.Files {
		keys[k] = true
	}
	for k := range st.Baseline {
		keys[k] = true
	}
	ordered := []string{}
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	blocked := map[string]bool{}
	for _, x := range st.Conflicts {
		blocked[x.Path] = true
	}
	downloads := map[string][]byte{}
	deletions := map[string]bool{}
	nextBase := map[string]string{}
	for k, v := range st.Baseline {
		nextBase[k] = v
	}
	changed := etag == ""
	for index, key := range ordered {
		if e = ctx.Err(); e != nil {
			return errors.New("同步已取消")
		}
		s.updateSyncStatus(w.ID, "syncing", "正在同步 "+key, index, len(ordered), len(st.Conflicts))
		lb, exists := local[key]
		lh := ""
		if exists {
			lh = digest(lb)
		}
		rf, remoteExists := m.Files[key]
		rh := syncFileHash(rf)
		base, known := st.Baseline[key]
		wasBlocked := blocked[key]
		if wasBlocked {
			unchanged := false
			for _, x := range st.Conflicts {
				comparedRemote := rh
				if x.RemotePath != "" {
					comparedRemote = syncFileHash(m.Files[x.RemotePath])
				}
				if x.Path == key && x.LocalHash == lh && x.RemoteHash == comparedRemote {
					unchanged = true
				}
			}
			if unchanged {
				continue
			}
			filtered := []SyncConflict{}
			for _, x := range st.Conflicts {
				if x.Path != key {
					filtered = append(filtered, x)
				}
			}
			st.Conflicts = filtered
		}
		if lh == rh {
			nextBase[key] = lh
			continue
		}
		if c.Mode == "backup" && lh == "" {
			continue
		}
		conflict := wasBlocked || (!known && remoteExists && rf.Deleted && lh != "") || (lh != base && rh != base)
		if c.Mode == "backup" && rh != base && rh != "" {
			conflict = true
		}
		if conflict {
			if key == "metadata/attachments-manifest.json" && lh != "" && rh != "" {
				rb, e := dav.getObject(ctx, rh)
				if e != nil {
					return e
				}
				if merged, ok := mergeAttachmentManifests(lb, rb); ok {
					h := digest(merged)
					if e = dav.putObject(ctx, h, merged); e != nil {
						return e
					}
					m.Files[key] = syncFile{Hash: h, Modified: time.Now().UTC().Format(time.RFC3339)}
					downloads[key] = merged
					nextBase[key] = h
					changed = true
					continue
				}
			}
			x := SyncConflict{ID: uid(), Path: key, LocalHash: lh, RemoteHash: rh, LocalDeleted: lh == "", RemoteDeleted: rh == "", Created: time.Now().Format(time.RFC3339)}
			if lh != "" {
				if e = atomicWrite(s.syncPath(w, "conflicts/"+x.ID+"-local"), lb); e != nil {
					return e
				}
			}
			if rh != "" {
				b, e := dav.getObject(ctx, rh)
				if e != nil {
					return e
				}
				if e = atomicWrite(s.syncPath(w, "conflicts/"+x.ID+"-remote"), b); e != nil {
					return e
				}
			}
			st.Conflicts = append(st.Conflicts, x)
			continue
		}
		if rh == base {
			if lh != "" {
				if e = dav.putObject(ctx, lh, lb); e != nil {
					return e
				}
			}
			m.Files[key] = syncFile{Hash: lh, Deleted: lh == "", Modified: time.Now().UTC().Format(time.RFC3339)}
			changed = true
			nextBase[key] = lh
		} else if c.Mode != "backup" {
			if rh == "" {
				deletions[key] = true
			} else {
				b, e := dav.getObject(ctx, rh)
				if e != nil {
					return e
				}
				downloads[key] = b
			}
			nextBase[key] = rh
		}
	}
	// Validate the complete prospective data set before committing or touching the
	// local working copy. Object uploads are immutable and harmless if aborted.
	if e = s.groupSyncMoves(w, local, downloads, st.Baseline, nextBase, &st); e != nil {
		return e
	}
	prospective := map[string][]byte{}
	for k, v := range local {
		prospective[k] = v
	}
	for k, v := range downloads {
		prospective[k] = v
	}
	for k := range deletions {
		delete(prospective, k)
	}
	if e = s.validateSyncData(w, prospective); e != nil {
		return fmt.Errorf("同步数据校验失败，未覆盖本地文件：%w", e)
	}
	if changed {
		if e = dav.commit(ctx, m, etag); e != nil {
			return e
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, e = s.loadSync(w)
	if ctx.Err() != nil {
		return errors.New("同步已取消")
	}
	if _, registered := s.workspaceByID(w.ID); registered != nil {
		return errors.New("工作区已移除，未覆盖本地文件")
	}
	if e != nil || current.Revision != c.Revision {
		return errors.New("同步配置已变化，未覆盖本地文件")
	}
	// Reject edits made during network requests, including changes by another editor.
	fresh, e := s.syncSnapshot(w)
	if e != nil {
		return e
	}
	if !sameSyncSnapshot(local, fresh) {
		return errors.New("同步期间本地文件发生变化，已保留最新内容，稍后重试")
	}
	oldState := st
	st.Baseline = nextBase
	st.LastSuccess = time.Now().Format(time.RFC3339)
	if e = s.applySyncFiles(w, local, downloads, deletions, oldState, st); e != nil {
		return e
	}
	state, msg := "synced", "同步完成"
	if len(st.Conflicts) > 0 {
		state = "conflict"
		msg = "存在冲突，双方版本已保留，请选择处理方式"
	}
	s.updateSyncStatus(w.ID, state, msg, len(ordered), len(ordered), len(st.Conflicts))
	return nil
}
func sameSyncSnapshot(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		other, ok := b[k]
		if !ok || digest(v) != digest(other) {
			return false
		}
	}
	return true
}
func (s *Service) validateSyncData(w Workspace, files map[string][]byte) error {
	stage, e := os.MkdirTemp(s.root, ".sync-validate-*")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	for key, b := range files {
		var path string
		switch {
		case strings.HasPrefix(key, "notes/"):
			path = filepath.Join(stage, filepath.FromSlash(key))
		case strings.HasPrefix(key, "assets/"):
			path = filepath.Join(stage, "metadata", "attachments", strings.TrimPrefix(key, "assets/"))
		case strings.HasPrefix(key, "metadata/"):
			path = filepath.Join(stage, filepath.FromSlash(key))
		default:
			return errors.New("同步路径无效")
		}
		if e = atomicWrite(path, b); e != nil {
			return e
		}
	}
	noteRoot := filepath.Join(stage, "notes")
	if e = os.MkdirAll(noteRoot, 0700); e != nil {
		return e
	}
	if _, e = s.scan(Workspace{Path: noteRoot}); e != nil {
		return e
	}
	meta := filepath.Join(stage, "metadata")
	if e = validateMetadata(meta); e != nil {
		return e
	}
	if _, ok := files["metadata/semantic-archive.json"]; ok {
		var c SemanticConfig
		if e = readJSON(filepath.Join(meta, "semantic-archive.json"), &c); e != nil {
			return e
		}
		normalizeSemantic(&c)
		if e = validateSemantic(Workspace{Path: noteRoot}, c); e != nil {
			return e
		}
	}
	if _, ok := files["metadata/archive-rules.json"]; ok {
		var c ArchiveRulesConfig
		if e = readJSON(filepath.Join(meta, "archive-rules.json"), &c); e != nil {
			return e
		}
		if e = validateArchiveRules(Workspace{Path: noteRoot}, c.Rules); e != nil {
			return e
		}
	}
	if b, ok := files["metadata/bookmarks.json"]; ok {
		var v []Bookmark
		if e = json.Unmarshal(b, &v); e != nil {
			return e
		}
	}
	return nil
}

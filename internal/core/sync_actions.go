package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) TestSync(id string) (string, error) {
	s.mu.Lock()
	w, e := s.workspaceByID(id)
	if e != nil {
		s.mu.Unlock()
		return "", e
	}
	c, e := s.loadSync(w)
	s.mu.Unlock()
	if e != nil {
		return "", e
	}
	secret, e := s.secrets.Get(s.secretKey("sync:" + id + ":" + c.Revision))
	if e != nil {
		return "", errors.New("无法读取安全凭据")
	}
	dav, e := newDAV(c, secret)
	if e != nil {
		return "", e
	}
	ctx, cancel := context.WithTimeout(s.externalContext, 60*time.Second)
	defer cancel()
	if e = dav.mkdir(ctx); e != nil {
		return "", e
	}
	if e = dav.checkConditionalWrites(ctx); e != nil {
		return "", e
	}
	return "连接成功，读写权限和条件请求校验通过", nil
}
func (d *davClient) checkConditionalWrites(ctx context.Context) error {
	name := ".smartnote-probe-" + uid()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r, e := d.request(cleanup, "DELETE", name, nil, nil)
		if e == nil {
			r.Body.Close()
		}
	}()
	r, e := d.request(ctx, "PUT", name, []byte("probe"), map[string]string{"If-None-Match": "*"})
	if e != nil {
		return e
	}
	r.Body.Close()
	if r.StatusCode != 201 && r.StatusCode != 200 && r.StatusCode != 204 {
		return externalHTTPError("测试写入", r.StatusCode)
	}
	for _, header := range []map[string]string{{"If-Match": "\"smartnote-deliberately-invalid-etag\""}, {"If-None-Match": "*"}} {
		r, e = d.request(ctx, "PUT", name, []byte("probe2"), header)
		if e != nil {
			return e
		}
		r.Body.Close()
		if r.StatusCode != 412 {
			return errors.New("服务未正确支持条件写入，无法安全执行多设备同步")
		}
	}
	return nil
}
func (s *Service) ListSyncConflicts(id string) ([]SyncConflict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspaceByID(id)
	if e != nil {
		return nil, e
	}
	c, e := s.loadSync(w)
	if e != nil {
		return nil, e
	}
	st, e := s.loadSyncState(w, c)
	return st.Conflicts, e
}
func (s *Service) ReadSyncConflict(id, conflictID string) (SyncConflictPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspaceByID(id)
	if e != nil {
		return SyncConflictPreview{}, e
	}
	c, e := s.loadSync(w)
	if e != nil {
		return SyncConflictPreview{}, e
	}
	st, e := s.loadSyncState(w, c)
	if e != nil {
		return SyncConflictPreview{}, e
	}
	for _, x := range st.Conflicts {
		if x.ID == conflictID {
			p := SyncConflictPreview{Conflict: x}
			ext := strings.ToLower(filepath.Ext(x.Path))
			p.Text = ext == ".md" || ext == ".json" || ext == ".txt"
			if !p.Text {
				return p, nil
			}
			if !x.LocalDeleted {
				b, e := os.ReadFile(s.syncPath(w, "conflicts/"+x.ID+"-local"))
				if e != nil {
					return p, e
				}
				if !utf8.Valid(b) || len(b) > 2<<20 {
					p.Text = false
					return p, nil
				}
				p.Local = string(b)
			}
			if !x.RemoteDeleted {
				b, e := os.ReadFile(s.syncPath(w, "conflicts/"+x.ID+"-remote"))
				if e != nil {
					return p, e
				}
				if !utf8.Valid(b) || len(b) > 2<<20 {
					p.Text = false
					return p, nil
				}
				p.Remote = string(b)
			}
			return p, nil
		}
	}
	return SyncConflictPreview{}, errors.New("冲突记录不存在，请刷新")
}

type SyncResolution struct {
	WorkspaceID string `json:"workspaceId"`
	ID          string `json:"id"`
	Choice      string `json:"choice"`
	Merged      string `json:"merged"`
}

func (s *Service) ResolveSyncConflict(in SyncResolution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.externalMu.Lock()
	running := s.syncCancels[in.WorkspaceID] != nil
	s.externalMu.Unlock()
	if running {
		return errors.New("请等待同步完成再处理冲突")
	}
	w, e := s.workspaceByID(in.WorkspaceID)
	if e != nil {
		return e
	}
	c, e := s.loadSync(w)
	if e != nil {
		return e
	}
	st, e := s.loadSyncState(w, c)
	if e != nil {
		return e
	}
	index := -1
	var x SyncConflict
	for i, item := range st.Conflicts {
		if item.ID == in.ID {
			index = i
			x = item
			break
		}
	}
	if index < 0 {
		return errors.New("冲突记录已变化")
	}
	if in.Choice != "local" && in.Choice != "remote" && in.Choice != "merge" {
		return errors.New("请选择保留本地、远端或手动合并")
	}
	files, e := s.syncSnapshot(w)
	if e != nil {
		return e
	}
	before := map[string][]byte{}
	for key, value := range files {
		before[key] = value
	}
	oldState := st
	oldState.Conflicts = append([]SyncConflict{}, st.Conflicts...)
	oldState.Baseline = map[string]string{}
	for key, value := range st.Baseline {
		oldState.Baseline[key] = value
	}
	lh := ""
	if b, ok := files[x.Path]; ok {
		lh = digest(b)
	}
	if lh != x.LocalHash {
		return errors.New("本地文件已变化，请先重新同步以更新冲突预览")
	}
	var selected []byte
	deleted := x.LocalDeleted
	if in.Choice == "local" {
		selected = files[x.Path]
	} else if in.Choice == "remote" {
		deleted = x.RemoteDeleted
		if !deleted {
			selected, e = os.ReadFile(s.syncPath(w, "conflicts/"+x.ID+"-remote"))
			if e != nil {
				return e
			}
			if digest(selected) != x.RemoteHash {
				return errors.New("冲突副本校验失败")
			}
		}
	} else {
		if !strings.HasPrefix(x.Path, "notes/") || !strings.HasSuffix(strings.ToLower(x.Path), ".md") || len(in.Merged) > 32<<20 {
			return errors.New("手动合并仅支持 Markdown 笔记，最大32MB")
		}
		selected = []byte(in.Merged)
		deleted = false
	}
	target := x.Path
	if x.RemotePath != "" && in.Choice != "local" {
		target = x.RemotePath
		if _, exists := files[target]; exists {
			return errors.New("远端移动目标已存在本地文件，请先重新同步")
		}
		delete(files, x.Path)
	}
	if deleted {
		delete(files, target)
	} else {
		files[target] = selected
	}
	if e = s.validateSyncData(w, files); e != nil {
		return e
	}
	writes := map[string][]byte{}
	deletes := map[string]bool{}
	for key, b := range files {
		old, ok := before[key]
		if !ok || digest(old) != digest(b) {
			writes[key] = b
		}
	}
	for key := range before {
		if _, ok := files[key]; !ok {
			deletes[key] = true
		}
	}
	if x.RemotePath != "" {
		st.Baseline[x.Path] = ""
		st.Baseline[x.RemotePath] = x.RemoteHash
	} else {
		st.Baseline[x.Path] = x.RemoteHash
	}
	st.Conflicts = append(st.Conflicts[:index], st.Conflicts[index+1:]...)
	if e = s.applySyncFiles(w, before, writes, deletes, oldState, st); e != nil {
		return e
	}
	s.externalMu.Lock()
	delete(s.syncStates, w.ID)
	s.externalMu.Unlock()
	return nil
}

// Start/interval/retry scheduling runs independently of the UI. Content changes
// are debounced by the application watcher and RequestAutoSync.
func (s *Service) RunSyncScheduler(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	attempts := map[string]time.Time{}
	versions := map[string]string{}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.Lock()
			configs := map[string]SyncConfig{}
			for _, w := range s.settings.Workspaces {
				c, e := s.loadSync(w)
				if e == nil && c.Enabled {
					configs[w.ID] = c
				}
			}
			s.mu.Unlock()
			for id, c := range configs {
				if versions[id] != c.Revision {
					delete(attempts, id)
					versions[id] = c.Revision
				}
				st := s.SyncStatus(id)
				gap := time.Duration(c.IntervalMinutes) * time.Minute
				if st.State == "error" {
					gap = 30 * time.Second
				}
				s.externalMu.Lock()
				pending, dirty := s.syncPending[id]
				s.externalMu.Unlock()
				due := dirty && now.Sub(pending) >= 5*time.Second && now.Sub(attempts[id]) >= 15*time.Second && st.State != "error"
				if st.State == "syncing" || !due && now.Sub(attempts[id]) < gap {
					continue
				}
				attempts[id] = now
				s.externalMu.Lock()
				delete(s.syncPending, id)
				s.externalMu.Unlock()
				_, _ = s.StartSync(id)
			}
		}
	}
}

func (s *Service) RequestAutoSync() {
	s.mu.Lock()
	id := s.settings.ActiveID
	s.mu.Unlock()
	if id == "" {
		return
	}
	s.externalMu.Lock()
	s.syncPending[id] = time.Now()
	s.externalMu.Unlock()
}

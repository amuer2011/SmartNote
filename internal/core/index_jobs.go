package core

import (
	"context"
	"errors"
	"smartnote/internal/embedding"
	"sort"
	"strings"
	"time"
)

type IndexStatus struct {
	WorkspaceID string `json:"workspaceId"`
	State       string `json:"state"`
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
	Notes       int    `json:"notes"`
	Message     string `json:"message"`
	Updated     string `json:"updated"`
}

func (s *Service) IndexStatus() IndexStatus {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if s.indexState.State == "" {
		return IndexStatus{State: "idle", Message: "打开工作区后自动建立检索索引"}
	}
	return s.indexState
}
func (s *Service) CancelIndex() IndexStatus {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if s.indexCancel != nil && (s.indexState.State == "running" || s.indexState.State == "queued") {
		s.indexCancel()
		s.indexState.State = "cancelling"
		s.indexState.Message = "正在停止，将保留已建立的索引"
	}
	return s.indexState
}

// StartIndex updates or explicitly rebuilds derived vectors, never note files.
func (s *Service) StartIndex(rebuild bool) (IndexStatus, error) {
	snapshot, err := s.Snapshot()
	if err != nil {
		return s.IndexStatus(), err
	}
	if snapshot.Settings.ActiveID == "" {
		return s.IndexStatus(), errors.New("请先打开工作区")
	}
	s.queueIndex(snapshot, true, rebuild)
	return s.IndexStatus(), nil
}

// QueueIndex is fed disk snapshots by the desktop host. Identical snapshots do
// not restart completed, failed or user-cancelled tasks on the polling timer.
func (s *Service) QueueIndex(snapshot Snapshot) { s.queueIndex(snapshot, false, false) }
func (s *Service) queueIndex(snapshot Snapshot, manual, rebuild bool) {
	workspace := snapshot.Settings.ActiveID
	keys := make([]string, len(snapshot.Notes))
	for i, n := range snapshot.Notes {
		keys[i] = n.ID + ":" + n.Path + ":" + n.Hash
	}
	sort.Strings(keys)
	fingerprint := digest([]byte(strings.Join(keys, "\n")))
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if s.indexClosed {
		return
	}
	if workspace == "" {
		if s.indexCancel != nil {
			s.indexCancel()
		}
		return
	}
	if s.indexAttempt == nil {
		s.indexAttempt = map[string]string{}
	}
	if !manual && s.indexState.WorkspaceID == workspace && s.indexAttempt[workspace] == fingerprint {
		return
	}
	if s.indexCancel != nil {
		s.indexCancel()
	}
	previous := s.indexDone
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.indexGeneration++
	generation := s.indexGeneration
	s.indexCancel = cancel
	s.indexDone = done
	s.indexAttempt[workspace] = fingerprint
	s.indexState = IndexStatus{WorkspaceID: workspace, State: "queued", Notes: len(snapshot.Notes), Message: "等待后台建立索引", Updated: time.Now().Format(time.RFC3339)}
	s.indexWG.Add(1)
	go func() {
		defer s.indexWG.Done()
		defer close(done)
		defer cancel()
		update := func(state string, completed, total int, message string) {
			s.indexMu.Lock()
			defer s.indexMu.Unlock()
			if generation != s.indexGeneration {
				return
			}
			s.indexState.State = state
			s.indexState.Completed = completed
			s.indexState.Total = total
			s.indexState.Message = message
			s.indexState.Updated = time.Now().Format(time.RFC3339)
		}
		if previous != nil {
			select {
			case <-previous:
			case <-ctx.Done():
				update("cancelled", 0, 0, "已停止，已完成的索引可继续使用")
				return
			}
		}
		if ctx.Err() != nil {
			update("cancelled", 0, 0, "已停止，已完成的索引可继续使用")
			return
		}
		passages := searchPassages(snapshot.Notes)
		update("running", 0, len(passages), "正在后台建立语义索引，可继续编辑笔记")
		completed := 0
		_, err := s.passageVectorsWithOptions(workspace, snapshot.Notes, passages, embedding.Dimensions, vectorOptions{ctx: ctx, rebuild: rebuild, background: true, progress: func(count, total int) {
			completed = count
			if ctx.Err() == nil {
				update("running", count, total, "正在后台建立语义索引，可继续编辑笔记")
			}
		}})
		if ctx.Err() != nil {
			update("cancelled", completed, len(passages), "已停止，已完成的索引可继续使用；点击继续更新可恢复")
		} else if err != nil {
			update("failed", completed, len(passages), "索引未完成："+err.Error())
		} else {
			update("ready", len(passages), len(passages), "索引已就绪，后续修改自动增量更新")
		}
	}()
}

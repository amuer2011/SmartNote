package core

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func jobVector() []float64 { v := make([]float64, 512); v[0] = 1; return v }
func waitIndex(t *testing.T, s *Service, state string) IndexStatus {
	t.Helper()
	limit := time.After(5 * time.Second)
	for {
		status := s.IndexStatus()
		if status.State == state {
			return status
		}
		if status.State == "failed" && state != "failed" {
			t.Fatalf("index failed: %+v", status)
		}
		select {
		case <-limit:
			t.Fatalf("waiting for %s: %+v", state, status)
		case <-time.After(time.Millisecond * 5):
		}
	}
}
func TestBackgroundIndexCancelRetainsBatchesAndResumes(t *testing.T) {
	s, _ := fixture(t)
	var first Note
	for i := 0; i < 9; i++ {
		n := mustNote(t, s, string(rune('A'+i)))
		if i == 0 {
			first = n
		}
	}
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	s.embedTexts = func(texts []string) ([][]float64, error) {
		if calls.Add(1) == 2 {
			close(started)
			<-release
		}
		out := [][]float64{}
		for range texts {
			out = append(out, jobVector())
		}
		return out, nil
	}
	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.QueueIndex(snapshot)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("worker did not start")
	}
	if st := s.IndexStatus(); st.Completed != 4 || st.Total != 9 {
		t.Fatalf("bad progress: %+v", st)
	}
	saved := make(chan error, 1)
	go func() {
		_, e := s.SaveNote(SaveInput{ID: first.ID, Hash: first.Hash, Path: first.Path, Content: first.Content + "\n仍然可以编辑"})
		saved <- e
	}()
	select {
	case err = <-saved:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("background inference blocked editing")
	}
	if st := s.CancelIndex(); st.State != "cancelling" {
		t.Fatal(st)
	}
	close(release)
	waitIndex(t, s, "cancelled")
	var cached int
	if err = s.db.QueryRow("SELECT count(*) FROM passage_vectors").Scan(&cached); err != nil || cached != 4 {
		t.Fatalf("completed cache lost: %d %v", cached, err)
	}
	s.QueueIndex(snapshot)
	if st := s.IndexStatus(); st.State != "cancelled" {
		t.Fatalf("polling restarted cancellation: %+v", st)
	}
	if _, err = s.StartIndex(false); err != nil {
		t.Fatal(err)
	}
	st := waitIndex(t, s, "ready")
	if st.Completed != 9 {
		t.Fatal(st)
	}
	before := calls.Load()
	if _, err = s.StartIndex(false); err != nil {
		t.Fatal(err)
	}
	waitIndex(t, s, "ready")
	if calls.Load() != before {
		t.Fatal("unchanged index recomputed")
	}
	if _, err = s.StartIndex(true); err != nil {
		t.Fatal(err)
	}
	waitIndex(t, s, "ready")
	if calls.Load() <= before {
		t.Fatal("explicit rebuild reused cache")
	}
}
func TestBackgroundIndexFailureDoesNotRetryEverySnapshot(t *testing.T) {
	s, _ := fixture(t)
	mustNote(t, s, "失败重试")
	var calls atomic.Int32
	s.embedTexts = func([]string) ([][]float64, error) { calls.Add(1); return nil, errors.New("测试模型故障") }
	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.QueueIndex(snapshot)
	waitIndex(t, s, "failed")
	s.QueueIndex(snapshot)
	if calls.Load() != 1 || s.IndexStatus().State != "failed" {
		t.Fatal("failed job restarted automatically")
	}
	s.embedTexts = func(texts []string) ([][]float64, error) {
		out := [][]float64{}
		for range texts {
			out = append(out, jobVector())
		}
		return out, nil
	}
	if _, err = s.StartIndex(false); err != nil {
		t.Fatal(err)
	}
	waitIndex(t, s, "ready")
}
func TestBackgroundIndexLatestWorkspaceWins(t *testing.T) {
	s, _ := fixture(t)
	mustNote(t, s, "旧工作区")
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s.embedTexts = func(texts []string) ([][]float64, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		out := [][]float64{}
		for range texts {
			out = append(out, jobVector())
		}
		return out, nil
	}
	old, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.QueueIndex(old)
	<-started
	latest := old
	latest.Settings.ActiveID = "another-workspace"
	latest.Notes = []Note{{ID: "new-note", Title: "新笔记", Path: "new.md", Hash: "new", Content: "新内容"}}
	s.QueueIndex(latest)
	close(release)
	status := waitIndex(t, s, "ready")
	if status.WorkspaceID != "another-workspace" || status.Completed != 1 {
		t.Fatalf("stale worker changed current status: %+v", status)
	}
}

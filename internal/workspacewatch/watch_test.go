package workspacewatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func start(t *testing.T, root string, fallback time.Duration) <-chan struct{} {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready, done, events := make(chan struct{}), make(chan struct{}), make(chan struct{}, 128)
	go func() { defer close(done); run(ctx, root, func() { events <- struct{}{} }, nil, fallback, ready) }()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher not ready")
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("watcher leaked on cancellation")
		}
	})
	return events
}
func await(t *testing.T, events <-chan struct{}) {
	t.Helper()
	select {
	case <-events:
	case <-time.After(5 * time.Second):
		t.Fatal("no filesystem change event")
	}
}
func write(t *testing.T, path string) {
	t.Helper()
	if e := os.WriteFile(path, []byte(time.Now().String()), 0600); e != nil {
		t.Fatal(e)
	}
}

func TestRecursiveChangesAndHiddenPaths(t *testing.T) {
	root := t.TempDir()
	events := start(t, root, time.Hour)
	path := filepath.Join(root, "笔记.md")
	write(t, path)
	await(t, events)
	write(t, path)
	await(t, events)
	renamed := filepath.Join(root, "改名.md")
	if e := os.Rename(path, renamed); e != nil {
		t.Fatal(e)
	}
	await(t, events)
	if e := os.Remove(renamed); e != nil {
		t.Fatal(e)
	}
	await(t, events)
	sub := filepath.Join(root, "新增", "子目录")
	if e := os.MkdirAll(sub, 0700); e != nil {
		t.Fatal(e)
	}
	path = filepath.Join(sub, "嵌套.md")
	write(t, path)
	await(t, events)
	write(t, path)
	await(t, events)
	hidden := filepath.Join(root, ".recovery")
	if e := os.Mkdir(hidden, 0700); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(hidden, "忽略.md"))
	write(t, filepath.Join(root, "image.png"))
	write(t, filepath.Join(root, ".临时.md"))
	select {
	case <-events:
		t.Fatal("hidden or unrelated file triggered refresh")
	case <-time.After(650 * time.Millisecond):
	}
	if e := os.RemoveAll(filepath.Join(root, "新增")); e != nil {
		t.Fatal(e)
	}
	await(t, events)
}

func TestMovedInDirectoryIsWatched(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "notes")
	staged := filepath.Join(parent, "incoming")
	if e := os.MkdirAll(filepath.Join(staged, "深层"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(staged, "深层", "原有.md"))
	events := start(t, root, time.Hour)
	target := filepath.Join(root, "移入")
	if e := os.Rename(staged, target); e != nil {
		t.Fatal(e)
	}
	await(t, events)
	write(t, filepath.Join(target, "深层", "原有.md"))
	await(t, events)
}

func TestFallbackRepairsMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	events := start(t, root, 100*time.Millisecond)
	await(t, events)
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	await(t, events)
	write(t, filepath.Join(root, "恢复.md"))
	await(t, events)
	if !visible(root, filepath.Join(root, "子目录", "笔记.md")) || visible(root, filepath.Join(root, "..", "outside.md")) {
		t.Fatal("invalid path filtering")
	}
}

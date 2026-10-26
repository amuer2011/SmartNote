// Package workspacewatch watches Markdown and directory structure changes.
package workspacewatch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func visible(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	if rel == "." {
		return true
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, "~") {
			return false
		}
	}
	return true
}

func Run(ctx context.Context, root string, changed func(), report func(error)) {
	run(ctx, root, changed, report, 30*time.Second, nil)
}

func run(ctx context.Context, root string, changed func(), report func(error), fallback time.Duration, ready chan<- struct{}) {
	w, err := fsnotify.NewWatcher()
	if report == nil {
		report = func(error) {}
	}
	if err != nil {
		report(err)
		if ready != nil {
			close(ready)
		}
		ticker := time.NewTicker(fallback)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				changed()
			}
		}
	}
	defer w.Close()
	watched := map[string]bool{}
	reconcile := func() {
		for path := range watched {
			info, e := os.Lstat(path)
			if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				_ = w.Remove(path)
				delete(watched, path)
			}
		}
		e := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if e != nil {
				return e
			}
			if !d.IsDir() {
				return nil
			}
			if !visible(root, path) {
				return filepath.SkipDir
			}
			if !watched[path] {
				if e = w.Add(path); e != nil {
					report(e)
				} else {
					watched[path] = true
				}
			}
			return nil
		})
		if e != nil && ctx.Err() == nil {
			report(e)
		}
	}
	reconcile()
	if ready != nil {
		close(ready)
	}
	ticker := time.NewTicker(fallback)
	defer ticker.Stop()
	var timer *time.Timer
	var due <-chan time.Time
	var first time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	notify := func() {
		if first.IsZero() {
			first = time.Now()
		}
		delay := 250 * time.Millisecond
		if remain := time.Until(first.Add(time.Second)); remain < delay {
			delay = remain
		}
		if delay < 0 {
			delay = 0
		}
		if timer == nil {
			timer = time.NewTimer(delay)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(delay)
		}
		due = timer.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
			changed()
		case <-due:
			due = nil
			first = time.Time{}
			changed()
		case e, ok := <-w.Errors:
			if !ok {
				return
			}
			report(e)
			reconcile()
			notify()
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if !visible(root, event.Name) {
				continue
			}
			isDir := watched[event.Name]
			if info, e := os.Lstat(event.Name); e == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				isDir = true
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
				if isDir {
					reconcile()
				}
			}
			if isDir || strings.EqualFold(filepath.Ext(event.Name), ".md") {
				if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
					notify()
				}
			}
		}
	}
}

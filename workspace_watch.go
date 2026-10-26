package main

import (
	"context"
	"log"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"smartnote/internal/workspacewatch"
)

func (a *App) watchWorkspaces(ctx context.Context) {
	defer close(a.watchDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var cancel context.CancelFunc
	var done chan struct{}
	path := ""
	stop := func() {
		if cancel != nil {
			cancel()
			<-done
			cancel = nil
		}
	}
	defer stop()
	for {
		next := a.service.ActiveWorkspacePath()
		if next != path {
			stop()
			path = next
			if path != "" {
				watchCtx, watchCancel := context.WithCancel(ctx)
				cancel = watchCancel
				done = make(chan struct{})
				root, finished := path, done
				go func() {
					defer close(finished)
					defer watchCancel()
					workspacewatch.Run(watchCtx, root, func() { a.service.RequestAutoSync(); runtime.EventsEmit(ctx, "workspace:check") }, func(e error) { log.Printf("Workspace watcher: %v", e) })
				}()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

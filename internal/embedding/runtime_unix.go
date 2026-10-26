//go:build cgo && (darwin || linux) && (amd64 || arm64)

package embedding

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Build preparation supplies only the current platform's verified CPU runtime.
//
//go:embed native/*
var nativeResources embed.FS

func init() { platformResources, _ = fs.Sub(nativeResources, "native") }

var runtimeMu sync.Mutex
var runtimeReady bool
var runtimeUsers int

func initializeRuntime(dir string) error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if runtimeReady {
		runtimeUsers++
		return nil
	}
	name := "libonnxruntime.so"
	if runtime.GOOS == "darwin" {
		name = "libonnxruntime.dylib"
	}
	ort.SetSharedLibraryPath(filepath.Join(dir, name))
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("加载内置推理组件：%w", err)
	}
	if err := ort.DisableTelemetry(); err != nil {
		_ = ort.DestroyEnvironment()
		return err
	}
	runtimeReady = true
	runtimeUsers = 1
	return nil
}

func releaseRuntime() error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if !runtimeReady {
		return nil
	}
	runtimeUsers--
	if runtimeUsers > 0 {
		return nil
	}
	if err := ort.DestroyEnvironment(); err != nil {
		return err
	}
	runtimeReady = false
	return nil
}

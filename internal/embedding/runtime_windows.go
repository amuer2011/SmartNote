//go:build windows && amd64 && cgo

package embedding

import (
	"embed"
	"fmt"
	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/sys/windows"
	"io/fs"
	"path/filepath"
	"sync"
)

//go:embed assets/*.dll
var windowsRuntimeResources embed.FS

func init() { additionalResources, _ = fs.Sub(windowsRuntimeResources, "assets") }

var runtimeMu sync.Mutex
var runtimeReady bool
var runtimeUsers int
var runtimeHandles []windows.Handle

func initializeRuntime(dir string) error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if runtimeReady {
		runtimeUsers++
		return nil
	}
	// Load the bundled C++ runtime explicitly, with dependency lookup restricted to
	// this verified resource directory and Windows system libraries.
	handles := []windows.Handle{}
	for _, name := range []string{"vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll", "msvcp140_1.dll", "onnxruntime.dll"} {
		h, err := windows.LoadLibraryEx(filepath.Join(dir, name), 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
		if err != nil {
			for i := len(handles) - 1; i >= 0; i-- {
				windows.FreeLibrary(handles[i])
			}
			return fmt.Errorf("加载内置推理组件 %s：%w", name, err)
		}
		handles = append(handles, h)
	}
	ort.SetSharedLibraryPath(filepath.Join(dir, "onnxruntime.dll"))
	if err := ort.InitializeEnvironment(); err != nil {
		for i := len(handles) - 1; i >= 0; i-- {
			windows.FreeLibrary(handles[i])
		}
		return err
	}
	// Keep the embedded runtime's optional telemetry disabled in local mode.
	if err := ort.DisableTelemetry(); err != nil {
		_ = ort.DestroyEnvironment()
		for i := len(handles) - 1; i >= 0; i-- {
			windows.FreeLibrary(handles[i])
		}
		return err
	}
	runtimeHandles = handles
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
	for i := len(runtimeHandles) - 1; i >= 0; i-- {
		windows.FreeLibrary(runtimeHandles[i])
	}
	runtimeHandles = nil
	runtimeReady = false
	return nil
}

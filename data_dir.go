package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func dataDirectoryCandidates() []string {
	if explicit := os.Getenv("SMARTNOTE_DATA_DIR"); explicit != "" {
		return []string{explicit}
	}
	candidates := []string{}
	if config, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(config, "SmartNote"))
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			candidates = append(candidates, filepath.Join(local, "SmartNote"))
		}
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "SmartNoteData"))
	}
	return candidates
}

// Never bypass an existing workspace registry: a permissions problem must not
// silently replace the user's registered workspaces with an empty profile.
func selectDataDirectory(candidates []string) (string, error) {
	// A formerly unwritable default may become writable after a permissions fix.
	// Prefer the existing workspace registry over creating a new empty profile.
	if len(candidates) > 1 {
		for _, candidate := range candidates {
			if _, err := os.Stat(filepath.Join(candidate, "settings.json")); err == nil {
				return selectDataDirectory([]string{candidate})
			}
		}
	}
	failures := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		root, err := filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		key := root
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		existingData := false
		for _, name := range []string{"settings.json", "index.db", "workspaces"} {
			if _, e := os.Stat(filepath.Join(root, name)); e == nil {
				existingData = true
			} else if !errors.Is(e, os.ErrNotExist) && !errors.Is(e, os.ErrPermission) && !errors.Is(e, syscall.ENOTDIR) {
				return "", e
			}
		}
		if err = probeDataDirectory(root); err == nil {
			return root, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", root, err))
		if existingData {
			return "", fmt.Errorf("已有数据目录无法写入，为避免遗漏原有工作区，未切换到其他目录：\n%s", strings.Join(failures, "\n"))
		}
	}
	return "", fmt.Errorf("没有可写的应用数据目录。可通过 SMARTNOTE_DATA_DIR 指定目录：\n%s", strings.Join(failures, "\n"))
}

func probeDataDirectory(root string) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".smartnote-write-check-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.WriteString("SmartNote"); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

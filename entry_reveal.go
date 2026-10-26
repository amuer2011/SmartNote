package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (a *App) RevealEntry(path string) error {
	full, err := a.service.EntryLocation(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(full)
	if err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if info.IsDir() {
			command = exec.Command("explorer.exe", full)
		} else {
			command = exec.Command("explorer.exe", "/select,"+full)
		}
	case "darwin":
		command = exec.Command("open", "-R", full)
	default:
		if !info.IsDir() {
			full = filepath.Dir(full)
		}
		command = exec.Command("xdg-open", full)
	}
	if err = command.Start(); err != nil {
		return err
	}
	go command.Wait()
	return nil
}

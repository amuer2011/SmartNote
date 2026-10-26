//go:build windows

package main

import (
	"golang.org/x/sys/windows"
)

func showStartupError(message string) {
	title, _ := windows.UTF16PtrFromString("SmartNote 启动失败")
	text, _ := windows.UTF16PtrFromString(message)
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}

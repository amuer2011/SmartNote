//go:build windows

package main

import "golang.org/x/sys/windows"

// CF_HDROP distinguishes copied filesystem images from clipboard bitmap data.
// Checking availability reads no clipboard contents and needs no OpenClipboard.
func clipboardHasFiles() bool {
	available := windows.NewLazySystemDLL("user32.dll").NewProc("IsClipboardFormatAvailable")
	result, _, _ := available.Call(15)
	return result != 0
}

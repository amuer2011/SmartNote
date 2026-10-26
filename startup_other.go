//go:build !windows

package main

import "fmt"
import "os"

func showStartupError(message string) { fmt.Fprintln(os.Stderr, "SmartNote 启动失败：", message) }

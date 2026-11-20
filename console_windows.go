//go:build windows

package main

import (
	"os"
	"syscall"
)

// attachConsole: the Windows program is built as a window program (no console
// of its own). Started from a command prompt, create writes to that prompt.
func attachConsole() {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	if r, _, _ := proc.Call(attachParentProcess); r == 0 {
		return // no console (e.g. Task Scheduler): redirect the output to keep it
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
}

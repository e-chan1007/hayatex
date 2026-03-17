//go:build windows

package utils

import (
	"syscall"
	"unsafe"
)

func IsLaunchedByGui() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetConsoleProcessList")

	pids := make([]uint32, 2)
	r1, _, _ := proc.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))

	return r1 <= 1
}

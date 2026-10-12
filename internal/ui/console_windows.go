//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const enableVirtualTerminalProcessing = 0x0004

func enableVirtualTerminal() {
	for _, std := range []int{syscall.STD_OUTPUT_HANDLE, syscall.STD_ERROR_HANDLE} {
		h, err := syscall.GetStdHandle(std)
		if err != nil || h == 0 {
			continue
		}
		var mode uint32
		r, _, _ := procGetConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode)))
		if r == 0 {
			continue
		}
		procSetConsoleMode.Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	}
}

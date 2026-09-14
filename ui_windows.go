//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func interactiveConsole() bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(os.Stdin.Fd()), &mode) == nil &&
		syscall.GetConsoleMode(syscall.Handle(os.Stdout.Fd()), &mode) == nil
}
func enableTerminalOutput() (bool, func()) {
	handle := syscall.Handle(os.Stdout.Fd())
	var before uint32
	if syscall.GetConsoleMode(handle, &before) != nil {
		return false, func() {}
	}
	set := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")
	if ok, _, _ := set.Call(uintptr(handle), uintptr(before|0x0001|0x0004)); ok == 0 {
		return false, func() {}
	}
	return true, func() { set.Call(uintptr(handle), uintptr(before)) }
}
func terminalColumns() int {
	type coord struct{ x, y int16 }
	type rect struct{ left, top, right, bottom int16 }
	type info struct {
		size, cursor coord
		attributes   uint16
		window       rect
		maximum      coord
	}
	var value info
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfo")
	if ok, _, _ := proc.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&value))); ok == 0 {
		return 78
	}
	return int(value.window.right - value.window.left + 1)
}

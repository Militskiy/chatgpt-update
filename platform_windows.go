//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

func powershellPath() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}

// Scripts are distributed as visible sidecar files. No extraction or policy override.
func installedScript(name string) (string, error) {
	exe, e := os.Executable()
	if e != nil {
		return "", e
	}
	return verifiedScript(filepath.Dir(exe), name)
}
func runScript(name string, args ...string) error {
	script, e := installedScript(name)
	if e != nil {
		return e
	}
	cmd := exec.Command(powershellPath(), powershellArgs(script, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("Windows PowerShell failed: %w. Existing execution policy is respected; ask IT to approve/sign these scripts if blocked. No security settings were changed", e)
	}
	return nil
}
func startReplacementHelper(planPath string) error {
	script, e := installedScript("replace-updater.ps1")
	if e != nil {
		return e
	}
	cmd := exec.Command(powershellPath(), powershellArgs(script, "-PlanPath", planPath)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010} // visible new console
	if e := cmd.Start(); e != nil {
		return e
	}
	return cmd.Process.Release()
}
func isReparsePoint(path string) bool {
	p, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return true
	}
	attr, e := syscall.GetFileAttributes(p)
	return e != nil || attr&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
func acquireOperationLock() (func(), error) {
	lockName, e := operationLockName()
	if e != nil {
		return nil, e
	}
	name, _ := syscall.UTF16PtrFromString(lockName)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW")
	h, _, callErr := proc.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, fmt.Errorf("cannot create operation lock: %v", callErr)
	}
	if callErr == syscall.ERROR_ALREADY_EXISTS {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, errors.New("another ChatGPT Update operation is already running for this user")
	}
	return func() { syscall.CloseHandle(syscall.Handle(h)) }, nil
}

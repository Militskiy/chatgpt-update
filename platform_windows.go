//go:build windows

package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func powershellPath() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}
func extractScript(name string) (string, string, error) {
	b, e := assets.ReadFile("scripts/" + name)
	if e != nil {
		return "", "", e
	}
	dir, e := os.MkdirTemp("", "ChatGPTUpdater-script-")
	if e != nil {
		return "", "", e
	}
	path := filepath.Join(dir, name)
	if e := os.WriteFile(path, b, 0600); e != nil {
		os.RemoveAll(dir)
		return "", "", e
	}
	return dir, path, nil
}
func runScript(name string, args ...string) error {
	dir, script, e := extractScript(name)
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	// Process-only setting: organizational Group Policy remains authoritative.
	all := append([]string{"-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script}, args...)
	cmd := exec.Command(powershellPath(), all...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("Windows PowerShell failed: %w (see the detailed error above)", e)
	}
	return nil
}
func startReplacementHelper(planPath string) error {
	dir, script, e := extractScript("replace-updater.ps1")
	if e != nil {
		return e
	}
	cmd := exec.Command(powershellPath(), "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, "-PlanPath", planPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010} // CREATE_NEW_CONSOLE
	if e := cmd.Start(); e != nil {
		os.RemoveAll(dir)
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
	home, e := os.UserHomeDir()
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256([]byte(strings.ToLower(home)))
	name, _ := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\Militskiy.ChatGPTUpdater-%x", sum[:12]))
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

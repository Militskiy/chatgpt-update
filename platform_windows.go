//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
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
	cmd.Env = windowsPSEnvironment(os.Environ())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("Windows PowerShell failed: %w. Existing execution policy is respected; ask IT to approve/sign these scripts if blocked. No security settings were changed", e)
	}
	return nil
}

// Use native console defaults instead of os/exec's nil-stream -> NUL mapping.
// STARTF_USESTDHANDLES is intentionally absent: Windows supplies the new
// console's input, output and error handles. No elevation or policy override.
func startConsoleProcess(exe string, args []string, dir string) (*syscall.ProcessInformation, error) {
	app, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return nil, err
	}
	quoted := []string{syscall.EscapeArg(exe)}
	for _, arg := range args {
		quoted = append(quoted, syscall.EscapeArg(arg))
	}
	command, err := syscall.UTF16PtrFromString(strings.Join(quoted, " "))
	if err != nil {
		return nil, err
	}
	cwd, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return nil, err
	}
	environment := windowsPSEnvironment(os.Environ())
	sort.SliceStable(environment, func(i, j int) bool { return strings.ToUpper(environment[i]) < strings.ToUpper(environment[j]) })
	var block []uint16
	for _, entry := range environment {
		value, e := syscall.UTF16FromString(entry)
		if e != nil {
			return nil, e
		}
		block = append(block, value...)
	}
	if len(block) == 0 { block = append(block, 0) }
	block = append(block, 0)
	// CREATE_NEW_CONSOLE | CREATE_UNICODE_ENVIRONMENT. No handles inherited.
	si := syscall.StartupInfo{Cb: uint32(unsafe.Sizeof(syscall.StartupInfo{}))}
	var pi syscall.ProcessInformation
	if err = syscall.CreateProcess(app, command, nil, nil, false, 0x00000410, &block[0], cwd, &si, &pi); err != nil {
		return nil, err
	}
	syscall.CloseHandle(pi.Thread)
	return &pi, nil
}
func startReplacementHelper(planPath string) error {
	script, err := installedScript("replace-updater.ps1")
	if err != nil {
		return err
	}
	return launchReplacementHelper(script, planPath, false, 30*time.Second)
}
func launchReplacementHelper(script, planPath string, noPause bool, timeout time.Duration) error {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return err
	}
	var plan replacementPlan
	if err = json.Unmarshal(data, &plan); err != nil {
		return err
	}
	if plan.HandoffToken == "" || plan.StatusPath != filepath.Join(filepath.Dir(plan.Target), updateStateName) {
		return errors.New("invalid self-update handshake plan")
	}
	args := powershellArgs(script, "-PlanPath", planPath)
	if noPause {
		args = append(args, "-NoPause")
	} else {
		// Keep pre-script errors (including policy blocks) visible as well.
		// The helper explicitly exits after its final result/pause.
		args = append([]string{"-NoExit"}, args...)
	}
	pi, err := startConsoleProcess(powershellPath(), args, filepath.Dir(script))
	if err != nil {
		return fmt.Errorf("cannot launch update helper: %w", err)
	}
	defer syscall.CloseHandle(pi.Process)
	exited := func() (bool, error) {
		result, e := syscall.WaitForSingleObject(pi.Process, 0)
		return result == syscall.WAIT_OBJECT_0, e
	}
	return awaitHelperReady(plan, planPath, int(pi.ProcessId), exited, timeout)
}
func helperRunning(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err == syscall.Errno(87) {
		return false, nil
	} // process no longer exists
	if err != nil {
		return false, err
	}
	defer syscall.CloseHandle(h)
	result, err := syscall.WaitForSingleObject(h, 0)
	return result == syscall.WAIT_TIMEOUT, err
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
	return acquireNamedOperationLock(lockName)
}
func acquireNamedOperationLock(lockName string) (func(), error) {
	name, err := syscall.UTF16PtrFromString(lockName)
	if err != nil {
		return nil, err
	}
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

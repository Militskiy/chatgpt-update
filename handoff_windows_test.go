//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This driver exists only in the Go test binary, never in the released app.
// It holds the target file open just like a running image, launches the real
// companion script through the production launcher, then exits after readiness.
func TestHandoffParentProcess(t *testing.T) {
	path := os.Getenv("CHATGPT_UPDATE_TEST_PLAN")
	if path == "" {
		t.Skip("child-process fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan replacementPlan
	if err = json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	plan.ParentPID = os.Getpid()
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	unlock, err := acquireNamedOperationLock(plan.LockName)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	p, err := syscall.UTF16PtrFromString(plan.Target)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	script := filepath.Join(filepath.Dir(plan.Target), "scripts", "replace-updater.ps1")
	noPause := os.Getenv("CHATGPT_UPDATE_TEST_INTERACTIVE") != "1"
	err = launchReplacementHelper(script, path, noPause, 20*time.Second)
	if os.Getenv("CHATGPT_UPDATE_TEST_FAIL") == "1" {
		if err == nil {
			t.Fatal("invalid stage handoff accepted")
		}
		fmt.Println("EXPECTED startup failure:", err)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path+".parent-ready", []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(800 * time.Millisecond) // assertion window for early reopen/lock transfer
}

func fixturePlan(t *testing.T) (replacementPlan, string) {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("dist", "portable"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(source, exeAsset)); os.IsNotExist(err) {
		t.Skip("build.ps1 must run first")
	}
	// Unicode, spaces and apostrophes exercise native argument quoting.
	root := filepath.Join(t.TempDir(), "user's folder Ж")
	if err = os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, ".chatgpt-update-stage-test")
	candidate := filepath.Join(work, "package")
	for _, dest := range []string{root, candidate} {
		if err = os.MkdirAll(filepath.Join(dest, "scripts"), 0700); err != nil {
			t.Fatal(err)
		}
		for _, name := range portableFiles {
			b, e := os.ReadFile(filepath.Join(source, filepath.FromSlash(name)))
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dest, filepath.FromSlash(name)), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	records, err := scanTree(candidate)
	if err != nil {
		t.Fatal(err)
	}
	token := uniqueName()
	plan := replacementPlan{ParentPID: os.Getpid(), Target: filepath.Join(root, exeAsset), Candidate: candidate, NewVersion: version,
		Previous: filepath.Join(root, ".chatgpt-update-previous-test"), Log: filepath.Join(root, "replacement.log"),
		LockName: "Local\\Militskiy.ChatGPTUpdater-" + strings.Repeat("a", 12) + strings.Split(token, "-")[2], Files: records,
		StatusPath: filepath.Join(root, updateStateName), HandoffToken: token}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(work, "replacement.json")
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	return plan, path
}
func waitFixtureFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", path)
}
func TestAutomaticConsoleHandoff(t *testing.T) {
	testAutomaticConsoleHandoff(t, false)
}
func TestAutomaticConsoleHandoffClosesOnSuccess(t *testing.T) {
	testAutomaticConsoleHandoff(t, true)
}
func testAutomaticConsoleHandoff(t *testing.T, interactive bool) {
	t.Helper()
	plan, path := fixturePlan(t)
	root := filepath.Dir(plan.Target)
	oldReadme := []byte("old fixture documentation")
	if err := os.WriteFile(filepath.Join(root, "README.md"), oldReadme, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHandoffParentProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "CHATGPT_UPDATE_TEST_PLAN="+path)
	if interactive {
		cmd.Env = append(cmd.Env, "CHATGPT_UPDATE_TEST_INTERACTIVE=1")
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, path+".parent-ready", 25*time.Second)
	// Reopening early must not obtain a lock or offer another download.
	if unlock, err := acquireNamedOperationLock(plan.LockName); err == nil {
		unlock()
		t.Error("operation lock lost during handoff")
	}
	if err := showLastUpdate(root); err == nil {
		t.Error("running helper was not reported")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("parent fixture: %v\n%s", err, output.String())
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		s, err := readUpdateState(plan.StatusPath)
		if err == nil && (s.Phase == "success" || s.Phase == "failed") {
			if s.Phase != "success" {
				b, _ := os.ReadFile(plan.Log)
				t.Fatalf("helper failed: %+v\n%s", s, b)
			}
			b, err := os.ReadFile(filepath.Join(plan.Previous, "README.md"))
			if err != nil || !bytes.Equal(b, oldReadme) {
				t.Fatal("old package not retained")
			}
			for _, record := range plan.Files {
				h, _, e := hashFile(filepath.Join(root, filepath.FromSlash(record.Path)))
				if e != nil || h != record.SHA256 {
					t.Fatalf("component not updated: %s", record.Path)
				}
			}
			waitForFixtureHelperExit(t, s.HelperPID)
			t.Log("actual new-console launch acknowledged, parent exited, replacement verified, helper exited without keyboard input, rollback copies retained")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	b, _ := os.ReadFile(plan.Log)
	t.Fatalf("helper completion timeout\n%s", b)
}
func TestAutomaticHandoffValidationFailure(t *testing.T) {
	testAutomaticHandoffValidationFailure(t, false)
}
func TestAutomaticHandoffKeepsErrorWindow(t *testing.T) {
	testAutomaticHandoffValidationFailure(t, true)
}
func testAutomaticHandoffValidationFailure(t *testing.T, interactive bool) {
	t.Helper()
	plan, path := fixturePlan(t)
	source := filepath.Join(plan.Candidate, "scripts", "path.ps1")
	f, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\r\n# altered test fixture")
	f.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHandoffParentProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "CHATGPT_UPDATE_TEST_PLAN="+path, "CHATGPT_UPDATE_TEST_FAIL=1")
	if interactive {
		cmd.Env = append(cmd.Env, "CHATGPT_UPDATE_TEST_INTERACTIVE=1")
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parent: %v\n%s", err, b)
	}
	s, err := readUpdateState(plan.StatusPath)
	if err != nil || s.Phase != "failed" || !strings.Contains(s.Message, "Staged component changed") {
		t.Fatalf("failure not persisted: %+v %v", s, err)
	}
	if interactive {
		// Keep only the test-created process handle; cleanup cannot target a
		// future process that happens to reuse the numeric PID.
		h, e := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_TERMINATE, false, uint32(s.HelperPID))
		if e != nil {
			t.Fatalf("error window closed before user input: %v", e)
		}
		defer syscall.CloseHandle(h)
		defer func() {
			// Isolated test helper only; production never terminates a window.
			_ = syscall.TerminateProcess(h, 1)
			_, _ = syscall.WaitForSingleObject(h, 5000)
		}()
		result, e := syscall.WaitForSingleObject(h, 700)
		if e != nil || result != syscall.WAIT_TIMEOUT {
			t.Fatalf("failed helper must wait for input: %d %v", result, e)
		}
	} else {
		waitForFixtureHelperExit(t, s.HelperPID)
	}
	log, err := os.ReadFile(plan.Log)
	if err != nil || !bytes.Contains(log, []byte("[FAIL]")) {
		t.Fatal("failure absent from log")
	}
	if _, err = os.Stat(plan.Previous); !os.IsNotExist(err) {
		t.Fatal("existing package touched before consent handshake")
	}
	if _, err = os.Stat(filepath.Join(filepath.Dir(path), "handoff-continue")); !os.IsNotExist(err) {
		t.Fatal("failed validation granted replacement")
	}
}
func TestNewConsoleStreams(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "console-probe.ps1")
	report := filepath.Join(dir, "console.json")
	script := `param([string]$Report)
[Console]::WriteLine('console output probe')
[Console]::Error.WriteLine('console error probe')
@{InputRedirected=[Console]::IsInputRedirected;OutputRedirected=[Console]::IsOutputRedirected;ErrorRedirected=[Console]::IsErrorRedirected} | ConvertTo-Json | Set-Content -LiteralPath $Report -Encoding ASCII
`
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	pi, err := startConsoleProcess(powershellPath(), powershellArgs(path, "-Report", report), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(pi.Process)
	result, err := syscall.WaitForSingleObject(pi.Process, 15000)
	if err != nil || result != syscall.WAIT_OBJECT_0 {
		t.Fatalf("console process: %v %v", result, err)
	}
	var code uint32
	if err = syscall.GetExitCodeProcess(pi.Process, &code); err != nil || code != 0 {
		t.Fatalf("console exit: %d %v", code, err)
	}
	b, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var streams map[string]bool
	if err = json.Unmarshal(b, &streams); err != nil {
		t.Fatal(err)
	}
	if len(streams) != 3 {
		t.Fatal("missing stream probes")
	}
	for name, redirected := range streams {
		if redirected {
			t.Errorf("%s must belong to the new console, not NUL", name)
		}
	}
}

// Require process termination, not merely a persisted success state. This
// catches both the old unconditional Read-Host and the lingering -NoExit host.
func waitForFixtureHelperExit(t *testing.T, pid int) {
	t.Helper()
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err == syscall.Errno(87) {
		return
	} // Process already exited.
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	result, err := syscall.WaitForSingleObject(h, 10000)
	if err != nil || result != syscall.WAIT_OBJECT_0 {
		_ = syscall.TerminateProcess(h, 1) // Our isolated test child, never production.
		_, _ = syscall.WaitForSingleObject(h, 5000)
		t.Fatalf("helper did not close without input: %d %v", result, err)
	}
}

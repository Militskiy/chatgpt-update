package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const updateStateName = ".chatgpt-update-status.json"

// Diagnostic state is not an authorization to install. Only the private,
// per-attempt acknowledgement in the staging folder authorizes the helper.
type updateState struct {
	Token     string `json:"token"`
	Phase     string `json:"phase"`
	Version   string `json:"version"`
	Message   string `json:"message"`
	Log       string `json:"log"`
	HelperPID int    `json:"helperPid"`
}

func readUpdateState(path string) (updateState, error) {
	var s updateState
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return s, err
	}
	if len(b) > 64*1024 {
		return s, errors.New("self-update status file is too large")
	}
	// Windows PowerShell 5.1 may write UTF-8 BOMs.
	err = json.Unmarshal(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}), &s)
	return s, err
}

// Wait for script-level readiness, not just CreateProcess success. A blocked
// script, dead helper, validation error or timeout never grants replacement.
func awaitHelperReady(plan replacementPlan, planPath string, pid int, exited func() (bool, error), timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s, readErr := readUpdateState(plan.StatusPath)
		valid := readErr == nil && s.Token == plan.HandoffToken && s.HelperPID == pid
		if valid {
			if s.Phase == "failed" {
				return fmt.Errorf("update helper failed: %s (log: %s)", s.Message, plan.Log)
			}
		}
		done, err := exited()
		if err != nil {
			return err
		}
		if done {
			return fmt.Errorf("update helper exited before confirming readiness; inspect its window and log %s. No replacement was authorized", plan.Log)
		}
		if valid {
			if s.Phase == "ready" {
				// Exclusive creation prevents a stale or duplicate grant.
				grant := filepath.Join(filepath.Dir(planPath), "handoff-continue")
				f, e := os.OpenFile(grant, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if e != nil {
					return fmt.Errorf("cannot acknowledge helper: %w", e)
				}
				_, e = f.WriteString(plan.HandoffToken)
				closeErr := f.Close()
				if e != nil {
					os.Remove(grant)
					return e
				}
				if closeErr != nil {
					os.Remove(grant)
					return closeErr
				}
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("update helper did not confirm readiness in %s; no replacement was authorized. Inspect the helper window (script policy may block it) and %s", timeout, plan.Log)
}

// Shown on the next normal launch; --version remains machine-readable.
func showLastUpdate(root string) error {
	s, err := readUpdateState(filepath.Join(root, updateStateName))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		fmt.Println("[WARN] Cannot read last self-update status:", err)
		return nil
	}
	switch s.Phase {
	case "starting", "ready", "applying":
		alive, e := helperRunning(s.HelperPID)
		if e != nil {
			return fmt.Errorf("cannot determine whether update helper %d is still running: %w; check %s", s.HelperPID, e, s.Log)
		}
		if alive {
			return fmt.Errorf("self-update to %s is still finishing in the helper window. Wait for its [OK] result, then reopen the app. Log: %s", s.Version, s.Log)
		}
		fmt.Printf("[FAIL] Previous self-update to %s stopped before reporting completion. Check %s before retrying.\n", s.Version, s.Log)
	case "failed":
		fmt.Printf("[FAIL] Last self-update to %s: %s\nLog: %s\n", s.Version, strings.TrimSpace(s.Message), s.Log)
	case "success":
		fmt.Printf("[OK] Last self-update completed: %s.\n", s.Version)
	}
	return nil
}

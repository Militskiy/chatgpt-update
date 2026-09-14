package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStatus(t *testing.T, path string, state updateState) {
	t.Helper()
	b, err := json.Marshal(state)
	if err != nil { t.Fatal(err) }
	if err = os.WriteFile(path, b, 0600); err != nil { t.Fatal(err) }
}
func TestHandoffAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, phase, token string
		pid int
		dead, wantGrant bool
	}{
		{"ready", "ready", "attempt", 123, false, true},
		{"ready but exited", "ready", "attempt", 123, true, false},
		{"validation failure", "failed", "attempt", 123, false, false},
		{"stale token", "ready", "old", 123, true, false},
		{"wrong process", "ready", "attempt", 99, true, false},
		{"exited without readiness", "starting", "attempt", 123, true, false},
		{"timeout", "starting", "attempt", 123, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			plan := replacementPlan{StatusPath: filepath.Join(root, updateStateName), HandoffToken: "attempt", Log: "test.log"}
			path := filepath.Join(root, "replacement.json")
			testStatus(t, plan.StatusPath, updateState{Token: tc.token, Phase: tc.phase, HelperPID: tc.pid, Message: "validation failed"})
			err := awaitHelperReady(plan, path, 123, func() (bool, error) { return tc.dead, nil }, 150*time.Millisecond)
			grant, e := os.ReadFile(filepath.Join(root, "handoff-continue"))
			if tc.wantGrant {
				if err != nil || e != nil || string(grant) != "attempt" { t.Fatalf("grant failed: %v %v %q", err, e, grant) }
			} else {
				if err == nil { t.Fatal("missing failure") }
				if !os.IsNotExist(e) { t.Fatal("failed handshake granted replacement") }
			}
		})
	}
}
func TestStatusBOMAndSize(t *testing.T) {
	p := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(p, append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"phase":"success","version":"0.2.2"}`)...), 0600); err != nil { t.Fatal(err) }
	s, err := readUpdateState(p)
	if err != nil || s.Version != "0.2.2" { t.Fatalf("BOM: %v %v", s, err) }
	os.WriteFile(p, []byte(strings.Repeat(" ", 70*1024)), 0600)
	if _, err = readUpdateState(p); err == nil { t.Fatal("unbounded status accepted") }
}

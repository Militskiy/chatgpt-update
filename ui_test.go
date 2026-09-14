package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUIPreferences(t *testing.T) {
	args, opts := parseUIOptions([]string{"--no-color", "update", "--no-backup", "--plain", "--skip-update-check", "--no-animation"})
	if !reflect.DeepEqual(args, []string{"update", "--no-backup"}) || !opts.Plain || !opts.NoColor || !opts.NoAnimation || !opts.SkipUpdateCheck {
		t.Fatal(args, opts)
	}
	previous := presentation
	defer func() { presentation = previous }()
	presentation = opts
	env := uiEnvironment([]string{"PATH=original", "HTTPS_PROXY=https://proxy", "NO_COLOR=existing"})
	for _, entry := range []string{"PATH=original", "HTTPS_PROXY=https://proxy", "NO_COLOR=1", "CHATGPT_UPDATER_PLAIN=1", "CHATGPT_UPDATER_NO_ANIMATION=1"} {
		found := false
		for _, v := range env {
			if v == entry {
				found = true
			}
		}
		if !found {
			t.Error("missing", entry)
		}
	}
	for _, entry := range env {
		if strings.Contains(entry, "ExecutionPolicy") {
			t.Fatal("UI changed policy")
		}
	}
}
func TestPlainActivityHasNoControlSequences(t *testing.T) {
	var out bytes.Buffer
	done := newActivity(&out, "Checking releases", false, false, func() int { return 80 })
	done("OK")
	done("OK")
	if strings.ContainsAny(out.String(), "\x1b\r") || strings.Count(out.String(), "Checking releases") != 1 {
		t.Fatal(out.String())
	}
}
func TestActivityStopsBeforePrompt(t *testing.T) {
	var out bytes.Buffer
	done := newActivity(&out, "Working", true, true, func() int { return 65 })
	time.Sleep(270 * time.Millisecond)
	done("OK")
	done("FAIL")
	out.WriteString("Install? [Y/N]: ")
	snapshot := out.String()
	time.Sleep(150 * time.Millisecond)
	if out.String() != snapshot || !strings.HasSuffix(snapshot, "Install? [Y/N]: ") || strings.Count(snapshot, "[OK]") != 1 {
		t.Fatal("rendering continued after stop")
	}
	if !strings.Contains(snapshot, "\x1b[96m") {
		t.Fatal("missing activity color")
	}
}
func TestDownloadUITruthful(t *testing.T) {
	half := formatDownload(50<<20, 100<<20, 10*time.Second, false)
	if strings.Count(half, "%") != 1 || !strings.Contains(half, "50.0%") || strings.Count(half, "#") != 12 || !strings.Contains(half, "5.0 MiB/s") || !strings.Contains(half, "ETA 00:10") {
		t.Fatal(half)
	}
	if s := formatDownload(100, 100, time.Second, false); strings.Contains(s, "100.0%") {
		t.Fatal("premature completion", s)
	}
	if s := formatDownload(100, 100, time.Second, true); !strings.Contains(s, "100.0%") {
		t.Fatal("missing completion", s)
	}
	if s := formatDownload(0, 0, 0, false); strings.Contains(s, "%") {
		t.Fatal("fake unknown-size percentage")
	}
	if s := oneLine("bad\x1b[31m\nmessage", 15); strings.ContainsAny(s, "\x1b\n") || len([]rune(s)) > 15 {
		t.Fatal("unbounded or unsafe terminal line")
	}
	if s := colored(false, cyan, "plain"); s != "plain" {
		t.Fatal(s)
	}
}

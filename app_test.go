package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTest(t *testing.T, path, data string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
}
func readTest(t *testing.T, path string) string {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestBackupAndRestore(t *testing.T) {
	h := t.TempDir()
	state := filepath.Join(h, ".codex")
	root := filepath.Join(h, "CodexBackups")
	writeTest(t, filepath.Join(state, "config.toml"), "version A")
	writeTest(t, filepath.Join(state, "sessions", "example.jsonl"), "private state")
	os.MkdirAll(filepath.Join(state, "empty-directory"), 0700)
	saved, e := createBackup(state, root)
	if e != nil {
		t.Fatal(e)
	}
	if _, legacy, e := verifyBackup(saved); e != nil || legacy {
		t.Fatalf("verify %v %v", legacy, e)
	}
	writeTest(t, filepath.Join(state, "config.toml"), "version B")
	writeTest(t, filepath.Join(state, "new.txt"), "live changes")
	safety, e := restoreBackup(saved, state, root)
	if e != nil {
		t.Fatal(e)
	}
	if readTest(t, filepath.Join(state, "config.toml")) != "version A" {
		t.Fatal("restore mismatch")
	}
	if readTest(t, filepath.Join(safety, ".codex", "config.toml")) != "version B" {
		t.Fatal("old live state not preserved")
	}
	if readTest(t, filepath.Join(safety, ".codex", "new.txt")) != "live changes" {
		t.Fatal("new live file lost")
	}
	if _, e := os.Stat(filepath.Join(state, "new.txt")); !os.IsNotExist(e) {
		t.Fatal("restore must replace, not merge")
	}
	if _, e := os.Stat(filepath.Join(state, "empty-directory")); e != nil {
		t.Fatal("empty directory not preserved")
	}
	if _, legacy, e := verifyBackup(safety); e != nil || legacy {
		t.Fatalf("safety backup invalid %v", e)
	}
	if _, e := restoreBackup(safety, state, root); e != nil {
		t.Fatal(e)
	}
	if readTest(t, filepath.Join(state, "config.toml")) != "version B" {
		t.Fatal("undo restore failed")
	}
}
func TestRestoreTamperedBackupDoesNotChangeLiveState(t *testing.T) {
	h := t.TempDir()
	s := filepath.Join(h, ".codex")
	r := filepath.Join(h, "CodexBackups")
	writeTest(t, filepath.Join(s, "file"), "original")
	b, e := createBackup(s, r)
	if e != nil {
		t.Fatal(e)
	}
	writeTest(t, filepath.Join(b, ".codex", "file"), "corrupt")
	if _, e := restoreBackup(b, s, r); e == nil {
		t.Fatal("tampered backup accepted")
	}
	if readTest(t, filepath.Join(s, "file")) != "original" {
		t.Fatal("live state changed")
	}
}
func TestBackupRejectsSymlinks(t *testing.T) {
	h := t.TempDir()
	s := filepath.Join(h, ".codex")
	r := filepath.Join(h, "CodexBackups")
	writeTest(t, filepath.Join(s, "file"), "text")
	if e := os.Symlink(filepath.Join(s, "file"), filepath.Join(s, "link")); e != nil {
		t.Skip("symlink privilege unavailable")
	}
	if _, e := createBackup(s, r); e == nil {
		t.Fatal("symlink accepted")
	}
	list, e := listBackups(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(list) != 0 {
		t.Fatal("incomplete backup published")
	}
}
func TestLegacyBackupAndMissingLiveState(t *testing.T) {
	h := t.TempDir()
	r := filepath.Join(h, "CodexBackups")
	source := filepath.Join(r, "legacy")
	writeTest(t, filepath.Join(source, ".codex", "config.toml"), "legacy")
	_, legacy, e := verifyBackup(source)
	if e != nil || !legacy {
		t.Fatal("legacy detection failed")
	}
	s := filepath.Join(h, ".codex")
	safety, e := restoreBackup(source, s, r)
	if e != nil || safety != "" {
		t.Fatalf("restore %v", e)
	}
	if readTest(t, filepath.Join(s, "config.toml")) != "legacy" {
		t.Fatal("legacy restore failed")
	}
}
func TestRejectPathTraversalManifest(t *testing.T) {
	r := filepath.Join(t.TempDir(), "backup")
	writeTest(t, filepath.Join(r, ".codex", "file"), "x")
	m := backupManifest{Schema: 1, Scope: ".codex", Files: []fileRecord{{Path: "../../outside", Size: 1, SHA256: strings.Repeat("0", 64)}}}
	b, _ := json.Marshal(m)
	writeTest(t, filepath.Join(r, "backup.json"), string(b))
	if _, _, e := verifyBackup(r); e == nil {
		t.Fatal("traversal manifest accepted")
	}
}
func TestVersionOrdering(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"v0.2.0", "0.1.9", true}, {"0.1.10", "0.1.9", true}, {"1.0.0", "1.0.0", false}, {"0.1.0", "0.2.0", false}, {"2.0.0", "1.99.99", true}} {
		actual, e := newerVersion(c.a, c.b)
		if e != nil || actual != c.want {
			t.Fatalf("%+v -> %v %v", c, actual, e)
		}
	}
	for _, s := range []string{"latest", "v1.0.0-rc1", "1.0", "1.0.0.1", "01.2.3", "1.2.9999999999999999"} {
		if _, e := parseVersion(s); e == nil {
			t.Fatalf("bad version accepted: %s", s)
		}
	}
}
func TestSelfUpdateURLAndAssetValidation(t *testing.T) {
	good := "https://github.com/Militskiy/chatgpt-update/releases/download/v0.1.0/chatgpt-update.exe"
	if e := validateAssetURL(good); e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"http://github.com/Militskiy/chatgpt-update/releases/download/v1/file.exe", "https://github.com/other/repo/releases/download/v1/file.exe", "https://github.com.evil.test/Militskiy/chatgpt-update/releases/download/v1/file.exe", "https://name@github.com/Militskiy/chatgpt-update/releases/download/v1/file.exe", "file:///tmp/app.exe"} {
		if e := validateAssetURL(s); e == nil {
			t.Fatalf("bad URL accepted: %s", s)
		}
	}
	a := releaseAsset{Name: exeAsset, URL: good, State: "uploaded", Size: 2048}
	r := releaseInfo{Tag: "v0.1.0", Assets: []releaseAsset{a}}
	if _, _, e := chooseExecutable(r); e != nil {
		t.Fatal(e)
	}
	r.Prerelease = true
	if _, _, e := chooseExecutable(r); e == nil {
		t.Fatal("prerelease accepted")
	}
	r.Prerelease = false
	r.Assets = append(r.Assets, a)
	if _, _, e := chooseExecutable(r); e == nil {
		t.Fatal("duplicate asset accepted")
	}
}
func TestChecksumParser(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, text := range []string{hash + "  chatgpt-update.exe\n", hash + " *chatgpt-update.exe\r\n"} {
		h, e := checksumFromText(text, exeAsset)
		if e != nil || h != hash {
			t.Fatalf("%v %s", e, h)
		}
	}
	for _, text := range []string{"bad  chatgpt-update.exe", hash + "  other.exe", hash + "  chatgpt-update.exe\n" + hash + "  chatgpt-update.exe"} {
		if _, e := checksumFromText(text, exeAsset); e == nil {
			t.Fatal("invalid checksum accepted")
		}
	}
}
func TestUpdateArguments(t *testing.T) {
	got, e := updateArgs([]string{"check", "--exact", "--plain"})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Join(got, " ") != "-CheckOnly -ExactVersionOnly -PlainOutput" {
		t.Fatal(got)
	}
	for _, a := range [][]string{{"update", "--backup", "--no-backup"}, {"update", "--eval"}, {"update", "--plain", "--plain"}} {
		if _, e := updateArgs(a); e == nil {
			t.Fatal("invalid arguments accepted")
		}
	}
}
func TestEmbeddedResources(t *testing.T) {
	if _, e := parseVersion(version); e != nil {
		t.Fatal(e)
	}
	for _, f := range []string{"update-chatgpt.ps1", "prepare-state.ps1", "path.ps1", "replace-updater.ps1"} {
		b, e := assets.ReadFile("scripts/" + f)
		if e != nil || len(b) < 50 {
			t.Fatal(f, e)
		}
	}
}
func TestRecordsRejectDuplicates(t *testing.T) {
	r := fileRecord{Path: "a", Size: 1, SHA256: "x"}
	if recordsEqual([]fileRecord{r, r}, []fileRecord{r, r}) {
		t.Fatal("duplicate manifest accepted")
	}
}

package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type fileRecord struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type backupManifest struct {
	Schema      int          `json:"schema"`
	CreatedUTC  string       `json:"createdUtc"`
	ToolVersion string       `json:"toolVersion"`
	Scope       string       `json:"scope"`
	Files       []fileRecord `json:"files"`
}

func uniqueName() string {
	var b [6]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}
func defaultStatePaths() (string, string, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return "", "", e
	}
	if custom := os.Getenv("CODEX_HOME"); custom != "" && !strings.EqualFold(filepath.Clean(custom), filepath.Join(home, ".codex")) {
		return "", "", errors.New("CODEX_HOME points to a custom location; this release only manages USERPROFILE\\.codex. No state was changed")
	}
	return filepath.Join(home, ".codex"), filepath.Join(home, "CodexBackups"), nil
}
func hashFile(path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, e
}
func assertRealDirectory(path string) error {
	f, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !f.IsDir() || isReparsePoint(path) {
		return fmt.Errorf("not a regular directory (links/junctions are not followed): %s", path)
	}
	return nil
}
func ensureRealDirectory(path string) error {
	if e := os.MkdirAll(path, 0700); e != nil {
		return e
	}
	return assertRealDirectory(path)
}

// Never traverse a symlink or Windows reparse point. Snapshots contain regular files only.
func scanTree(root string) ([]fileRecord, error) {
	if e := assertRealDirectory(root); e != nil {
		return nil, e
	}
	records := make([]fileRecord, 0)
	e := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if isReparsePoint(path) {
			return fmt.Errorf("link/reparse point is not supported in a snapshot: %s", path)
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported state file: %s", path)
		}
		hash, size, e := hashFile(path)
		if e != nil {
			return e
		}
		after, e := os.Stat(path)
		if e != nil {
			return e
		}
		if info.Size() != size || after.Size() != size || !info.ModTime().Equal(after.ModTime()) {
			return fmt.Errorf("file changed during snapshot; close all Codex sessions: %s", path)
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		records = append(records, fileRecord{filepath.ToSlash(rel), size, hash})
		return nil
	})
	return records, e
}
func copyTree(source, target string) error {
	if e := assertRealDirectory(source); e != nil {
		return e
	}
	if e := ensureRealDirectory(target); e != nil {
		return e
	}
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if isReparsePoint(path) {
			return fmt.Errorf("links/junctions are not copied: %s", path)
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		dest := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		before, e := os.Lstat(path)
		if e != nil {
			return e
		}
		if !before.Mode().IsRegular() {
			return fmt.Errorf("not a regular file: %s", path)
		}
		in, e := os.Open(path)
		if e != nil {
			return e
		}
		out, e := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			in.Close()
			return e
		}
		n, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		after, e := os.Stat(path)
		if e != nil {
			return e
		}
		if n != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
			return fmt.Errorf("file changed while copying; close all Codex sessions: %s", path)
		}
		return os.Chtimes(dest, before.ModTime(), before.ModTime())
	})
}
func recordsEqual(a, b []fileRecord) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]fileRecord, len(a))
	for _, r := range a {
		if _, ok := m[r.Path]; ok {
			return false
		}
		m[r.Path] = r
	}
	for _, r := range b {
		if expected, ok := m[r.Path]; !ok || expected != r {
			return false
		}
		delete(m, r.Path)
	}
	return len(m) == 0
}
func writeManifest(dir string, records []fileRecord) error {
	m := backupManifest{1, time.Now().UTC().Format(time.RFC3339), version, ".codex", records}
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "backup.json"), b, 0600)
}
func createBackup(state, root string) (string, error) {
	if e := assertRealDirectory(state); e != nil {
		return "", e
	}
	if e := ensureRealDirectory(root); e != nil {
		return "", e
	}
	name := uniqueName()
	staging := filepath.Join(root, ".incomplete-"+name)
	target := filepath.Join(root, name)
	if e := os.Mkdir(staging, 0700); e != nil {
		return "", e
	}
	defer os.RemoveAll(staging)
	if e := copyTree(state, filepath.Join(staging, ".codex")); e != nil {
		return "", e
	}
	copied, e := scanTree(filepath.Join(staging, ".codex"))
	if e != nil {
		return "", e
	}
	original, e := scanTree(state)
	if e != nil {
		return "", e
	}
	if !recordsEqual(original, copied) {
		return "", errors.New("state changed or copy verification failed; no completed backup was published")
	}
	if e := writeManifest(staging, copied); e != nil {
		return "", e
	}
	if e := os.Rename(staging, target); e != nil {
		return "", e
	}
	return target, nil
}
func backupInteractive() error {
	state, root, e := defaultStatePaths()
	if e != nil {
		return e
	}
	if e := assertRealDirectory(state); e != nil {
		return fmt.Errorf("nothing to back up: %w", e)
	}
	fmt.Println("Scope:", state, "only. Projects and other desktop-app data are NOT included.")
	fmt.Println("Backups may contain tokens and credentials. They remain local; nothing is uploaded.")
	fmt.Println("Finish tasks and close all Codex CLI/IDE sessions. This closes the desktop app.")
	if !confirm("Create backup?") {
		return nil
	}
	if e := runScript("prepare-state.ps1"); e != nil {
		return e
	}
	fmt.Println("[>>] Copying and verifying state...")
	path, e := createBackup(state, root)
	if e != nil {
		return e
	}
	fmt.Println("[OK] Backup saved:", path)
	return nil
}
func listBackups(root string) ([]string, error) {
	if _, e := os.Stat(root); os.IsNotExist(e) {
		return nil, nil
	}
	if e := assertRealDirectory(root); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(root)
	if e != nil {
		return nil, e
	}
	list := []string{}
	for _, d := range entries {
		path := filepath.Join(root, d.Name())
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") || isReparsePoint(path) {
			continue
		}
		if assertRealDirectory(filepath.Join(path, ".codex")) == nil {
			list = append(list, path)
		}
	}
	sort.Slice(list, func(i, j int) bool { return filepath.Base(list[i]) > filepath.Base(list[j]) })
	return list, nil
}
func verifyBackup(path string) ([]fileRecord, bool, error) {
	if e := assertRealDirectory(path); e != nil {
		return nil, false, e
	}
	actual, e := scanTree(filepath.Join(path, ".codex"))
	if e != nil {
		return nil, false, e
	}
	manifestPath := filepath.Join(path, "backup.json")
	f, e := os.Open(manifestPath)
	if os.IsNotExist(e) {
		return actual, true, nil
	}
	if e != nil {
		return nil, false, e
	}
	defer f.Close()
	if isReparsePoint(manifestPath) {
		return nil, false, errors.New("backup manifest cannot be a link")
	}
	var m backupManifest
	dec := json.NewDecoder(io.LimitReader(f, 64<<20))
	if e := dec.Decode(&m); e != nil {
		return nil, false, e
	}
	if m.Schema != 1 || m.Scope != ".codex" || !recordsEqual(m.Files, actual) {
		return nil, false, errors.New("backup manifest/hash verification failed; no state was replaced")
	}
	return actual, false, nil
}

// Stage and verify before changing live state. Preserve old state and roll back a failed swap.
func restoreBackup(source, state, backupRoot string) (string, error) {
	records, _, e := verifyBackup(source)
	if e != nil {
		return "", e
	}
	if e := ensureRealDirectory(filepath.Dir(state)); e != nil {
		return "", e
	}
	if e := ensureRealDirectory(backupRoot); e != nil {
		return "", e
	}
	staging := filepath.Join(filepath.Dir(state), ".codex-restoring-"+uniqueName())
	if e := os.Mkdir(staging, 0700); e != nil {
		return "", e
	}
	defer os.RemoveAll(staging)
	if e := copyTree(filepath.Join(source, ".codex"), staging); e != nil {
		return "", e
	}
	ready, e := scanTree(staging)
	if e != nil {
		return "", e
	}
	if !recordsEqual(records, ready) {
		return "", errors.New("staged restore differs from backup; no state was replaced")
	}
	safety := ""
	if _, e := os.Lstat(state); e == nil {
		current, e := scanTree(state)
		if e != nil {
			return "", e
		}
		safety = filepath.Join(backupRoot, uniqueName()+"-before-restore")
		if e := os.Mkdir(safety, 0700); e != nil {
			return "", e
		}
		if e := writeManifest(safety, current); e != nil {
			os.RemoveAll(safety)
			return "", e
		}
		if e := os.Rename(state, filepath.Join(safety, ".codex")); e != nil {
			os.RemoveAll(safety)
			return "", fmt.Errorf("cannot preserve current state: %w", e)
		}
	} else if !os.IsNotExist(e) {
		return "", e
	}
	if e := os.Rename(staging, state); e != nil {
		if safety != "" {
			if rollback := os.Rename(filepath.Join(safety, ".codex"), state); rollback != nil {
				return safety, fmt.Errorf("restore failed (%v); rollback failed (%v). Original state retained at %s", e, rollback, safety)
			}
		}
		return safety, fmt.Errorf("restore failed; previous state restored where possible: %w", e)
	}
	return safety, nil
}
func restoreInteractive() error {
	state, root, e := defaultStatePaths()
	if e != nil {
		return e
	}
	list, e := listBackups(root)
	if e != nil {
		return e
	}
	if len(list) == 0 {
		fmt.Println("No backups with a .codex subfolder found in", root)
		return nil
	}
	fmt.Println("Available backups:")
	for i, path := range list {
		fmt.Printf("%d) %s\n", i+1, filepath.Base(path))
	}
	a, e := readLine("Backup number (0 cancels): ")
	if e != nil {
		return e
	}
	n, e := strconv.Atoi(a)
	if e != nil || n < 0 || n > len(list) {
		return errors.New("invalid backup number")
	}
	if n == 0 {
		return nil
	}
	source := list[n-1]
	fmt.Println("[>>] Checking backup integrity...")
	_, legacy, e := verifyBackup(source)
	if e != nil {
		return e
	}
	if legacy {
		fmt.Println("WARNING: Legacy script backup without a hash manifest. Only restore backups you trust.")
	}
	fmt.Println("Restore FROM:", source)
	fmt.Println("Restore TO:  ", state)
	fmt.Println("This replaces .codex settings/history/credentials, NOT the app version or project folders.")
	fmt.Println("Current state is preserved as a separate before-restore backup.")
	fmt.Println("Finish tasks and close all Codex CLI/IDE sessions. The desktop app will be closed.")
	a, e = readLine("Type RESTORE to continue (anything else cancels): ")
	if e != nil || a != "RESTORE" {
		return nil
	}
	if e := runScript("prepare-state.ps1"); e != nil {
		return e
	}
	safety, e := restoreBackup(source, state, root)
	if e != nil {
		return e
	}
	fmt.Println("[OK] Backup restored. Open ChatGPT/Codex when ready.")
	if safety != "" {
		fmt.Println("Previous state saved:", safety)
	}
	return nil
}

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var scriptNames = []string{"path.ps1", "prepare-state.ps1", "replace-updater.ps1", "update-chatgpt.ps1"}

func powershellArgs(script string, args ...string) []string {
	return append([]string{"-NoLogo", "-NoProfile", "-File", script}, args...)
}
func operationLockName() (string, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256([]byte(strings.ToLower(home)))
	return fmt.Sprintf("Local\\Militskiy.ChatGPTUpdater-%x", sum[:12]), nil
}
func expectedScripts() (map[string]string, error) {
	b, e := assets.ReadFile("script-hashes.json")
	if e != nil {
		return nil, e
	}
	m := map[string]string{}
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, e
	}
	if len(m) != len(scriptNames) {
		return nil, errors.New("invalid embedded script hash manifest")
	}
	for _, name := range scriptNames {
		h, e := hex.DecodeString(m[name])
		if e != nil || len(h) != 32 {
			return nil, errors.New("invalid script hash")
		}
	}
	return m, nil
}
func verifiedScript(root, name string) (string, error) {
	hashes, e := expectedScripts()
	if e != nil {
		return "", e
	}
	expected, ok := hashes[name]
	if !ok {
		return "", errors.New("unknown script name")
	}
	dir := filepath.Join(root, "scripts")
	if e = assertRealDirectory(root); e != nil {
		return "", e
	}
	if e = assertRealDirectory(dir); e != nil {
		return "", fmt.Errorf("extract the complete portable ZIP, including scripts: %w", e)
	}
	path := filepath.Join(dir, name)
	info, e := os.Lstat(path)
	if e != nil {
		return "", fmt.Errorf("missing companion script; extract the complete release ZIP: %w", e)
	}
	if !info.Mode().IsRegular() || isReparsePoint(path) {
		return "", errors.New("script cannot be a link/reparse point")
	}
	actual, _, e := hashFile(path)
	if e != nil {
		return "", e
	}
	if actual != expected {
		return "", fmt.Errorf("%s differs from this EXE's reviewed build; restore the complete release package, or rebuild after approved changes", name)
	}
	return path, nil
}
func verifyScripts(root string) error {
	for _, name := range scriptNames {
		if _, e := verifiedScript(root, name); e != nil {
			return e
		}
	}
	return nil
}

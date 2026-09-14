package main

import (
	"archive/zip"
	"debug/pe"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var portableFiles = []string{"chatgpt-update.exe", "VERSION", "README.md", "scripts/path.ps1", "scripts/prepare-state.ps1", "scripts/replace-updater.ps1", "scripts/update-chatgpt.ps1", "FILES.sha256"}

// Exact allowlist also rejects traversal, NTFS alternate streams, links, extra
// executables, duplicate/case-colliding entries and accidental nested packages.
func validateZipEntries(files []*zip.File) error {
	allowed := map[string]bool{}
	for _, name := range portableFiles {
		allowed[name] = true
	}
	seen := map[string]bool{}
	var total uint64
	for _, f := range files {
		if f.FileInfo().IsDir() {
			if f.Name != "scripts/" {
				return fmt.Errorf("unexpected archive directory: %s", f.Name)
			}
			continue
		}
		if !allowed[f.Name] || seen[f.Name] || !f.Mode().IsRegular() {
			return fmt.Errorf("unexpected, duplicate or nonregular archive entry: %s", f.Name)
		}
		if (f.Name != exeAsset && f.UncompressedSize64 > 2<<20) || f.UncompressedSize64 > 100<<20 {
			return errors.New("archive entry exceeds size limit")
		}
		total += f.UncompressedSize64
		if total > 200<<20 {
			return errors.New("unpacked archive exceeds size limit")
		}
		seen[f.Name] = true
	}
	if len(seen) != len(allowed) {
		return errors.New("incomplete portable package; companion files are required")
	}
	return nil
}
func extractPortable(archive, destination, nextVersion string) ([]fileRecord, error) {
	z, e := zip.OpenReader(archive)
	if e != nil {
		return nil, e
	}
	defer z.Close()
	if e = validateZipEntries(z.File); e != nil {
		return nil, e
	}
	if e = os.Mkdir(destination, 0700); e != nil {
		return nil, e
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(destination)
		}
	}()
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		dest := filepath.Join(destination, filepath.FromSlash(f.Name))
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return nil, e
		}
		input, e := f.Open()
		if e != nil {
			return nil, e
		}
		output, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			input.Close()
			return nil, e
		}
		n, copyErr := io.Copy(output, io.LimitReader(input, int64(f.UncompressedSize64)+1))
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if uint64(n) != f.UncompressedSize64 {
			return nil, errors.New("unpacked length mismatch")
		}
	}
	records, e := validatePortableFolder(destination, nextVersion)
	if e != nil {
		return nil, e
	}
	ok = true
	return records, nil
}
func validatePortableFolder(root, nextVersion string) ([]fileRecord, error) {
	if _, e := parseVersion(nextVersion); e != nil {
		return nil, e
	}
	records, e := scanTree(root)
	if e != nil {
		return nil, e
	}
	if len(records) != len(portableFiles) {
		return nil, errors.New("incorrect package file count")
	}
	contents, e := os.ReadFile(filepath.Join(root, "FILES.sha256"))
	if e != nil {
		return nil, e
	}
	text := string(contents)
	if len(strings.Fields(text)) != (len(portableFiles)-1)*2 {
		return nil, errors.New("unexpected package checksum entries")
	}
	actual := map[string]string{}
	for _, file := range records {
		actual[file.Path] = file.SHA256
	}
	for _, name := range portableFiles {
		if actual[name] == "" {
			return nil, fmt.Errorf("missing package file %s", name)
		}
		if name == "FILES.sha256" {
			continue
		}
		expected, e := checksumFromText(text, name)
		if e != nil || expected != actual[name] {
			return nil, fmt.Errorf("component hash mismatch: %s", name)
		}
	}
	v, e := os.ReadFile(filepath.Join(root, "VERSION"))
	if e != nil || strings.TrimSpace(string(v)) != nextVersion {
		return nil, errors.New("package VERSION does not match release")
	}
	exe, e := pe.Open(filepath.Join(root, exeAsset))
	if e != nil {
		return nil, fmt.Errorf("invalid Windows executable: %w", e)
	}
	defer exe.Close()
	if exe.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return nil, errors.New("package EXE is not Windows x64")
	}
	return records, nil
}

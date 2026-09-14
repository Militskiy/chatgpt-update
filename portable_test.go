package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packageFixture() map[string][]byte {
	m := map[string][]byte{}
	for _, name := range portableFiles {
		if name != "FILES.sha256" {
			m[name] = []byte("ordinary file\n")
		}
	}
	m["VERSION"] = []byte("0.2.0\n")
	b := make([]byte, 512)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[0x3c:], 0x80)
	copy(b[0x80:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(b[0x84:], 0x8664)
	m[exeAsset] = b
	var manifest strings.Builder
	for _, name := range portableFiles {
		if name != "FILES.sha256" {
			fmt.Fprintf(&manifest, "%x  %s\n", sha256.Sum256(m[name]), name)
		}
	}
	m["FILES.sha256"] = []byte(manifest.String())
	return m
}
func fixtureZip(t *testing.T, m map[string][]byte, extra string, symlink bool) string {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, name := range portableFiles {
		if data, ok := m[name]; ok {
			f, e := z.Create(name)
			if e != nil {
				t.Fatal(e)
			}
			f.Write(data)
		}
	}
	if extra != "" {
		h := &zip.FileHeader{Name: extra, Method: zip.Store}
		if symlink {
			h.SetMode(os.ModeSymlink | 0700)
		}
		f, e := z.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		f.Write([]byte("target"))
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "package.zip")
	os.WriteFile(p, b.Bytes(), 0600)
	return p
}
func TestPortableRoundTrip(t *testing.T) {
	archive := fixtureZip(t, packageFixture(), "", false)
	dest := filepath.Join(t.TempDir(), "unpacked")
	files, e := extractPortable(archive, dest, "0.2.0")
	if e != nil || len(files) != len(portableFiles) {
		t.Fatal(e, files)
	}
	if _, e := validatePortableFolder(dest, "0.2.1"); e == nil {
		t.Fatal("version mismatch accepted")
	}
	os.WriteFile(filepath.Join(dest, "scripts/path.ps1"), []byte("changed"), 0600)
	if _, e := validatePortableFolder(dest, "0.2.0"); e == nil {
		t.Fatal("changed component accepted")
	}
}
func TestRejectUnsafeZIPs(t *testing.T) {
	for _, name := range []string{"../escape.exe", "/absolute", "C:/evil.exe", "scripts/path.ps1:evil", "scripts/PATH.ps1", "scripts/path.ps1", "additional.exe", "scripts\\path.ps1"} {
		t.Run(name, func(t *testing.T) {
			a := fixtureZip(t, packageFixture(), name, false)
			d := filepath.Join(t.TempDir(), "out")
			if _, e := extractPortable(a, d, "0.2.0"); e == nil {
				t.Fatal("unsafe ZIP accepted")
			}
		})
	}
	m := packageFixture()
	delete(m, "scripts/path.ps1")
	if _, e := extractPortable(fixtureZip(t, m, "scripts/path.ps1", true), filepath.Join(t.TempDir(), "out"), "0.2.0"); e == nil {
		t.Fatal("link accepted")
	}
	if _, e := extractPortable(fixtureZip(t, m, "", false), filepath.Join(t.TempDir(), "out"), "0.2.0"); e == nil {
		t.Fatal("missing sidecar accepted")
	}
}
func TestBadPackageHashCleansStage(t *testing.T) {
	m := packageFixture()
	m["scripts/path.ps1"] = []byte("changed")
	dest := filepath.Join(t.TempDir(), "out")
	if _, e := extractPortable(fixtureZip(t, m, "", false), dest, "0.2.0"); e == nil {
		t.Fatal("bad hash accepted")
	}
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("failed stage was not cleaned")
	}
}

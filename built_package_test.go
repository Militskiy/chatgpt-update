package main

import (
    "os"
    "path/filepath"
    "testing"
)

// Validate the actual Windows-built release archive, not only synthetic ZIPs.
func TestActualReleaseZIP(t *testing.T) {
    archive := filepath.Join("dist", portableAsset)
    if _, err := os.Stat(archive); os.IsNotExist(err) {
        t.Skip("release ZIP not built on this development machine; mandatory in Windows build CI")
    } else if err != nil {
        t.Fatal(err)
    }
    files, err := extractPortable(archive, filepath.Join(t.TempDir(), "package"), version)
    if err != nil || len(files) != len(portableFiles) {
        t.Fatalf("actual release package rejected: %v (%d files)", err, len(files))
    }
}

//go:build !windows

package main

import (
	"errors"
	"os"
)

func runScript(name string, args ...string) error { return errors.New("Windows is required") }
func startReplacementHelper(path string) error    { return errors.New("Windows is required") }
func isReparsePoint(path string) bool {
	f, e := os.Lstat(path)
	return e != nil || f.Mode()&os.ModeSymlink != 0
}
func acquireOperationLock() (func(), error) { return func() {}, nil }

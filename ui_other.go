//go:build !windows

package main

func interactiveConsole() bool             { return false }
func enableTerminalOutput() (bool, func()) { return false, func() {} }
func terminalColumns() int                 { return 78 }

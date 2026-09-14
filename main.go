// ChatGPT Update is a portable console frontend for the visible, integrity-checked Windows updater scripts.
package main

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed VERSION script-hashes.json
var assets embed.FS
var version = embeddedVersion()
var input = bufio.NewReader(os.Stdin)
var errUpdating = errors.New("self-update handed to replacement helper")

func embeddedVersion() string {
	b, _ := assets.ReadFile("VERSION")
	return strings.TrimSpace(string(b))
}
func readLine(prompt string) (string, error) {
	fmt.Print(paint(yellow, prompt))
	s, err := input.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}
func confirm(prompt string) bool {
	for {
		s, err := readLine(prompt + " [Y/N]: ")
		if err != nil {
			return false
		}
		switch strings.ToLower(s) {
		case "y", "yes":
			return true
		case "n", "no", "":
			return false
		}
		fmt.Println("Enter Y or N.")
	}
}
func main() {
	args, opts := parseUIOptions(os.Args[1:])
	if len(args) == 1 && args[0] == "--verify-package" {
		exe, err := os.Executable()
		if err == nil {
			err = verifyScripts(filepath.Dir(exe))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(version)
		return
	}
	if len(args) > 0 && (args[0] == "--version" || args[0] == "version") {
		fmt.Println(version)
		return
	}
	if len(args) > 0 && (args[0] == "--help" || args[0] == "help" || args[0] == "-h") {
		help()
		return
	}
	if runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "This app runs on Windows x64. Source tests can run on other systems.")
		os.Exit(1)
	}
	restoreUI := configureUI(opts)
	defer restoreUI()
	startup := shouldCheckAtStartup(args, opts, interactiveConsole())
	if len(args) == 0 {
		args = []string{"menu"}
	}
	exe, err := os.Executable()
	if err == nil {
		err = showLastUpdate(filepath.Dir(exe))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		restoreUI()
		os.Exit(1)
	}
	if startup {
		err = runStartupCheck()
		if errors.Is(err, errUpdating) {
			return
		}
		if err != nil {
			uiStatus("FAIL", "Startup self-update did not complete: "+err.Error())
		}
	}
	err = dispatch(args)
	if errors.Is(err, errUpdating) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR:", err)
		restoreUI()
		os.Exit(1)
	}
}
func help() {
	fmt.Printf(`ChatGPT Update %s - portable Windows x64 utility

chatgpt-update                         Open the numbered menu
chatgpt-update check                   Check only; do not install
chatgpt-update update                  Check and ask to install ChatGPT
chatgpt-update update --no-backup       Skip the upgrade backup question
chatgpt-update update --backup          Select an upgrade backup
chatgpt-update update --exact           Require OpenAI's advertised build
chatgpt-update update --plain           Scrolling output instead of live steps
chatgpt-update backup                  Back up USERPROFILE\.codex
chatgpt-update restore                 Select a backup and confirm restoration
chatgpt-update self-update             Update the complete portable folder from a verified release ZIP
chatgpt-update self-update --yes        Approve a newer updater noninteractively
chatgpt-update path add                Add this EXE's folder to your user PATH
chatgpt-update path remove             Remove this folder from your user PATH
chatgpt-update preview                 Offline demonstration of colors, activity and download progress
chatgpt-update --version               Print only the updater version

Menu startup checks for a newer UPDATER (5 second limit) and asks before downloading.
N postpones it. Metadata errors still open the menu. Other commands do not self-check.
--skip-update-check  Open the menu without its automatic update check
--no-color          No ANSI colors (also honors NO_COLOR)
--no-animation      Keep colors, disable spinner motion
--plain             Plain scrolling output, no colors or animation
These options can appear before or after a command.

Updates require the public internet; backup/restore and preview work offline.
Extract the WHOLE portable ZIP. No MSI, Store account, PowerShell 7 or Go is needed.
Visible scripts beside the EXE use Windows PowerShell 5.1 and existing Windows policies.
No scripts are extracted into TEMP; no execution policy override is applied.
ChatGPT's MSIX comes from the third-party Wangnov/codex-app-mirror.
The updater EXE comes from Militskiy/chatgpt-update GitHub Releases.
`, version)
}
func dispatch(args []string) error {
	if args[0] == "menu" {
		return menu()
	}
	if args[0] == "preview" {
		if len(args) != 1 {
			return errors.New("preview takes no arguments")
		}
		previewActivity()
		return runScript("update-chatgpt.ps1", "-Preview")
	}
	unlock, err := acquireOperationLock()
	if err != nil {
		return err
	}
	defer unlock()
	switch args[0] {
	case "check", "update":
		ps, err := updateArgs(args)
		if err != nil {
			return err
		}
		return runScript("update-chatgpt.ps1", ps...)
	case "backup":
		if len(args) != 1 {
			return errors.New("backup takes no arguments")
		}
		return backupInteractive()
	case "restore":
		if len(args) != 1 {
			return errors.New("restore takes no arguments; choose from the list")
		}
		return restoreInteractive()
	case "self-update":
		yes := len(args) == 2 && args[1] == "--yes"
		if len(args) > 2 || (len(args) == 2 && !yes) {
			return errors.New("usage: chatgpt-update self-update [--yes]")
		}
		return selfUpdate(yes)
	case "path":
		if len(args) != 2 || (args[1] != "add" && args[1] != "remove") {
			return errors.New("usage: chatgpt-update path add|remove")
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		dir := filepath.Dir(exe)
		if !confirm(fmt.Sprintf("%s this folder in your USER PATH: %s?", strings.ToUpper(args[1]), dir)) {
			return nil
		}
		return runScript("path.ps1", "-Action", args[1], "-Directory", dir)
	default:
		return fmt.Errorf("unknown command %q; use --help", args[0])
	}
}
func updateArgs(args []string) ([]string, error) {
	var ps []string
	if args[0] == "check" {
		ps = append(ps, "-CheckOnly")
	}
	seen := map[string]bool{}
	options := map[string]string{"--no-backup": "-NoBackup", "--backup": "-Backup", "--exact": "-ExactVersionOnly", "--plain": "-PlainOutput"}
	for _, arg := range args[1:] {
		opt, ok := options[arg]
		if !ok || seen[arg] {
			return nil, fmt.Errorf("unknown or repeated option %q", arg)
		}
		seen[arg] = true
		ps = append(ps, opt)
	}
	if seen["--backup"] && seen["--no-backup"] {
		return nil, errors.New("choose --backup OR --no-backup")
	}
	return ps, nil
}
func menu() error {
	for {
		printMenu()
		answer, err := readLine("Choose a number: ")
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var cmd []string
		switch answer {
		case "0":
			return nil
		case "1":
			cmd = []string{"update"}
		case "2":
			cmd = []string{"backup"}
		case "3":
			cmd = []string{"restore"}
		case "4":
			cmd = []string{"self-update"}
		case "5":
			cmd = []string{"path", "add"}
		case "6":
			cmd = []string{"preview"}
		default:
			fmt.Println("Choose a number from 0 to 6.")
			continue
		}
		if err := dispatch(cmd); errors.Is(err, errUpdating) {
			return err
		} else if err != nil {
			fmt.Fprintln(os.Stderr, "\nERROR:", err)
		}
	}
}

// ChatGPT Update is a portable console frontend for the embedded Windows updater.
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

//go:embed VERSION scripts/*.ps1
var assets embed.FS
var version = embeddedVersion()
var input = bufio.NewReader(os.Stdin)
var errUpdating = errors.New("self-update handed to replacement helper")

func embeddedVersion() string {
	b, _ := assets.ReadFile("VERSION")
	return strings.TrimSpace(string(b))
}
func readLine(prompt string) (string, error) {
	fmt.Print(prompt)
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
	args := os.Args[1:]
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
	if len(args) == 0 {
		args = []string{"menu"}
	}
	err := dispatch(args)
	if errors.Is(err, errUpdating) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR:", err)
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
chatgpt-update self-update             Download a newer release of this EXE
chatgpt-update self-update --yes        Approve a newer updater noninteractively
chatgpt-update path add                Add this EXE's folder to your user PATH
chatgpt-update path remove             Remove this folder from your user PATH
chatgpt-update preview                 Simulated ChatGPT update progress
chatgpt-update --version               Print only the updater version

Updates require the public internet; backup/restore work offline.
No MSI, Store account, PowerShell 7 or Go installation is required.
The embedded scripts use Windows PowerShell 5.1 and existing Windows policies.
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
		fmt.Printf("\nChatGPT Update %s | portable Windows app\n", version)
		fmt.Println("------------------------------------------------")
		fmt.Println("1) Check for ChatGPT update / install")
		fmt.Println("2) Create backup")
		fmt.Println("3) Restore backup")
		fmt.Println("4) Update the updater")
		fmt.Println("5) Add this folder to user PATH")
		fmt.Println("0) Exit")
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
		default:
			fmt.Println("Choose 0, 1, 2, 3, 4 or 5.")
			continue
		}
		if err := dispatch(cmd); errors.Is(err, errUpdating) {
			return err
		} else if err != nil {
			fmt.Fprintln(os.Stderr, "\nERROR:", err)
		}
	}
}

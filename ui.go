package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Presentation only. No policy, signature, network trust or deployment settings.
type uiOptions struct {
	Plain, NoColor, NoAnimation, SkipUpdateCheck bool
}

var presentation uiOptions
var useColors, useAnimation bool

const (
	cyan   = "96"
	green  = "92"
	yellow = "93"
	red    = "91"
	muted  = "90"
	violet = "95"
)

func parseUIOptions(args []string) ([]string, uiOptions) {
	var opts uiOptions
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--plain":
			opts.Plain = true
		case "--no-color":
			opts.NoColor = true
		case "--no-animation":
			opts.NoAnimation = true
		case "--skip-update-check":
			opts.SkipUpdateCheck = true
		default:
			rest = append(rest, arg)
		}
	}
	return rest, opts
}
func enabledEnvironment(name string) bool { return os.Getenv(name) != "" && os.Getenv(name) != "0" }
func configureUI(opts uiOptions) func() {
	opts.Plain = opts.Plain || enabledEnvironment("CHATGPT_UPDATER_PLAIN")
	opts.NoColor = opts.NoColor || os.Getenv("NO_COLOR") != ""
	opts.NoAnimation = opts.NoAnimation || enabledEnvironment("CHATGPT_UPDATER_NO_ANIMATION")
	presentation = opts
	useColors, useAnimation = false, false
	if opts.Plain || os.Getenv("TERM") == "dumb" || enabledEnvironment("CI") {
		return func() {}
	}
	supported, restore := enableTerminalOutput()
	useColors = supported && !opts.NoColor
	useAnimation = supported && interactiveConsole() && !opts.NoAnimation
	return restore
}
func colored(enabled bool, code, text string) string {
	if !enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}
func paint(code, text string) string { return colored(useColors, code, text) }
func uiStatus(kind, message string) {
	code := cyan
	switch kind {
	case "OK":
		code = green
	case "WARN", "SKIP":
		code = yellow
	case "FAIL":
		code = red
	}
	fmt.Printf("%s %s\n", paint(code, "["+kind+"]"), message)
}

// Do not let control characters from a remote response manipulate the terminal.
func oneLine(text string, width int) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	chars := []rune(text)
	if width < 8 {
		width = 8
	}
	if len(chars) > width {
		return string(chars[:width-3]) + "..."
	}
	return text
}

func printMenu() {
	fmt.Println()
	fmt.Println(paint(cyan, "ChatGPT Update "+version) + paint(muted, " | portable Windows app"))
	fmt.Println(paint(muted, strings.Repeat("-", 58)))
	fmt.Println(paint(muted, "Tip: 1 updates ChatGPT; 4 updates this utility."))
	fmt.Println(paint(muted, "Self-update: auto-close on success; pause on error."))
	fmt.Println(paint(cyan, "  APP"))
	fmt.Println("  " + paint(cyan, "1)") + " Check for ChatGPT update / install")
	fmt.Println(paint(violet, "  LOCAL DATA"))
	fmt.Println("  " + paint(violet, "2)") + " Create backup")
	fmt.Println("  " + paint(violet, "3)") + " Restore backup")
	fmt.Println(paint(cyan, "  UPDATER"))
	fmt.Println("  " + paint(cyan, "4)") + " Update the updater")
	fmt.Println("  " + paint(cyan, "5)") + " Add this folder to user PATH")
	fmt.Println("  " + paint(cyan, "6)") + " Preview colors and progress (offline)")
	fmt.Println("  " + paint(muted, "0)") + " Exit")
	fmt.Println(paint(muted, strings.Repeat("-", 58)))
}

// The caller must finish the activity before prompting or printing another
// operation's output. Stop is idempotent and joins the rendering goroutine.
// No cursor hiding, raw input modes or full-screen clearing are used.
func newActivity(w io.Writer, label string, animate, colors bool, columns func() int) func(string) {
	label = oneLine(label, 100)
	if !animate {
		fmt.Fprintf(w, "%s %s\n", colored(colors, cyan, "[>>]"), label)
		return func(string) {}
	}
	started := time.Now()
	stop, finished := make(chan struct{}), make(chan struct{})
	var once sync.Once
	go func() {
		defer close(finished)
		tick := time.NewTicker(120 * time.Millisecond)
		defer tick.Stop()
		frames := []string{"[|]", "[/]", "[-]", "[\\]"}
		n := 0
		draw := func() {
			width := columns()
			if width < 25 {
				return
			}
			line := fmt.Sprintf("%s %s  %.1fs", frames[n%len(frames)], label, time.Since(started).Seconds())
			fmt.Fprint(w, "\r\x1b[2K", colored(colors, cyan, oneLine(line, width-2)))
			n++
		}
		draw()
		for {
			select {
			case <-stop:
				fmt.Fprint(w, "\r\x1b[2K")
				return
			case <-tick.C:
				draw()
			}
		}
	}()
	return func(kind string) {
		once.Do(func() {
			close(stop)
			<-finished
			code := green
			if kind == "FAIL" {
				code = red
			} else if kind == "WARN" {
				code = yellow
			}
			fmt.Fprintf(w, "%s %s\n", colored(colors, code, "["+kind+"]"), label)
		})
	}
}
func startActivity(label string) func(string) {
	return newActivity(os.Stdout, label, useAnimation, useColors, terminalColumns)
}
func activityResult(stop func(string), err error) {
	if err != nil {
		stop("FAIL")
	} else {
		stop("OK")
	}
}

// Forward presentation preferences only. Other child environment values and
// the v0.2.2 PowerShell-module-path compatibility fix remain unchanged.
func uiEnvironment(environment []string) []string {
	changes := map[string]string{}
	if presentation.Plain {
		changes["CHATGPT_UPDATER_PLAIN"] = "1"
	}
	if presentation.NoColor {
		changes["NO_COLOR"] = "1"
	}
	if presentation.NoAnimation {
		changes["CHATGPT_UPDATER_NO_ANIMATION"] = "1"
	}
	result := make([]string, 0, len(environment)+len(changes))
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if _, replacing := changes[strings.ToUpper(key)]; !replacing {
			result = append(result, entry)
		}
	}
	for _, key := range []string{"CHATGPT_UPDATER_PLAIN", "NO_COLOR", "CHATGPT_UPDATER_NO_ANIMATION"} {
		if value, ok := changes[key]; ok {
			result = append(result, key+"="+value)
		}
	}
	return result
}

// A single percentage is based on real bytes. 100% is only drawn after the
// downloader has checked both final size and SHA-256. Elapsed time drives ETA.
func formatDownload(received, total int64, elapsed time.Duration, completed bool) string {
	if received < 0 {
		received = 0
	}
	if total <= 0 {
		return fmt.Sprintf("[>>] Downloaded %.1f MiB (total unknown)", float64(received)/(1<<20))
	}
	ratio := float64(received) / float64(total)
	if ratio > 0.999 && !completed {
		ratio = 0.999
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * 24)
	rate, eta := "--", "--:--"
	if elapsed > 0 && received > 0 {
		speed := float64(received) / elapsed.Seconds()
		rate = fmt.Sprintf("%.1f", speed/(1<<20))
		remaining := float64(total-received) / speed
		if remaining < 0 {
			remaining = 0
		}
		seconds := int(remaining)
		eta = fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
	}
	return fmt.Sprintf("[%s%s] %5.1f%% | %.1f/%.1f MiB | %s MiB/s | ETA %s",
		strings.Repeat("#", filled), strings.Repeat("-", 24-filled), ratio*100,
		float64(received)/(1<<20), float64(total)/(1<<20), rate, eta)
}
func previewActivity() {
	fmt.Println(paint(yellow, "PREVIEW ONLY - simulated activity, no network or installation."))
	printMenu()
	stop := startActivity("Example activity indicator (simulation)")
	time.Sleep(1200 * time.Millisecond)
	stop("OK")
}

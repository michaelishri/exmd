// Command exmd is the reference runner of the exmd specification
// (SPEC.md). Execution lives in the TUI; headless verbs read and validate
// documents without ever running blocks.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/michaelishri/exmd/internal/config"
	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/render"
	"github.com/michaelishri/exmd/internal/termsafe"
	"github.com/michaelishri/exmd/internal/theme"
)

// version is the application version. Release builds stamp it with
// -ldflags "-X main.version=..." (docs/releasing.md), so a binary always
// reports the tag it was built from.
var version = "0.7.0"

const usage = `exmd — executable Markdown (runbooks)

Usage:
  exmd DOC [--resume] [--set K=V] [--theme NAME] [--no-session]
                                    open a runbook in the TUI (backtick selects blocks, enter runs, r reloads, c copies output, esc stops, , settings);
                                    --resume restores the last saved session for DOC; --no-session saves nothing
  exmd --resume --list               list saved sessions
  exmd --resume --prune              remove sessions that can no longer be resumed
  exmd DOC --resume --clear          discard a saved session
  exmd check DOC                 validate; report all diagnostics at once
  exmd list DOC                  enumerate executable blocks
  exmd globals DOC [--set K=V]   show the document's global variables
  exmd print DOC [--width N] [--theme NAME]
                                    render the document to the terminal
  exmd version                   print the version

Launch-time overrides (--set KEY=VALUE) may target any global declared in the
frontmatter; the frontmatter is the complete contract of the document's
variable surface (SPEC.md §2.2). --theme selects the palette for the TUI and
print. auto (the default) is dark when the terminal supports color and notty
otherwise. Other palettes:
  catppuccin-mocha, catppuccin-latte, one-dark, one-light, gruvbox-dark,
  gruvbox-light, dracula, tokyo-night, github-dark, github-light,
  solarized-dark, solarized-light, nord, nord-light, rose-pine,
  rose-pine-dawn, everforest-light, ayu-dark, ayu-light, dark, notty
Aliases: catppuccin, atom, atom-light, tokyonight, rosepine.

The TUI saves one session per document once you run a block. Pass --resume to
restore its globals, block outputs, and selected block; nothing about the spec's
state model changes (SPEC.md §9). Sessions can hold secrets from globals and
output: --no-session (or save_sessions: false in the config) turns saving off.
Edited executable blocks start fresh and later outputs are marked stale. Press r
in the TUI to reload an edited document.

Exit codes: 0 success · 1 execution failed · 2 spec/usage error (SPEC.md §6.2)`

// errHelp marks a -h/--help request so every verb can answer it the same
// way: usage on stdout, exit 0.
var errHelp = errors.New("help")

// OpenCode-adjacent minimal styling: color only where it carries meaning.
// The colors come from the selected theme, never from the terminal's ANSI
// palette.
var (
	diagnosticPalette = diagnosticTheme()
	styleErr          lipgloss.Style
	styleWarn         lipgloss.Style
	styleDim          lipgloss.Style
)

func init() { setDiagnosticTheme(diagnosticPalette) }

// diagnosticTheme supplies the default diagnostics palette. On a terminal
// without color support every token is empty, so styles render as plain text.
func diagnosticTheme() theme.Theme {
	if !colorSupported() {
		return theme.Notty
	}
	return theme.Dark
}

// setDiagnosticTheme points diagnostics at the palette selected for this
// invocation, so headless output agrees with the TUI and print.
func setDiagnosticTheme(th theme.Theme) {
	diagnosticPalette = th
	styleErr = lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Error)))
	styleWarn = lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Warning)))
	styleDim = lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.TextMuted)))
}

// printThemeLine writes a line with the same canvas rule as the TUI and
// print, so headless output never falls back to the terminal's palette.
func printThemeLine(s string) {
	fmt.Fprintln(stdout, render.PaintCanvas(s, lipgloss.Width(s), diagnosticPalette))
}

// stdout and stderr reduce styled output to what the terminal can show:
// true color passes through, 256- and 16-color terminals get the nearest
// colors, and a pipe or NO_COLOR gets plain text. They are bound to the
// process's streams at the start of every run.
var stdout, stderr io.Writer = os.Stdout, os.Stderr

func bindOutput() {
	stdout = colorprofile.NewWriter(os.Stdout, os.Environ())
	stderr = colorprofile.NewWriter(os.Stderr, os.Environ())
}

func main() {
	os.Exit(run(os.Args[1:]))
}

// colorSupported reports whether the terminal can display color.
func colorSupported() bool {
	return colorprofile.Detect(os.Stdout, os.Environ()) > colorprofile.ASCII
}

// parseFlag extracts every occurrence of a flag in any position, accepting
// --name VALUE, --name=VALUE and their single-dash forms. Repeated flags
// append to values; callers decide which one wins.
func parseFlag(args []string, long, short string) (values, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--"+long || a == "-"+short:
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--%s requires a value", long)
			}
			i++
			values = append(values, args[i])
		case strings.HasPrefix(a, "--"+long+"="):
			values = append(values, strings.TrimPrefix(a, "--"+long+"="))
		case strings.HasPrefix(a, "-"+short+"="):
			values = append(values, strings.TrimPrefix(a, "-"+short+"="))
		default:
			rest = append(rest, a)
		}
	}
	return values, rest, nil
}

// extractHelp removes -h/--help from args and reports whether it was present.
func extractHelp(args []string) (rest []string, help bool) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			help = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, help
}

// parseTheme extracts --theme NAME from args in any position. A repeated
// flag wins; the empty result means "auto".
func parseTheme(args []string) (string, []string, error) {
	values, rest, err := parseFlag(args, "theme", "theme")
	if err != nil {
		return "", nil, err
	}
	name := ""
	if len(values) > 0 {
		name = values[len(values)-1]
	}
	return name, rest, nil
}

// resolveTheme turns a parsed --theme value into a Theme for the terminal.
func resolveTheme(name string) (theme.Theme, error) {
	return theme.Resolve(name, colorSupported())
}

// resolveInvocationTheme applies theme precedence: an explicit --theme flag
// wins, then the saved preference, then the default. A bad saved name is
// reported and ignored rather than failing the command.
func resolveInvocationTheme(flagName string) (theme.Theme, error) {
	if flagName != "" {
		return resolveTheme(flagName)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
	}
	if cfg.Theme != "" {
		if th, err := theme.Resolve(cfg.Theme, colorSupported()); err == nil {
			return th, nil
		} else {
			fmt.Fprintf(os.Stderr, "exmd: ignoring saved theme %q: %v\n", cfg.Theme, err)
		}
	}
	return resolveTheme("")
}

// hasResumeFlag reports whether --resume is present anywhere in args. Flags
// may appear before or after the document, so dispatch cannot look at args[0].
func hasResumeFlag(args []string) bool {
	for _, a := range args {
		if a == "--resume" || a == "-resume" || strings.HasPrefix(a, "--resume=") || strings.HasPrefix(a, "-resume=") {
			return true
		}
	}
	return false
}

func run(args []string) int {
	// Verbs without --theme use the default diagnostics palette. Commands
	// that accept --theme override it after parsing their flags.
	bindOutput()
	setDiagnosticTheme(diagnosticTheme())
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	// --resume belongs to the viewer: `exmd check DOC --resume` stays a
	// check (and reports the stray flag) rather than becoming a resume.
	if !isVerb(args[0]) && hasResumeFlag(args) {
		return cmdResume(args)
	}
	switch args[0] {
	case "check":
		return cmdCheck(args[1:])
	case "list":
		return cmdList(args[1:])
	case "globals":
		return cmdGlobals(args[1:])
	case "print":
		return cmdPrint(args[1:])
	case "--help", "-h", "help":
		fmt.Println(usage)
		return 0
	case "--version", "-v", "version":
		fmt.Printf("exmd version %s (spec v1)\n", version)
		return 0
	default:
		if strings.HasPrefix(args[0], "-") && !isViewerFlag(args[0]) {
			fmt.Fprintf(os.Stderr, "unknown flag or command %q\n\n", args[0])
			fmt.Fprintln(os.Stderr, usage)
			return 2
		}
		return cmdTUI(args)
	}
}

// isViewerFlag reports whether arg is a viewer flag, which may come before
// the document like every other flag.
func isViewerFlag(arg string) bool {
	name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
	switch name {
	case "set", "theme", "no-session":
		return true
	}
	return false
}

// isVerb reports whether arg names a headless command or a help/version
// request rather than a document.
func isVerb(arg string) bool {
	switch arg {
	case "check", "list", "globals", "print", "help", "--help", "-h", "version", "--version", "-v":
		return true
	}
	return false
}

// loadDoc is the shared load step for CLI verbs: I/O errors exit 2, all
// document diagnostics are collected and reported together (SPEC.md §2.3).
func loadDoc(path string) (*doc.Result, int) {
	res, err := doc.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return nil, 2
	}
	return res, 0
}

// printDiagnostics reports every diagnostic in compiler style:
//
//	path:line: error: block N: message
func printDiagnostics(res *doc.Result) {
	path := styleDim.Render(termsafe.Visible(res.Doc.Path))
	for _, d := range res.Diagnostics {
		sev := styleErr.Render("error")
		if d.Severity == doc.SeverityWarning {
			sev = styleWarn.Render("warning")
		}
		msg := termsafe.Visible(d.Detail())
		if d.Line > 0 {
			printThemeLine(fmt.Sprintf("%s:%d: %s: %s", path, d.Line, sev, msg))
		} else {
			printThemeLine(fmt.Sprintf("%s: %s: %s", path, sev, msg))
		}
	}
}

// printTable prints tab-aligned rows with a header.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

func printSummary(errs, warns int) {
	if errs == 0 && warns == 0 {
		return
	}
	printThemeLine(styleDim.Render(fmt.Sprintf("%d error(s), %d warning(s)", errs, warns)))
}

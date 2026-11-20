// Command exmd is the reference runner of the exmd specification
// (SPEC.md). Execution lives in the TUI; headless verbs read and validate
// documents without ever running blocks.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/theme"
	"github.com/muesli/termenv"
)

const version = "0.3.0"

const usage = `exmd — executable Markdown (runbooks)

Usage:
  exmd DOC [--set K=V] [--theme auto|dark|notty]
                                    open a runbook in the TUI (backtick selects blocks, r runs, c copies output, s stops)
  exmd check DOC                 validate; report all diagnostics at once
  exmd list DOC                  enumerate executable blocks
  exmd globals DOC [--set K=V]   show the document's global variables
  exmd print DOC [--width N] [--theme auto|dark|notty]
                                    render the document to the terminal
  exmd version                   print the version

Launch-time overrides (--set KEY=VALUE) may target any global declared in the
frontmatter; the frontmatter is the complete contract of the document's
variable surface (SPEC.md §2.2). --theme selects the palette for the TUI and
print: auto (the default) is dark when the terminal supports color and notty
otherwise; dark and notty force their palette.

Exit codes: 0 success · 1 execution failed · 2 spec/usage error (SPEC.md §6.2)`

// errHelp marks a -h/--help request so every verb can answer it the same
// way: usage on stdout, exit 0.
var errHelp = errors.New("help")

// OpenCode-adjacent minimal styling: color only where it carries meaning.
var (
	styleErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// colorSupported reports whether the terminal can display color.
func colorSupported() bool {
	return lipgloss.ColorProfile() != termenv.Ascii
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

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
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
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprintf(os.Stderr, "unknown flag or command %q\n\n", args[0])
			fmt.Fprintln(os.Stderr, usage)
			return 2
		}
		return cmdTUI(args)
	}
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
	path := styleDim.Render(res.Doc.Path)
	for _, d := range res.Diagnostics {
		sev := styleErr.Render("error")
		if d.Severity == doc.SeverityWarning {
			sev = styleWarn.Render("warning")
		}
		msg := d.Detail()
		if d.Line > 0 {
			fmt.Printf("%s:%d: %s: %s\n", path, d.Line, sev, msg)
		} else {
			fmt.Printf("%s: %s: %s\n", path, sev, msg)
		}
	}
}

// printTable prints tab-aligned rows with a header.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
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
	fmt.Println(styleDim.Render(fmt.Sprintf("%d error(s), %d warning(s)", errs, warns)))
}

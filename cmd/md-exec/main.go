// Command md-exec is the reference runner of the md-exec specification
// (SPEC.md). Execution lives in the TUI; headless verbs read and validate
// documents without ever running blocks.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/michaelishri/md-exec/internal/doc"
)

const version = "0.1.0-dev"

const usage = `md-exec — executable Markdown (runbooks)

Usage:
  md-exec DOC [--set K=V]           open a runbook in the TUI (r runs, n steps, s stops)
  md-exec check DOC                 validate; report all diagnostics at once
  md-exec list DOC                  enumerate executable blocks
  md-exec globals DOC [--set K=V]   show the document's global variables
  md-exec print DOC                 render the document to the terminal
  md-exec version                   print the version

Launch-time overrides (--set KEY=VALUE) may target any global declared in the
frontmatter; the frontmatter is the complete contract of the document's
variable surface (SPEC.md §2.2).

Exit codes: 0 success · 1 execution failed · 2 spec/usage error (SPEC.md §6.2)`

// OpenCode-adjacent minimal styling: color only where it carries meaning.
var (
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
)

func main() {
	os.Exit(run(os.Args[1:]))
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
	case "tui":
		return cmdTUI(args[1:])
	case "--help", "-h", "help":
		fmt.Println(usage)
		return 0
	case "--version", "-v", "version":
		fmt.Printf("md-exec version %s (spec v1)\n", version)
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
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return nil, 2
	}
	return res, 0
}

// printDiagnostics reports every diagnostic in compiler style:
//
//	path:line: error: block N: message
func printDiagnostics(res *doc.Result) {
	path := res.Doc.Path
	for _, d := range res.Diagnostics {
		sev := styleErr.Render("error")
		if d.Severity == doc.SeverityWarning {
			sev = styleWarn.Render("warning")
		}
		msg := d.Message
		if d.Block > 0 {
			msg = fmt.Sprintf("block %d: %s", d.Block, msg)
		}
		if d.Line > 0 {
			fmt.Printf("%s:%d: %s: %s\n", styleDim.Render(path), d.Line, sev, msg)
		} else {
			fmt.Printf("%s: %s: %s\n", styleDim.Render(path), sev, msg)
		}
	}
}

func printSummary(errs, warns int) {
	if errs == 0 && warns == 0 {
		return
	}
	fmt.Println(styleDim.Render(fmt.Sprintf("%d error(s), %d warning(s)", errs, warns)))
}

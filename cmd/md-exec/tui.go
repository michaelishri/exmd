package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/michaelishri/md-exec/internal/doc"
	"github.com/michaelishri/md-exec/internal/tui"
)

// cmdTUI opens a document in the viewer. Execution lives here; headless
// verbs never run blocks (SPEC.md §6: the interface is the tool's choice).
func cmdTUI(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: md-exec DOC")
		return 2
	}
	res, err := doc.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return 2
	}
	res.Sort()
	if res.HasErrors() {
		printDiagnostics(res)
		fmt.Fprintln(os.Stderr, styleDim.Render("fix the issues above; `md-exec check` reports them all at once"))
		return 2
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintf(os.Stderr, "md-exec: the TUI needs an interactive terminal; try `md-exec check %s`\n", args[0])
		return 2
	}

	p := tea.NewProgram(tui.New(res), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return 2
	}
	return 0
}

package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/michaelishri/md-exec/internal/tui"
)

// cmdTUI opens a document in the viewer. Execution lives here; headless
// verbs never run blocks (SPEC.md §6: the interface is the tool's choice).
// Launch-time overrides arrive as --set KEY=VALUE in any position; --theme
// selects the palette.
func cmdTUI(args []string) int {
	themeName, args, err := parseTheme(args)
	if errors.Is(err, errHelp) {
		fmt.Println(usage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return 2
	}
	sets, positional, err := parseSetArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Println(usage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "usage: md-exec DOC [--set KEY=VALUE]... [--theme auto|dark|notty]")
		return 2
	}

	th, err := resolveTheme(themeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return 2
	}
	tui.SetTheme(th)

	res, code := loadDoc(positional[0])
	if res == nil {
		return code
	}
	res.Sort()
	printDiagnostics(res)
	if res.HasErrors() {
		fmt.Fprintln(os.Stderr, styleDim.Render("fix the issues above; `md-exec check` reports them all at once"))
		return 2
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintf(os.Stderr, "md-exec: the TUI needs an interactive terminal; try `md-exec check %s`\n", positional[0])
		return 2
	}

	p := tea.NewProgram(tui.New(res, sets.toMap()), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return 2
	}
	return 0
}

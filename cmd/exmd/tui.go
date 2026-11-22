package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/session"
	"github.com/michaelishri/exmd/internal/tui"
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
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	sets, positional, err := parseSetArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Println(usage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "usage: exmd DOC [--set KEY=VALUE]... [--theme auto|dark|notty]")
		return 2
	}

	res, code := loadDoc(positional[0])
	if res == nil {
		return code
	}
	res.Sort()
	printDiagnostics(res)
	if res.HasErrors() {
		fmt.Fprintln(os.Stderr, styleDim.Render("fix the issues above; `exmd check` reports them all at once"))
		return 2
	}
	return launchTUI(res, sets.toMap(), themeName, nil)
}

// launchTUI is shared by fresh opens and resumes. A non-nil snapshot restores
// the saved viewer state; the underlying exec.Session is always created fresh.
func launchTUI(res *doc.Result, overrides map[string]string, themeName string, snap *session.Snapshot) int {
	th, err := resolveTheme(themeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	tui.SetTheme(th)

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintf(os.Stderr, "exmd: the TUI needs an interactive terminal; try `exmd check %s`\n", res.Doc.Path)
		return 2
	}

	var model tea.Model = tui.New(res, overrides)
	if snap != nil {
		model = tui.NewWithSnapshot(res, overrides, snap)
	}
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	if snap == nil {
		fmt.Printf("Session saved — resume with: exmd %s --resume\n", res.Doc.Path)
	}
	return 0
}

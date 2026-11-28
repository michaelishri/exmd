package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"

	"github.com/michaelishri/exmd/internal/config"
	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/session"
	"github.com/michaelishri/exmd/internal/termsafe"
	"github.com/michaelishri/exmd/internal/theme"
	"github.com/michaelishri/exmd/internal/tui"
)

// cmdTUI opens a document in the viewer. Execution lives here; headless
// verbs never run blocks (SPEC.md §6: the interface is the tool's choice).
// Launch-time overrides arrive as --set KEY=VALUE in any position; --theme
// selects the palette; --no-session turns session saving off.
func cmdTUI(args []string) int {
	args, noSession := extractBool(args, "--no-session")
	themeName, args, err := parseTheme(args)
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
		fmt.Fprintln(os.Stderr, "usage: exmd DOC [--set KEY=VALUE]... [--theme NAME] [--no-session]")
		return 2
	}

	if _, code := applyInvocationTheme(themeName); code != 0 {
		return code
	}
	res, code := loadForViewer(positional[0])
	if res == nil {
		return code
	}
	return launchTUI(res, sets.toMap(), nil, !noSession)
}

// applyInvocationTheme resolves the theme once per invocation and points the
// viewer and the diagnostics at it.
func applyInvocationTheme(name string) (theme.Theme, int) {
	th, err := resolveInvocationTheme(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return th, 2
	}
	tui.SetTheme(th)
	setDiagnosticTheme(th)
	return th, 0
}

// loadForViewer loads a document and runs pre-flight before anything can
// execute (SPEC.md §5.3). A document with errors is refused with all of
// them reported at once.
func loadForViewer(path string) (*doc.Result, int) {
	res, code := loadDoc(path)
	if res == nil {
		return nil, code
	}
	res.Preflight()
	res.Sort()
	printDiagnostics(res)
	if res.HasErrors() {
		fmt.Fprintln(stderr, styleDim.Render("fix the issues above; `exmd check` reports them all at once"))
		return nil, 2
	}
	return res, 0
}

// launchTUI is shared by fresh opens and resumes. A non-nil snapshot restores
// the saved viewer state; the underlying exec.Session is always created fresh.
// saveSessions is false for --no-session; the save_sessions preference can
// also turn saving off.
func launchTUI(res *doc.Result, overrides map[string]string, snap *session.Snapshot, saveSessions bool) int {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintf(os.Stderr, "exmd: the TUI needs an interactive terminal; try `exmd check %s`\n", termsafe.Visible(res.Doc.Path))
		return 2
	}
	if cfg, err := config.Load(); err == nil && !cfg.SessionsEnabled() {
		saveSessions = false
	}

	var model tui.Model
	if snap != nil {
		model = tui.NewWithSnapshot(res, overrides, snap)
	} else {
		model = tui.New(res, overrides)
		if saveSessions {
			if old, err := session.Load(res.Doc.Path); err == nil && old != nil {
				model = model.WithNotice("a saved session exists — quit and reopen with --resume to restore it; running a block replaces it")
			}
		}
	}
	if !saveSessions {
		model = model.WithoutSessionSaving()
	}
	final, err := tea.NewProgram(model).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	if m, ok := final.(tui.Model); ok && m.SessionSaved() && snap == nil {
		fmt.Printf("Session saved — resume with: exmd %s --resume\n", termsafe.Visible(res.Doc.Path))
	}
	return 0
}

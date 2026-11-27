package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/michaelishri/exmd/internal/session"
	"github.com/michaelishri/exmd/internal/termsafe"
)

const resumeUsage = `exmd DOC --resume [--set KEY=VALUE]... [--theme auto|dark|notty]
                     reopen the last saved session for DOC (--resume may precede DOC)
exmd --resume --list list saved sessions
exmd DOC --resume --clear
                     discard a saved session

A resume replays saved globals and block outputs into a fresh run. --set
overrides must match the saved ones. Edited executable blocks start fresh and
later outputs are marked stale; a v1 snapshot must be cleared with
\"exmd DOC --resume --clear\".`

// extractBool removes a stand-alone boolean flag from args and reports
// whether it appeared. Flags may appear in any position.
func extractBool(args []string, flag string) (rest []string, found bool) {
	for _, a := range args {
		if a == flag {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, found
}

// extractResume removes --resume (or -resume) and reports its optional value,
// so flags may appear before or after the document.
func extractResume(args []string) (rest []string, value string, found bool) {
	for _, a := range args {
		switch {
		case a == "--resume" || a == "-resume":
			found = true
		case strings.HasPrefix(a, "--resume="):
			found = true
			value = strings.TrimPrefix(a, "--resume=")
		case strings.HasPrefix(a, "-resume="):
			found = true
			value = strings.TrimPrefix(a, "-resume=")
		default:
			rest = append(rest, a)
		}
	}
	if value != "" {
		rest = append(rest, value)
	}
	return rest, value, found
}

// cmdResume reopens a saved viewer session, lists snapshots, or clears one.
// A resume never reattaches to a process: it replays legal §4 state into a
// fresh run (SPEC.md §9).
func cmdResume(args []string) int {
	args, _, _ = extractResume(args)
	args, help := extractHelp(args)
	if help {
		fmt.Println(resumeUsage)
		return 0
	}
	args, doClear := extractBool(args, "--clear")
	args, doList := extractBool(args, "--list")
	themeName, args, err := parseTheme(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	sets, positional, err := parseSetArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Println(resumeUsage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}

	if doList {
		if doClear || len(positional) != 0 {
			fmt.Fprintln(os.Stderr, resumeUsage)
			return 2
		}
		snaps, err := session.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
			return 2
		}
		if len(snaps) == 0 {
			fmt.Println("no saved sessions")
			return 0
		}
		rows := make([][]string, 0, len(snaps))
		for _, s := range snaps {
			rows = append(rows, []string{
				termsafe.Visible(s.DocPath),
				s.UpdatedAt.Local().Format("2006-01-02 15:04"),
				fmt.Sprintf("%d", len(s.Blocks)),
			})
		}
		printTable([]string{"DOCUMENT", "SAVED", "BLOCKS"}, rows)
		return 0
	}

	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, resumeUsage)
		return 2
	}
	path := positional[0]
	if doClear {
		if err := session.Delete(path); err != nil {
			fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
			return 2
		}
		fmt.Printf("Session cleared for %s\n", path)
		return 0
	}

	snap, err := session.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	if snap == nil {
		fmt.Fprintf(os.Stderr, "exmd: no saved session for %s; start one with `exmd %s`\n", path, path)
		return 2
	}

	res, code := loadDoc(path)
	if res == nil {
		return code
	}
	// Pre-flight runs before anything can execute (SPEC.md §5.3).
	res.Preflight()
	res.Sort()
	printDiagnostics(res)
	if res.HasErrors() {
		fmt.Fprintln(os.Stderr, styleDim.Render("fix the issues above; `exmd check` reports them all at once"))
		return 2
	}
	if err := snap.Validate(res.Doc, sets.toMap()); err != nil {
		fmt.Fprintf(os.Stderr, "exmd: cannot resume %s: %v\n", path, err)
		return 2
	}
	return launchTUI(res, sets.toMap(), themeName, snap)
}

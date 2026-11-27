package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/michaelishri/exmd/internal/session"
	"github.com/michaelishri/exmd/internal/termsafe"
)

const resumeUsage = `exmd DOC --resume [--set KEY=VALUE]... [--theme NAME] [--no-session]
                     reopen the last saved session for DOC (--resume may precede DOC)
exmd --resume --list list saved sessions
exmd --resume --prune
                     remove sessions that can no longer be resumed
exmd DOC --resume --clear
                     discard a saved session

A resume replays saved globals and block outputs into a fresh run. --set
overrides must match the saved ones. Edited executable blocks start fresh and
later outputs are marked stale; a v1 snapshot must be cleared with
"exmd DOC --resume --clear".`

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
	args, doPrune := extractBool(args, "--prune")
	args, noSession := extractBool(args, "--no-session")
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

	if doList || doPrune {
		if doList == doPrune || doClear || len(positional) != 0 {
			fmt.Fprintln(os.Stderr, resumeUsage)
			return 2
		}
		if doPrune {
			return pruneSessions()
		}
		return listSessions()
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

	if _, code := applyInvocationTheme(themeName); code != 0 {
		return code
	}
	res, code := loadForViewer(path)
	if res == nil {
		return code
	}
	if err := snap.Validate(res.Doc, sets.toMap()); err != nil {
		fmt.Fprintf(os.Stderr, "exmd: cannot resume %s: %v\n", path, err)
		return 2
	}
	return launchTUI(res, sets.toMap(), snap, !noSession)
}

// listSessions prints the saved sessions. Files that cannot be read are
// reported on stderr with their path instead of hiding the others.
func listSessions() int {
	entries, err := session.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	if len(entries) == 0 {
		fmt.Println("no saved sessions")
		return 0
	}
	var rows [][]string
	for _, e := range entries {
		if e.Err != nil {
			fmt.Fprintf(os.Stderr, "exmd: skipping %s: %v\n", termsafe.Visible(e.File), termsafe.Visible(e.Err.Error()))
			continue
		}
		s := e.Snapshot
		docPath := termsafe.Visible(s.DocPath)
		if _, err := os.Stat(s.DocPath); err != nil {
			docPath += " (missing)"
		}
		rows = append(rows, []string{
			docPath,
			s.UpdatedAt.Local().Format("2006-01-02 15:04"),
			fmt.Sprintf("%d", len(s.Blocks)),
		})
	}
	if len(rows) > 0 {
		printTable([]string{"DOCUMENT", "SAVED", "BLOCKS"}, rows)
	}
	if len(rows) < len(entries) {
		fmt.Fprintln(os.Stderr, "exmd: remove unreadable sessions with `exmd --resume --prune`")
	}
	return 0
}

// pruneSessions removes snapshots that are unreadable, unsupported, or whose
// document no longer exists.
func pruneSessions() int {
	removed, err := session.Prune()
	for _, e := range removed {
		what := termsafe.Visible(e.File)
		if e.Snapshot != nil {
			what = termsafe.Visible(e.Snapshot.DocPath) + " (document missing)"
		}
		fmt.Printf("removed %s\n", what)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd: %v\n", err)
		return 2
	}
	if len(removed) == 0 {
		fmt.Println("nothing to prune")
	}
	return 0
}

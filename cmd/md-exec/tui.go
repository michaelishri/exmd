package main

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/michaelishri/md-exec/internal/doc"
)

// cmdTUI opens a document in the TUI. Execution lives here (the headless
// verbs never run blocks). The viewer lands in the next phase; this stub
// still validates the document so `md-exec DOC` is useful today.
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
		return 2
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintf(os.Stderr, "md-exec: the TUI needs an interactive terminal; try `md-exec check %s`\n", args[0])
		return 2
	}
	fmt.Fprintln(os.Stderr, "md-exec: the TUI viewer is next on the roadmap; try `md-exec check` or `md-exec list` meanwhile")
	return 2
}

package main

import (
	"fmt"
	"os"
)

// cmdPrint renders the document to the terminal. It lands together with the
// shared renderer (TUI phase).
func cmdPrint(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: md-exec print DOC")
		return 2
	}
	res, code := loadDoc(args[0])
	if res == nil {
		return code
	}
	res.Sort()
	printDiagnostics(res)
	if res.HasErrors() {
		return 2
	}
	fmt.Fprintln(os.Stderr, "md-exec print: not implemented yet (ships with the TUI renderer)")
	return 2
}

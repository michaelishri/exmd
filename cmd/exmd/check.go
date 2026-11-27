package main

import (
	"fmt"
	"os"
)

// cmdCheck validates a document: load-time diagnostics plus the
// environment-dependent pre-flight checks (SPEC.md §2.3). All diagnostics
// are reported together; any error exits 2 with zero side effects.
func cmdCheck(args []string) int {
	args, help := extractHelp(args)
	if help {
		fmt.Println("usage: exmd check DOC")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: exmd check DOC")
		return 2
	}
	res, code := loadDoc(args[0])
	if res == nil {
		return code
	}
	res.Preflight()
	res.Sort()
	printDiagnostics(res)

	errs, warns := res.Counts()
	printSummary(errs, warns)
	if errs > 0 {
		return 2
	}
	if warns == 0 {
		printThemeLine(styleDim.Render(fmt.Sprintf("no issues · %d executable block(s) · %d global(s)",
			len(res.Doc.Blocks), len(res.Doc.Globals))))
	}
	return 0
}

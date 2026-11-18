package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/michaelishri/md-exec/internal/doc"
)

// cmdList enumerates the document's executable blocks (SPEC.md §3).
func cmdList(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: md-exec list DOC")
		return 2
	}
	res, code := loadDoc(args[0])
	if res == nil {
		return code
	}
	res.Sort()
	printDiagnostics(res)

	if len(res.Doc.Blocks) == 0 {
		if !res.HasErrors() {
			fmt.Println(styleDim.Render("no executable blocks"))
		}
		if res.HasErrors() {
			return 2
		}
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLANG\tKIND\tLINE\tSOURCE")
	for _, b := range res.Doc.Blocks {
		src := "(embedded)"
		if b.Kind == doc.KindScript {
			src = b.ScriptPath
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\n", b.Index, b.Lang, b.Kind, b.Line, src)
	}
	w.Flush()

	errs, warns := res.Counts()
	printSummary(errs, warns)
	if errs > 0 {
		return 2
	}
	return 0
}

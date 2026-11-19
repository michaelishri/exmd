package main

import (
	"fmt"
	"os"

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

	rows := make([][]string, 0, len(res.Doc.Blocks))
	for _, b := range res.Doc.Blocks {
		src := "(embedded)"
		if b.Kind == doc.KindScript {
			src = b.ScriptPath
		}
		rows = append(rows, []string{
			fmt.Sprint(b.Index), b.Lang, b.Kind.String(), fmt.Sprint(b.Line), src,
		})
	}
	printTable([]string{"ID", "LANG", "KIND", "LINE", "SOURCE"}, rows)

	errs, warns := res.Counts()
	printSummary(errs, warns)
	if errs > 0 {
		return 2
	}
	return 0
}

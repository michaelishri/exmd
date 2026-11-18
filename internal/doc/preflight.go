package doc

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
)

// KnownLanguages maps fence languages to the interpreter binary the v1
// adapter runs them with (SPEC.md §5.2).
var KnownLanguages = map[string]string{
	"bash":    "bash",
	"python3": "python3",
	"python":  "python3", // alias (SPEC.md §5.2)
}

// Preflight performs the environment-dependent checks of SPEC.md §2.3:
// script-reference files and interpreter availability. It appends to the
// result's diagnostics and performs no other side effects.
func (r *Result) Preflight() {
	d := r.Doc

	for _, b := range d.Blocks {
		if b.Kind != KindScript {
			continue
		}
		st, err := os.Stat(b.ScriptAbs)
		switch {
		case err != nil && os.IsNotExist(err):
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Severity: SeverityError,
				Line:     b.Line,
				Block:    b.Index,
				Message:  fmt.Sprintf("script file not found: %s", b.ScriptAbs),
			})
		case err != nil:
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Severity: SeverityError,
				Line:     b.Line,
				Block:    b.Index,
				Message:  fmt.Sprintf("cannot access script %s: %v", b.ScriptAbs, err),
			})
		case st.IsDir():
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Severity: SeverityError,
				Line:     b.Line,
				Block:    b.Index,
				Message:  fmt.Sprintf("script path is not a regular file: %s", b.ScriptAbs),
			})
		default:
			if f, openErr := os.Open(b.ScriptAbs); openErr != nil {
				r.Diagnostics = append(r.Diagnostics, Diagnostic{
					Severity: SeverityError,
					Line:     b.Line,
					Block:    b.Index,
					Message:  fmt.Sprintf("script is not readable: %s: %v", b.ScriptAbs, openErr),
				})
			} else {
				f.Close()
			}
		}
	}

	var binaries []string
	seenBin := map[string]bool{}
	seenLang := map[string]bool{}
	for _, b := range d.Blocks {
		bin, ok := KnownLanguages[b.Lang]
		if !ok {
			if !seenLang[b.Lang] {
				seenLang[b.Lang] = true
				r.Diagnostics = append(r.Diagnostics, Diagnostic{
					Severity: SeverityError,
					Line:     b.Line,
					Block:    b.Index,
					Message:  fmt.Sprintf("no adapter for language `%s` (v1 supports: bash, python3)", b.Lang),
				})
			}
			continue
		}
		if !seenBin[bin] {
			seenBin[bin] = true
			binaries = append(binaries, bin)
		}
	}
	sort.Strings(binaries)
	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err != nil {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Severity: SeverityError,
				Message:  fmt.Sprintf("interpreter `%s` not found on PATH", bin),
			})
		}
	}
}

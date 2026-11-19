package doc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
)

// knownLanguages maps fence languages to the interpreter binary the v1
// adapter runs them with (SPEC.md §5.2).
var knownLanguages = map[string]string{
	"bash":    "bash",
	"python3": "python3",
	"python":  "python3", // alias (SPEC.md §5.2)
}

// Preflight performs the environment-dependent checks of SPEC.md §2.3:
// script-reference files and interpreter availability. It appends to the
// result's diagnostics and performs no other side effects.
func (r *Result) Preflight() {
	r.preflightScripts()
	r.preflightInterpreters()
}

// preflightScripts checks every @script reference once. One os.Open call
// tests both existence and readability.
func (r *Result) preflightScripts() {
	for _, b := range r.Doc.Blocks {
		if b.Kind != KindScript {
			continue
		}
		f, err := os.Open(b.ScriptAbs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			r.add(newBlockDiag(SeverityError, b.Line, b.Index, fmt.Sprintf("script file not found: %s", b.ScriptAbs)))
		case err != nil:
			r.add(newBlockDiag(SeverityError, b.Line, b.Index, fmt.Sprintf("cannot access script %s: %v", b.ScriptAbs, err)))
		default:
			st, statErr := f.Stat()
			f.Close()
			if statErr != nil {
				r.add(newBlockDiag(SeverityError, b.Line, b.Index, fmt.Sprintf("cannot access script %s: %v", b.ScriptAbs, statErr)))
			} else if st.IsDir() {
				r.add(newBlockDiag(SeverityError, b.Line, b.Index, fmt.Sprintf("script path is not a regular file: %s", b.ScriptAbs)))
			}
		}
	}
}

// preflightInterpreters reports unknown languages and missing interpreters.
// It reports each binary only once.
func (r *Result) preflightInterpreters() {
	var binaries []string
	seenBin := map[string]bool{}
	seenLang := map[string]bool{}
	for _, b := range r.Doc.Blocks {
		bin, ok := knownLanguages[b.Lang]
		if !ok {
			if !seenLang[b.Lang] {
				seenLang[b.Lang] = true
				r.add(newBlockDiag(SeverityError, b.Line, b.Index, fmt.Sprintf("no adapter for language `%s` (v1 supports: bash, python3)", b.Lang)))
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
			r.add(newDiag(SeverityError, 0, fmt.Sprintf("interpreter `%s` not found on PATH", bin)))
		}
	}
}

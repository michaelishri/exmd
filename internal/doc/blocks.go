package doc

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
)

var langRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]*$`)

// parseFenceInfo implements the fence-info grammar of SPEC.md §3.1:
//
//	fence-info := lang ":" "exec" [ SP attribute ]*
//
// It reports whether the info string selects an executable block. Anything
// else — `bash`, `exec`, `bash:execute`, `bash : exec` — is a display fence.
func parseFenceInfo(info string) (lang string, attrs []BlockAttribute, matched bool) {
	info = strings.TrimSpace(info)
	colon := strings.IndexByte(info, ':')
	if colon <= 0 {
		return "", nil, false
	}
	lang = info[:colon]
	rest := info[colon+1:]
	if !langRE.MatchString(lang) {
		return "", nil, false
	}
	switch {
	case rest == "exec":
		return lang, nil, true
	case strings.HasPrefix(rest, "exec ") || strings.HasPrefix(rest, "exec\t"):
		for _, field := range strings.Fields(rest[len("exec"):]) {
			key, value, found := strings.Cut(field, "=")
			if !found || key == "" {
				attrs = append(attrs, BlockAttribute{Key: field, Malformed: true})
				continue
			}
			attrs = append(attrs, BlockAttribute{Key: key, Value: value})
		}
		return lang, attrs, true
	default:
		return "", nil, false
	}
}

// collectBlock turns a fenced code block into a *Block, or nil when the
// fence is display-only (SPEC.md §3.3).
func collectBlock(b *ast.FencedCodeBlock, src []byte, starts []int, docdir string) (*Block, []Diagnostic) {
	if b.Info == nil {
		return nil, nil
	}
	info := strings.TrimSpace(string(b.Info.Value(src)))
	lang, attrs, matched := parseFenceInfo(info)
	if !matched {
		return nil, nil
	}

	blk := &Block{
		Lang: lang,
		Line: lineOf(starts, b.Info.Segment.Start),
	}
	diags := attributeDiags(attrs, blk.Line)

	segs := b.Lines()
	blk.BodyLine = blk.Line + 1
	// Goldmark appends one segment per body line and consumes the closing
	// fence without appending it, so this is the closing fence's line
	// (or one past the last body line for an unclosed fence).
	blk.EndLine = blk.BodyLine + segs.Len()

	var body strings.Builder
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		body.Write(seg.Value(src))
	}
	bodyStr := strings.TrimRight(body.String(), "\n")

	if strings.TrimSpace(bodyStr) == "" {
		diags = append(diags, newDiag(SeverityError, blk.Line, "empty executable block"))
		return blk, diags
	}

	if p, ok := scriptRef(bodyStr); ok {
		if p == "" {
			diags = append(diags, newDiag(SeverityError, blk.BodyLine, "empty script path"))
		} else {
			blk.Kind = KindScript
			blk.ScriptPath = p
			blk.ScriptAbs = resolvePath(docdir, p)
		}
		return blk, diags
	}

	blk.Body = bodyStr
	return blk, diags
}

// attributeDiags reports v1's reserved block attributes (SPEC.md §3.1).
func attributeDiags(attrs []BlockAttribute, line int) []Diagnostic {
	diags := make([]Diagnostic, 0, len(attrs))
	for _, a := range attrs {
		if a.Malformed {
			diags = append(diags, newDiag(SeverityError, line,
				fmt.Sprintf("malformed block attribute `%s` (expected key=value)", a.Key)))
			continue
		}
		diags = append(diags, newDiag(SeverityError, line,
			fmt.Sprintf("unknown block attribute `%s` (v1 defines none; reserved)", a.Key)))
	}
	return diags
}

// scriptRef reports the path when the body is a single @script line.
func scriptRef(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	if len(lines) != 1 {
		return "", false
	}
	trimmed := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(trimmed, "@") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "@")), true
}

// resolvePath resolves a script-reference path against the document
// directory (SPEC.md §3.2); absolute paths are used as-is.
func resolvePath(dir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(dir, filepath.FromSlash(p))
}

// normalizeNewlines converts CRLF and lone CR to LF (SPEC.md §2.1 accepts
// CRLF documents; block source is passed to interpreters normalized). The
// common LF-only case returns src unchanged.
func normalizeNewlines(src []byte) []byte {
	if !bytes.ContainsAny(src, "\r") {
		return src
	}
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] == '\r' {
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			out = append(out, '\n')
			continue
		}
		out = append(out, src[i])
	}
	return out
}

// indexLines records the byte offset where each line starts.
func indexLines(src []byte) []int {
	starts := []int{0}
	for i, c := range src {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineOf converts a byte offset into a 1-based source line using a
// precomputed line index, avoiding a full scan per heading or block.
func lineOf(starts []int, offset int) int {
	return sort.SearchInts(starts, offset+1)
}

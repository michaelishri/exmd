package doc

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
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
func collectBlock(b *ast.FencedCodeBlock, src []byte, docdir string) (*Block, []Diagnostic) {
	if b.Info == nil {
		return nil, nil
	}
	info := strings.TrimSpace(string(b.Info.Value(src)))
	lang, attrs, matched := parseFenceInfo(info)
	if !matched {
		return nil, nil
	}

	blk := &Block{
		Lang:       lang,
		Attributes: attrs,
		Line:       lineOf(src, b.Info.Segment.Start),
	}
	diags := make([]Diagnostic, 0, 1)
	for _, a := range attrs {
		if a.Malformed {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     blk.Line,
				Message:  fmt.Sprintf("malformed block attribute `%s` (expected key=value)", a.Key),
			})
		} else {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Line:     blk.Line,
				Message:  fmt.Sprintf("unknown block attribute `%s` (v1 defines none; reserved)", a.Key),
			})
		}
	}

	segs := b.Lines()
	blk.BodyLines = segs.Len()
	blk.BodyLine = blk.Line + 1

	var body strings.Builder
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		body.Write(seg.Value(src))
	}
	bodyStr := strings.TrimRight(normalizeNewlines(body.String()), "\n")

	if strings.TrimSpace(bodyStr) == "" {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Line:     blk.Line,
			Message:  "empty executable block",
		})
		return blk, diags
	}

	lines := strings.Split(bodyStr, "\n")
	if len(lines) == 1 {
		trimmed := strings.TrimSpace(lines[0])
		if strings.HasPrefix(trimmed, "@") {
			p := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
			if p == "" {
				diags = append(diags, Diagnostic{
					Severity: SeverityError,
					Line:     blk.BodyLine,
					Message:  "empty script path",
				})
			} else {
				blk.Kind = KindScript
				blk.ScriptPath = p
				blk.ScriptAbs = resolvePath(docdir, p)
			}
			return blk, diags
		}
	}

	blk.Body = bodyStr
	return blk, diags
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
// CRLF documents; block source is passed to interpreters normalized).
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// lineOf converts a byte offset into a 1-based source line.
func lineOf(src []byte, offset int) int {
	return 1 + bytes.Count(src[:offset], []byte{'\n'})
}

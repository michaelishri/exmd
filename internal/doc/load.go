package doc

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Load reads and parses the document at path.
//
// I/O failures return an error. Every specification-level problem is
// reported as a diagnostic on the returned Result so that consumers can show
// all diagnostics at once (SPEC.md §2.3).
func Load(path string) (*Result, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	src = normalizeNewlines(src)

	dir := filepath.Dir(path)
	if abs, absErr := filepath.Abs(path); absErr == nil {
		dir = filepath.Dir(abs)
	}

	d := &Document{
		Path:    path,
		Dir:     dir,
		Globals: map[string]*string{},
		Source:  src,
	}
	res := &Result{Doc: d}

	yamlSrc, openLine, closed, bodyLine, masked := splitFrontmatter(src)
	d.BodyLine = bodyLine
	switch {
	case openLine == 0:
		// No frontmatter: the document cannot be a runbook (SPEC.md §2.1).
		res.add(newDiag(SeverityError, 0, "not a runbook: frontmatter is missing (expected `---`, `type: runbook`, `---` at the top of the file)"))
	case !closed:
		res.add(newDiag(SeverityError, openLine, "frontmatter is not closed (expected a closing `---` line)"))
	default:
		if parseFrontmatter(yamlSrc, openLine, d, res) && d.IsRunbook {
			collect(masked, indexLines(src), d, res)
		}
	}
	return res, nil
}

// collect walks the Markdown AST and gathers headings plus — for runbooks —
// executable blocks, in document order (SPEC.md §3.1: blocks inside list
// items still execute in document order).
func collect(src []byte, starts []int, d *Document, res *Result) {
	parsed := goldmark.New().Parser().Parse(text.NewReader(src))

	type entry struct {
		b     *Block
		diags []Diagnostic
	}
	var entries []entry

	ast.Walk(parsed, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Heading:
			if _, topLevel := t.Parent().(*ast.Document); topLevel {
				d.Headings = append(d.Headings, collectHeading(t, src, starts))
			}
		case *ast.FencedCodeBlock:
			if b, diags := collectBlock(t, src, starts, d.Dir); b != nil {
				entries = append(entries, entry{b: b, diags: diags})
			}
		}
		return ast.WalkContinue, nil
	})

	for i, e := range entries {
		e.b.Index = i + 1
		for j := range e.diags {
			e.diags[j].Block = e.b.Index
		}
		d.Blocks = append(d.Blocks, e.b)
		res.Diagnostics = append(res.Diagnostics, e.diags...)
	}
}

// collectHeading records a heading's text and the source lines it spans.
// An ATX heading (`# Title`) is one line, even when empty; a setext heading
// (`Title` over `=====`) runs from its first text line to its underline.
func collectHeading(h *ast.Heading, src []byte, starts []int) *Heading {
	segs := h.Lines()
	hd := &Heading{Level: h.Level}
	if pos := h.Pos(); pos >= 0 {
		hd.Line = lineOf(starts, pos)
	} else if segs.Len() > 0 {
		hd.Line = lineOf(starts, segs.At(0).Start)
	}
	hd.EndLine = hd.Line
	if segs.Len() == 0 {
		return hd
	}
	var sb strings.Builder
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		sb.Write(seg.Value(src))
	}
	hd.Text = strings.Join(strings.Fields(sb.String()), " ")
	if !isATX(src, starts, hd.Line) {
		hd.EndLine = lineOf(starts, segs.At(segs.Len()-1).Start) + 1
	}
	return hd
}

// isATX reports whether the source line starts with `#` after indentation.
func isATX(src []byte, starts []int, line int) bool {
	if line < 1 || line > len(starts) {
		return false
	}
	rest := bytes.TrimLeft(src[starts[line-1]:], " \t")
	return len(rest) > 0 && rest[0] == '#'
}

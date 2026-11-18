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

	dir := filepath.Dir(path)
	if abs, absErr := filepath.Abs(path); absErr == nil {
		dir = filepath.Dir(abs)
	}

	d := &Document{
		Path:        path,
		Dir:         dir,
		SpecVersion: 1,
		Globals:     map[string]*string{},
	}
	res := &Result{Doc: d}

	yamlSrc, openLine, closed, masked := splitFrontmatter(src)
	switch {
	case openLine == 0:
		// No frontmatter: the document cannot be a runbook (SPEC.md §2.1).
		res.Diagnostics = append(res.Diagnostics, Diagnostic{
			Severity: SeverityError,
			Message:  "not a runbook: frontmatter is missing (expected `---`, `type: runbook`, `---` at the top of the file)",
		})
	case !closed:
		res.Diagnostics = append(res.Diagnostics, Diagnostic{
			Severity: SeverityError,
			Line:     openLine,
			Message:  "frontmatter is not closed (expected a closing `---` line)",
		})
	default:
		if parseFrontmatter(yamlSrc, openLine, d, res) && d.IsRunbook {
			collect(masked, d, res)
		}
	}
	return res, nil
}

// collect walks the Markdown AST and gathers headings plus — for runbooks —
// executable blocks, in document order (SPEC.md §3.1: blocks inside list
// items still execute in document order).
func collect(src []byte, d *Document, res *Result) {
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
			if h := collectHeading(t, src); h != nil {
				d.Headings = append(d.Headings, h)
			}
		case *ast.FencedCodeBlock:
			if b, diags := collectBlock(t, src, d.Dir); b != nil {
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

func collectHeading(h *ast.Heading, src []byte) *Heading {
	segs := h.Lines()
	hd := &Heading{Level: h.Level}
	if segs.Len() == 0 {
		return hd
	}
	var sb strings.Builder
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		sb.Write(seg.Value(src))
	}
	hd.Text = strings.Join(strings.Fields(sb.String()), " ")
	hd.Line = lineOf(src, segs.At(0).Start)
	return hd
}

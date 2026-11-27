package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/theme"
)

func plainRender(t *testing.T, src string) (string, *Output) {
	t.Helper()
	d := loadSource(t, src)
	out := Render(d, 60, theme.Notty)
	var lines []string
	for _, l := range out.Lines {
		lines = append(lines, strings.TrimSpace(ansi.Strip(l)))
	}
	return strings.Join(lines, "\n"), out
}

// TestSetextHeadingsRenderLikeATX: an underlined heading renders exactly
// like its `#` form — no stray underline text or rule, and numbered alike.
func TestSetextHeadingsRenderLikeATX(t *testing.T) {
	head := "---\ntype: runbook\n---\n"
	setext, _ := plainRender(t, head+"Title\n=====\n\nSub\n---\n\nBody text.\n")
	atx, _ := plainRender(t, head+"# Title\n\n## Sub\n\nBody text.\n")
	if setext != atx {
		t.Fatalf("setext render differs from ATX:\n--- setext\n%s\n--- atx\n%s", setext, atx)
	}

	multi, out := plainRender(t, head+"# Doc\n\nTwo\nlines\n---\n\nAfter.\n")
	if !strings.Contains(multi, "## 1. Two") || strings.Contains(multi, "---") || len(out.Headings) != 2 {
		t.Fatalf("multi-line setext heading:\n%s", multi)
	}
}

// TestQuotedHeadingStaysInItsQuote: a heading inside a block quote is part
// of the quote, not a section that splits it.
func TestQuotedHeadingStaysInItsQuote(t *testing.T) {
	text, out := plainRender(t, "---\ntype: runbook\n---\n# Doc\n\n> # Quoted\n> more\n\nend\n")
	if len(out.Headings) != 1 {
		t.Fatalf("%d heading marks, want only the top-level heading", len(out.Headings))
	}
	if !strings.Contains(text, "| # Quoted\n|\n| more") {
		t.Fatalf("quote was split:\n%s", text)
	}
}

// TestEveryHeadingHasAMark: outline marks line up with the document's
// headings, including an empty `#` heading.
func TestEveryHeadingHasAMark(t *testing.T) {
	d := loadSource(t, "---\ntype: runbook\n---\n# One\n\n#\n\n## Two\n\nThree\n-----\n")
	out := Render(d, 60, theme.Notty)
	if len(out.Headings) != len(d.Headings) || len(d.Headings) != 4 {
		t.Fatalf("marks %d, headings %d; want 4 each", len(out.Headings), len(d.Headings))
	}
	for i := 1; i < len(out.Headings); i++ {
		if out.Headings[i].Line <= out.Headings[i-1].Line {
			t.Fatalf("marks out of order: %+v", out.Headings)
		}
	}
}

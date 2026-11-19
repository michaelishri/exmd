package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/md-exec/internal/doc"
)

func loadDemo(t *testing.T) *doc.Document {
	t.Helper()
	res, err := doc.Load("../../examples/demo/demo.md")
	if err != nil {
		t.Fatalf("load demo: %v", err)
	}
	if res.HasErrors() {
		t.Fatalf("demo has errors")
	}
	return res.Doc
}

func TestRenderStructure(t *testing.T) {
	d := loadDemo(t)
	out := Render(d, 80, false)
	if len(out.Lines) == 0 {
		t.Fatal("no lines rendered")
	}
	if len(out.Headings) != len(d.Headings) {
		t.Fatalf("heading marks = %d, want %d", len(out.Headings), len(d.Headings))
	}
	if len(out.Blocks) != len(d.Blocks) {
		t.Fatalf("block marks = %d, want %d", len(out.Blocks), len(d.Blocks))
	}
	// Marks must be in ascending order.
	for i := 1; i < len(out.Headings); i++ {
		if out.Headings[i].Line <= out.Headings[i-1].Line {
			t.Fatalf("heading marks out of order at %d", i)
		}
	}
	for i := 1; i < len(out.Blocks); i++ {
		if out.Blocks[i].Line <= out.Blocks[i-1].Line {
			t.Fatalf("block marks out of order at %d", i)
		}
	}
	// Executable blocks get a badge line with the source line number.
	var found bool
	for _, l := range out.Lines {
		if strings.Contains(l, "bash — line 17 · embedded") {
			found = true
		}
	}
	if !found {
		t.Fatal("badge for block 1 not found")
	}
}

func TestRenderWidth(t *testing.T) {
	d := loadDemo(t)
	for _, width := range []int{40, 80, 120} {
		out := Render(d, width, false)
		for i, l := range out.Lines {
			if w := ansi.StringWidth(l); w > width {
				t.Fatalf("line %d width %d exceeds %d: %q", i, w, width, l)
			}
		}
	}
}

func TestRenderDeterministic(t *testing.T) {
	d := loadDemo(t)
	a := Render(d, 80, false)
	b := Render(d, 80, false)
	if strings.Join(a.Lines, "\n") != strings.Join(b.Lines, "\n") {
		t.Fatal("render is not deterministic")
	}
}

// TestExecCodeRowsCarryBackground checks that an executable block's surface
// covers its code text, not just its badge row — chroma emits a per-token
// reset that would otherwise clear the background.
func TestExecCodeRowsCarryBackground(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	d := loadDemo(t)
	out := Render(d, 80, true)
	bm := out.Blocks[0]
	badge := out.Lines[bm.Line]
	if strings.Contains(badge, execPanelEscape) {
		t.Fatalf("badge row should sit on the page background: %q", badge)
	}
	// The first non-blank row after the badge is actual code text (the
	// row right after the badge is the shaded blank above the code).
	var code string
	for i := bm.Line + 1; i < bm.End; i++ {
		if strings.TrimSpace(ansi.Strip(out.Lines[i])) != "" {
			code = out.Lines[i]
			break
		}
	}
	if code == "" {
		t.Fatal("no code text row in the block")
	}
	if !strings.Contains(code, execPanelEscape) {
		t.Fatalf("code row has no exec background: %q", code)
	}
	if !strings.Contains(code, "\x1b[0m"+execPanelEscape) {
		t.Fatal("background is not re-emitted after chroma's token reset")
	}
	// The block ends with a shaded blank row inside its surface, then a
	// plain spacing row on the page.
	shadedBlank := out.Lines[bm.End-1]
	if !strings.Contains(shadedBlank, execPanelEscape) ||
		strings.TrimSpace(ansi.Strip(shadedBlank)) != "" {
		t.Fatalf("no shaded blank row after the code: %q", shadedBlank)
	}
	plain := out.Lines[bm.End]
	if strings.Contains(plain, execPanelEscape) || strings.TrimSpace(ansi.Strip(plain)) != "" {
		t.Fatalf("no plain spacing row below the block: %q", plain)
	}
	// The first column stays on the page background: the shaded surface
	// begins at column 1, so the block reads as indented.
	if !strings.HasPrefix(code, " "+execPanelEscape) {
		t.Fatalf("the first column should be page-colored: %q", code)
	}
	leading := func(s string) int {
		v := ansi.Strip(s)
		return len(v) - len(strings.TrimLeft(v, " "))
	}
	if want := leading(out.Lines[out.Headings[1].Line]) + 2; leading(code) != want {
		t.Fatalf("code indented %d spaces, want %d (heading + block margin)", leading(code), want)
	}
}

// TestLastBlockExtentExcludesSpacingRow checks that the final block's rail
// extent stops before the trailing blank row, like every other block.
func TestLastBlockExtentExcludesSpacingRow(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	d := loadDemo(t)
	out := Render(d, 80, true)
	last := out.Blocks[len(out.Blocks)-1]
	if last.End >= len(out.Lines) {
		t.Fatalf("last block extent %d past output (%d lines)", last.End, len(out.Lines))
	}
	if row := ansi.Strip(out.Lines[last.End]); strings.TrimSpace(row) != "" {
		t.Fatalf("extent should stop before the trailing blank row: %q", row)
	}
}

// TestHeadingFollowedByBlank checks that every heading in the rendered
// output is followed by an empty line.
func TestHeadingFollowedByBlank(t *testing.T) {
	d := loadDemo(t)
	out := Render(d, 80, true)
	for _, hm := range out.Headings {
		if hm.Line+1 >= len(out.Lines) {
			continue // the document ends with the heading
		}
		if strings.TrimSpace(ansi.Strip(out.Lines[hm.Line+1])) != "" {
			t.Fatalf("heading at line %d not followed by a blank line: %q",
				hm.Line, ansi.Strip(out.Lines[hm.Line+1]))
		}
	}
}

func TestExecBlocksShadedApartFromDisplayFences(t *testing.T) {
	// Executable blocks carry a soft raised shade; display-only fences
	// keep the neutral panel shade; inline code in prose is unchanged.
	dark, exec := darkStyle(), execDarkStyle()
	if dark.CodeBlock.StylePrimitive.BackgroundColor == nil ||
		exec.CodeBlock.StylePrimitive.BackgroundColor == nil {
		t.Fatal("code-block backgrounds unset")
	}
	if got := *exec.CodeBlock.StylePrimitive.BackgroundColor; got != execPanel {
		t.Fatalf("exec background = %q, want %q", got, execPanel)
	}
	if *dark.Code.StylePrimitive.BackgroundColor != *exec.Code.StylePrimitive.BackgroundColor {
		t.Fatal("inline code shade should not change between styles")
	}
}

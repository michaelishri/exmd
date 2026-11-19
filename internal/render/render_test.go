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
	out, err := Render(d, 80, false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
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
		out, err := Render(d, width, false)
		if err != nil {
			t.Fatalf("Render(%d): %v", width, err)
		}
		for i, l := range out.Lines {
			if w := ansi.StringWidth(l); w > width {
				t.Fatalf("line %d width %d exceeds %d: %q", i, w, width, l)
			}
		}
	}
}

func TestRenderDeterministic(t *testing.T) {
	d := loadDemo(t)
	a, err := Render(d, 80, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(d, 80, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(a.Lines, "\n") != strings.Join(b.Lines, "\n") {
		t.Fatal("render is not deterministic")
	}
}

// TestExecCodeRowsUsePageBackground checks that executable blocks carry
// no background of their own — badge, code rows and spacing all sit on the
// page — while keeping their indentation and spacing cues.
func TestExecCodeRowsUsePageBackground(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	d := loadDemo(t)
	out, err := Render(d, 80, true)
	if err != nil {
		t.Fatal(err)
	}
	bm := out.Blocks[0]

	// The first non-blank row after the badge is actual code text (the
	// row right after the badge is the blank above the code).
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
	// No row in the block carries a background escape; only foreground
	// colors from glamour/chroma remain.
	for i := bm.Line; i < bm.End; i++ {
		if strings.Contains(out.Lines[i], "\x1b[48;") {
			t.Fatalf("block row %d carries a background: %q", i, out.Lines[i])
		}
	}
	// Blank row after the code, then a spacing row below the block.
	for i := bm.End - 1; i <= bm.End; i++ {
		if strings.TrimSpace(ansi.Strip(out.Lines[i])) != "" {
			t.Fatalf("no blank spacing row at %d: %q", i, out.Lines[i])
		}
	}
	// Code remains indented inside the block by its two-column margin.
	leading := func(s string) int {
		v := ansi.Strip(s)
		return len(v) - len(strings.TrimLeft(v, " "))
	}
	if want := leading(out.Lines[out.Headings[1].Line]) + 2; leading(code) != want {
		t.Fatalf("code indented %d spaces, want %d (heading + block margin)", leading(code), want)
	}
}

// TestHeadingFollowedByBlank checks that every heading in the rendered
// output is followed by an empty line.
func TestHeadingFollowedByBlank(t *testing.T) {
	d := loadDemo(t)
	out, err := Render(d, 80, true)
	if err != nil {
		t.Fatal(err)
	}
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

func TestExecBlocksUseNoBackground(t *testing.T) {
	// Executable blocks render on the page background; display-only fences
	// keep the neutral panel shade. Inline code in prose is unchanged.
	dark, exec := darkStyle(), execDarkStyle()
	if dark.CodeBlock.StylePrimitive.BackgroundColor == nil {
		t.Fatal("display fence background unset")
	}
	if exec.CodeBlock.StylePrimitive.BackgroundColor != nil {
		t.Fatalf("exec block should have no background, got %q",
			*exec.CodeBlock.StylePrimitive.BackgroundColor)
	}
	if *dark.Code.StylePrimitive.BackgroundColor != *exec.Code.StylePrimitive.BackgroundColor {
		t.Fatal("inline code shade should not change between styles")
	}
}

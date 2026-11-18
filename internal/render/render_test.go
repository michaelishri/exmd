package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

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

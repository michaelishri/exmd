package tui

import (
	"strings"
	"testing"
)

// TestOutlinePaddingAndSpacing checks the sidebar geometry: the strip is
// full height, its first and last rows are blank internal pads, the entries
// sit between them with one column of base left padding, rows are packed
// (no blank lines between entries), and the outer margin rows sit outside
// the strip.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	// Internal pad rows: the first row of the strip is painted blank.
	if lines[0] == "" || strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("top internal pad row missing: %q", lines[0])
	}

	// The entries sit between the pads: the first entry (an h1, selected
	// by the scroll-spy) one row down, one column in.
	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	if n > rows-2 {
		t.Fatalf("demo headings exceed the sidebar: %d", n)
	}
	if !strings.HasPrefix(lines[1], " ▸ md-exec feature tour") {
		t.Fatalf("first entry wrong: %q", lines[1])
	}
	for r := 2; r <= n; r++ {
		if strings.TrimSpace(lines[r]) == "" {
			t.Fatalf("row %d is blank: blank lines were removed", r)
		}
		if !strings.HasPrefix(lines[r], " ") {
			t.Fatalf("row %d has no base padding: %q", r, lines[r])
		}
	}
	// Blank shaded rows fill the rest of the column — full height, with
	// the outer margins sitting outside the strip.
	if lines[len(lines)-1] == "" {
		t.Fatal("strip should be painted to the bottom of the body")
	}
}

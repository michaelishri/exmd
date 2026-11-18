package tui

import (
	"strings"
	"testing"
)

// TestOutlinePaddingAndSpacing checks the sidebar geometry: the strip is
// full height, entries start at its first row with one column of base left
// padding, rows are packed (no blank lines between entries), blank shaded
// rows fill below the entries, and the outer margin rows sit outside the
// strip.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	// The strip starts at its first row: the first entry (an h1, selected
	// by the scroll-spy) sits directly at the top, one column in.
	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	if !strings.HasPrefix(lines[0], " ▸ md-exec feature tour") {
		t.Fatalf("first entry wrong: %q", lines[0])
	}
	for r := 1; r < n; r++ {
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

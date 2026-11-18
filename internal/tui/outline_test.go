package tui

import (
	"strings"
	"testing"
)

// TestOutlinePaddingAndSpacing checks the sidebar geometry: the strip hugs
// the entries — one pad row above and below, packed rows, and exactly two
// blank rows below the last entry (pad plus the viewer's bottom margin),
// with the rest of the column unpainted.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	// Top pad row (painted blank), then the entries (the first one is an
	// h1, selected by the scroll-spy).
	if lines[0] == "" || strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("top pad row missing: %q", lines[0])
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
	// Bottom pad row directly under the last entry (painted blank)...
	if lines[n+1] == "" || strings.TrimSpace(lines[n+1]) != "" {
		t.Fatalf("bottom pad row missing: %q", lines[n+1])
	}
	// ...and nothing painted below the strip.
	if lines[n+2] != "" {
		t.Fatalf("filler below the strip should be unpainted: %q", lines[n+2])
	}
}

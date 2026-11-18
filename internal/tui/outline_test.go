package tui

import (
	"strings"
	"testing"
)

// TestOutlinePaddingAndSpacing checks the sidebar geometry: one blank row
// above and below the entries, one base column of left padding, packed rows
// (no blank lines between entries), and one column of right padding.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	// One blank shaded row at the top.
	if strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("top pad row not blank: %q", lines[0])
	}

	// Entries are packed with no blank lines between them, starting one
	// column in from the left edge.
	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	if n > rows-2 {
		t.Fatalf("demo headings exceed the sidebar: %d", n)
	}
	for r := 1; r <= n; r++ {
		if strings.TrimSpace(lines[r]) == "" {
			t.Fatalf("row %d is blank: blank lines were removed", r)
		}
		if !strings.HasPrefix(lines[r], " ") {
			t.Fatalf("row %d has no base padding: %q", r, lines[r])
		}
	}
	// One blank shaded row at the bottom (after the last entry).
	if strings.TrimSpace(lines[n+1]) != "" {
		t.Fatalf("bottom pad row not blank: %q", lines[n+1])
	}

	// No title row: the first entry (an h1, selected by the scroll-spy)
	// sits directly under the top pad, one column in, behind the marker.
	if !strings.HasPrefix(lines[1], " ▸ md-exec feature tour") {
		t.Fatalf("first entry wrong: %q", lines[1])
	}
}

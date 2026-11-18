package tui

import (
	"strings"
	"testing"
)

// TestOutlinePaddingAndSpacing checks the sidebar breathes horizontally:
// entries from the top row (no title row), left padding on every row, packed
// rows — no blank lines between entries.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	// Entries are packed with no blank lines between them.
	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	if n > rows {
		t.Fatalf("demo headings exceed the sidebar: %d", n)
	}
	for r := 0; r < n; r++ {
		if strings.TrimSpace(lines[r]) == "" {
			t.Fatalf("row %d is blank: blank lines were removed", r)
		}
		if !strings.HasPrefix(lines[r], "  ") {
			t.Fatalf("row %d not left-padded: %q", r, lines[r])
		}
	}
	// Beyond the entries the column is blank filler.
	if strings.TrimSpace(lines[n]) != "" {
		t.Fatalf("row %d after the entries should be blank: %q", n, lines[n])
	}
	// No title row: the sidebar starts with the first entry.
	if !strings.Contains(lines[0], "md-exec feature tour") {
		t.Fatalf("first entry not at the top row: %q", lines[0])
	}
}

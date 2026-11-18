package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// TestOutlinePaddingAndSpacing checks the sidebar breathes horizontally:
// padded title, left padding on entries, packed rows — no blank lines.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height + 2*paneInset
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	if !strings.HasPrefix(lines[0], "  ") || !strings.Contains(lines[0], "Outline") {
		t.Fatalf("title row not padded: %q", lines[0])
	}

	// Entries are packed with no blank lines between them.
	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	if n > rows-1 {
		t.Fatalf("demo headings exceed the sidebar: %d", n)
	}
	for r := 1; r <= n; r++ {
		if strings.TrimSpace(lines[r]) == "" {
			t.Fatalf("row %d is blank: blank lines were removed", r)
		}
		if !strings.HasPrefix(lines[r], "  ") {
			t.Fatalf("row %d not left-padded: %q", r, lines[r])
		}
	}
	// Beyond the entries the column is blank filler.
	if strings.TrimSpace(lines[n+1]) != "" {
		t.Fatalf("row %d after the entries should be blank: %q", n+1, lines[n+1])
	}
}

// TestFocusRingMoves checks tab focus is visible: exactly one pane carries
// the ring, it encloses the content by default and the sidebar after tab,
// and toggling focus never shifts the layout.
func TestFocusRingMoves(t *testing.T) {
	m := newViewer(t, 120, 40)

	v := ansi.Strip(m.View())
	if n := strings.Count(v, "┌"); n != 1 {
		t.Fatalf("focus rings = %d, want exactly 1", n)
	}
	if col := ringColumn(v); col != 0 {
		t.Fatalf("content ring corner at column %d, want 0", col)
	}
	widthBefore := m.content.Width

	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := mm.(Model)
	if got.content.Width != widthBefore {
		t.Fatalf("content width shifted on tab: %d -> %d", widthBefore, got.content.Width)
	}

	v = ansi.Strip(got.View())
	if n := strings.Count(v, "┌"); n != 1 {
		t.Fatalf("focus rings after tab = %d, want exactly 1", n)
	}
	if col := ringColumn(v); col < m.width/2 {
		t.Fatalf("sidebar ring corner at column %d, want the right half", col)
	}

	// Tab back: the ring returns to the content.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = mm.(Model)
	if col := ringColumn(ansi.Strip(got.View())); col != 0 {
		t.Fatalf("ring corner after tab back at column %d, want 0", col)
	}
}

// ringColumn finds the column of the ring's top-left corner (-1 if absent).
func ringColumn(view string) int {
	for _, l := range strings.Split(view, "\n") {
		if i := strings.Index(l, "┌"); i >= 0 {
			return i
		}
	}
	return -1
}

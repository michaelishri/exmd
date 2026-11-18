package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestOutlinePaddingAndSpacing checks the sidebar breathes: padded title,
// a spacer row under it, left padding on entries, and blank shaded rows
// between entries.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	v := m.outline.view(m.content.Height, false)
	lines := strings.Split(v, "\n")
	if len(lines) != m.content.Height {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), m.content.Height)
	}

	first := func(s string) string { return strings.TrimSpace(s) }

	if !strings.HasPrefix(first(lines[0]), "Outline") || !strings.HasPrefix(lines[0], "  ") {
		t.Fatalf("title row not padded: %q", first(lines[0]))
	}
	if !isBlankRow(lines[1]) {
		t.Fatalf("no spacer under the title: %q", lines[1])
	}
	if isBlankRow(lines[2]) {
		t.Fatal("first entry is blank")
	}
	if !strings.HasPrefix(lines[2], "  ") {
		t.Fatalf("entry row not left-padded: %q", lines[2])
	}
	if !isBlankRow(lines[3]) {
		t.Fatalf("no blank row between entries: %q", lines[3])
	}
	if isBlankRow(lines[4]) {
		t.Fatal("second entry is blank")
	}
}

func isBlankRow(s string) bool { return strings.TrimSpace(s) == "" }

// TestOutlineFocusShade checks that tab focus is visible: with a color
// profile forced, the focused sidebar column renders differently from the
// unfocused one (the whole column lifts one background shade).
func TestOutlineFocusShade(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	unfocused := m.outline.view(m.content.Height, false)
	focused := m.outline.view(m.content.Height, true)
	if unfocused == focused {
		t.Fatal("focus did not change the sidebar shade")
	}
	// And with color stripped the views are identical again — the focus
	// treatment is a shade, not a layout change.
	if ansi.Strip(unfocused) != ansi.Strip(focused) {
		t.Fatal("focus treatment changed the sidebar layout")
	}
}

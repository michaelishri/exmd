package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestOutlineUsesHeadingNumbers checks that the sidebar labels carry the
// same generated numbers as the rendered headings, and that the title h1 is
// still excluded from the outline.
func TestOutlineUsesHeadingNumbers(t *testing.T) {
	m := newViewerFromDoc(t, 120, 40, "../../internal/render/testdata/unnumbered.md", nil)
	want := []string{
		"1. Preflight", "1.1. Checks", "1.1.1. Deep",
		"1.2. More checks", "2. Build", "2.1. Package",
	}
	if len(m.outline.entries) != len(want) {
		t.Fatalf("outline entries = %d, want %d", len(m.outline.entries), len(want))
	}
	for i, e := range m.outline.entries {
		if got := ansi.Strip(e.label); got != want[i] {
			t.Errorf("outline entry %d = %q, want %q", i, got, want[i])
		}
	}
	if m.outline.title != "Numbered sections" {
		t.Fatalf("outline title = %q, want the unnumbered h1", m.outline.title)
	}
}

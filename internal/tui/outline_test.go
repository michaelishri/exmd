package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestOutlinePaddingAndSpacing checks the sidebar geometry: the strip is
// full height; a blank pad row, the title row (the document's first h1, two
// columns in), a blank pad row, then the packed entries (one column of base
// left padding, no blank lines between entries); blank shaded rows fill the
// rest, and the outer margin rows sit outside the strip.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	// Top pad row, then the title (the first h1, two columns in), then
	// another pad row.
	if lines[0] == "" || strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("top pad row missing: %q", lines[0])
	}
	if !strings.Contains(lines[1], "md-exec feature tour") {
		t.Fatalf("title row missing the first h1: %q", lines[1])
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Fatalf("title not two columns in: %q", lines[1])
	}
	if lines[2] == "" || strings.TrimSpace(lines[2]) != "" {
		t.Fatalf("pad row missing under the title: %q", lines[2])
	}

	// The entries sit below the pads: the first h1 is the title row and is
	// not a selectable entry, so the outline starts with the next heading.
	// Nothing is selected while the reader is in the h1's section, so the
	// first entry row has no marker yet.
	n := len(m.outline.entries)
	if n < 2 {
		t.Fatalf("demo should have several headings, got %d", n)
	}
	if n > rows-4 {
		t.Fatalf("demo headings exceed the sidebar: %d", n)
	}
	if m.outline.sel != -1 {
		t.Fatalf("initial sel = %d, want -1 (the h1 section has no entry)", m.outline.sel)
	}
	if !strings.HasPrefix(lines[3], "   ") ||
		!strings.Contains(lines[3], "1. Preflight and point-of-use guards") {
		t.Fatalf("first entry wrong: %q", lines[3])
	}
	for r := 4; r <= n+2; r++ {
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

// TestSelectionKeepsTextColumn checks that selecting an entry does not
// shift its text: the stripped row is identical before and after, and the
// background carries the selection instead.
func TestSelectionKeepsTextColumn(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	before := strings.Split(m.outline.view(m.content.Height), "\n")

	// Focus the sidebar and select the first entry.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm, _ = mm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := mm.(Model)
	after := strings.Split(got.outline.view(got.content.Height), "\n")

	if got.outline.sel != 0 {
		t.Fatalf("sel = %d, want 0", got.outline.sel)
	}
	if ansi.Strip(after[3]) != ansi.Strip(before[3]) {
		t.Fatalf("selection shifted the text: %q -> %q",
			ansi.Strip(before[3]), ansi.Strip(after[3]))
	}
	if after[3] == before[3] {
		t.Fatal("selected row should render with a different background")
	}
	// Unselected rows keep their exact rendering.
	if after[4] != before[4] {
		t.Fatal("unselected rows should not change when the cursor moves")
	}
}

// TestOutlineTitleIsFirstH1 checks that the sidebar title is the document's
// first level-1 heading even when it is not the first body line, and that
// the title is not a selectable entry.
func TestOutlineTitleIsFirstH1(t *testing.T) {
	src := `---
type: runbook
title: Fixture
spec: 1
---

Intro prose before any heading.

## An h2 comes first

# The Real Title

More text after the h1.
`
	path := filepath.Join(t.TempDir(), "late-h1.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newViewerFromDoc(t, 120, 40, path, nil)
	if m.outline.title != "The Real Title" {
		t.Fatalf("sidebar title = %q, want the first h1 %q", m.outline.title, "The Real Title")
	}
	// The h1 is the title, not an entry: only the h2 remains.
	if len(m.outline.entries) != 1 || m.outline.entries[0].label != "An h2 comes first" {
		t.Fatalf("entries = %+v, want only the h2", m.outline.entries)
	}
	v := m.outline.view(m.content.Height)
	if !strings.Contains(strings.Split(v, "\n")[1], "The Real Title") {
		t.Fatalf("title row wrong: %q", strings.Split(v, "\n")[1])
	}
}

// TestOutlineTitleFallback checks documents without an h1: the title falls
// back to the document title (frontmatter, else file name).
func TestOutlineTitleFallback(t *testing.T) {
	m := newViewerFromDoc(t, 120, 40, "../../internal/doc/testdata/valid-flow.md", nil)
	if m.outline.title != "Flow globals" {
		t.Fatalf("sidebar title = %q, want the frontmatter title %q", m.outline.title, "Flow globals")
	}
}

// TestHeadingRowShading checks the level hierarchy: h2 rows render brighter
// than h3 rows, and the title row is bold. The test forces a color profile
// so the ANSI escapes differ.
func TestHeadingRowShading(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	v := m.outline.view(m.content.Height)
	lines := strings.Split(v, "\n")

	// Entries start at row 3: entry 0 is an h2 (1. Preflight…), entry 5 is
	// an h3 (5.1 From bash).
	h2 := lines[3]
	h3 := lines[3+5]
	if !strings.Contains(ansi.Strip(h2), "1. Preflight") ||
		!strings.Contains(ansi.Strip(h3), "5.1 From bash") {
		t.Fatal("test setup: rows are not the expected entries")
	}
	if h2 == h3 {
		t.Fatal("h2 and h3 rows should render with different shading")
	}
	title := lines[1]
	if !strings.Contains(ansi.Strip(title), "md-exec feature tour") {
		t.Fatal("test setup: row 1 is not the title row")
	}
	if title == h2 {
		t.Fatal("title row should render brighter (bold) than h2 rows")
	}
}

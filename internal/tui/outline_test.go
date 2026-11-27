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
// rest of the strip, flush to the right edge.
func TestOutlinePaddingAndSpacing(t *testing.T) {
	m := newViewer(t, 120, 40)
	rows := m.content.Height
	v := m.outline.view(rows, false, m.styles)
	lines := strings.Split(v, "\n")
	if len(lines) != rows {
		t.Fatalf("sidebar rows = %d, want %d", len(lines), rows)
	}

	// A blank pad row, then the title (the first h1, two columns in), then
	// another pad row.
	if lines[0] == "" || strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("top pad row missing: %q", lines[0])
	}
	if !strings.Contains(lines[1], "exmd feature tour") {
		t.Fatalf("title row missing the first h1: %q", lines[1])
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Fatalf("title not two columns in: %q", lines[1])
	}
	if lines[2] == "" || strings.TrimSpace(lines[2]) != "" {
		t.Fatalf("pad row missing under the title: %q", lines[2])
	}

	// The entries sit below the pads: the first h1 is the title row (itself
	// selectable via sel == -1), so the entries list starts with the next
	// heading. At the top of the document the title holds the cursor, so
	// the first entry row carries no marker yet.
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
	// Blank shaded rows fill the rest of the column — full height, flush
	// to the right edge with no outer margin.
	if lines[len(lines)-1] == "" {
		t.Fatal("strip should be painted to the bottom of the body")
	}
}

// TestRailMarksHeadingLine checks the selection rail at the left edge of
// the text area: no bar renders without a selection; with an entry
// selected, exactly one bar marks its heading's line (not the whole
// section), aligned to the content row the jump placed at the top.
func TestRailMarksHeadingLine(t *testing.T) {
	m := newViewer(t, 120, 40)
	if v := m.View(); strings.Contains(v, "▌") {
		t.Fatal("no bar should render without a selection")
	}

	// Focus the sidebar and select the first entry.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm, _ = mm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := mm.(Model)
	if got.outline.sel != 0 {
		t.Fatalf("sel = %d, want 0", got.outline.sel)
	}

	// The jump put the heading one row below the viewport top (its blank
	// separator row is at the top): exactly one rail bar, on the same body
	// row as the heading's text, at the rail column (the leftmost column).
	// The sidebar's border bars live further right and don't count.
	v := got.View()
	railRows := 0
	found := false
	for _, line := range strings.Split(v, "\n") {
		stripped := ansi.Strip(line)
		i := strings.Index(stripped, "▌")
		if i < 0 || ansi.StringWidth(stripped[:i]) != 0 {
			continue
		}
		railRows++
		if strings.Contains(stripped, "## 1. Preflight and point-of-use guards") {
			found = true
		}
	}
	if railRows != 1 {
		t.Fatalf("rail bar rows = %d, want 1", railRows)
	}
	if !found {
		t.Fatal("rail bar not beside the selected heading's row")
	}
}

// TestFocusBorderMoves checks that no sidebar border is drawn in any focus
// state and that the viewport frame never renders.
func TestFocusBorderMoves(t *testing.T) {
	m := newViewer(t, 120, 40)
	resting := m.View()

	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	focused := mm.(Model).View()

	for name, v := range map[string]string{"resting": resting, "focused": focused} {
		if n := strings.Count(v, "│"); n != 0 {
			t.Errorf("%s view has %d sidebar border rows, want 0", name, n)
		}
		if n := strings.Count(v, "┌"); n != 0 {
			t.Errorf("%s view has %d frame boxes, want 0", name, n)
		}
	}
}

// TestSidebarFocusKeepsLayout checks the focused sidebar: the strip's own
// rendering is identical with and without focus (nothing shifts), and focus
// is expressed by shading alone — no extra chrome in the gutter.
func TestSidebarFocusKeepsLayout(t *testing.T) {
	m := newViewer(t, 120, 40)
	unfocused := strings.Split(m.outline.view(m.paneHeight, false, m.styles), "\n")
	focused := strings.Split(m.outline.view(m.paneHeight, true, m.styles), "\n")

	// The strip's own rendering does not depend on focus at all.
	for i := range unfocused {
		if ansi.Strip(unfocused[i]) != ansi.Strip(focused[i]) {
			t.Fatalf("row %d shifted on focus:\n before: %q\n after:  %q",
				i, ansi.Strip(unfocused[i]), ansi.Strip(focused[i]))
		}
	}

	// Focus adds no border or frame in the full view either.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	v := mm.(Model).View()
	if strings.Contains(v, "│") || strings.Contains(v, "┌") {
		t.Fatal("focus added border or frame chrome")
	}
}

// TestSidebarFocusShading checks the resting-to-focused lift: the strip is
// painted for its full height on the panel shade at rest, and focus lifts it
// to the raised element shade. No border is drawn in either state. (The
// title row is excluded: at the top of the document it holds the cursor.)
// TrueColor is forced so the exact colors are assertable.
func TestSidebarFocusShading(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	unfocused := strings.Split(m.outline.view(m.paneHeight, false, m.styles), "\n")
	focused := strings.Split(m.outline.view(m.paneHeight, true, m.styles), "\n")

	// The first entry (body row 3) is unselected in both states.
	if ansi.Strip(unfocused[3]) != ansi.Strip(focused[3]) {
		t.Fatal("test setup: the rows should carry the same text")
	}
	if !strings.Contains(unfocused[3], "48;2;24;24;30") {
		t.Fatalf("resting row should use the panel background: %q", unfocused[3])
	}
	if !strings.Contains(unfocused[3], "38;2;227;227;233") {
		t.Fatalf("resting row should keep its bright text color: %q", unfocused[3])
	}
	if !strings.Contains(focused[3], "48;2;36;36;44") {
		t.Fatalf("focused row should lift to the element background: %q", focused[3])
	}

	// Neither focus state draws a border beside the strip.
	if strings.Contains(m.View(), "│") || strings.Contains(gotFocusedView(t), "│") {
		t.Fatal("sidebar should have no left border")
	}
}

// gotFocusedView tabs focus to the sidebar and returns the rendered view.
func gotFocusedView(t *testing.T) string {
	t.Helper()
	m := newViewer(t, 120, 40)
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	return mm.(Model).View()
}

// TestSelectionKeepsTextColumn checks that selecting an entry does not
// shift its text: the stripped row is identical before and after, and the
// background carries the selection instead.
func TestSelectionKeepsTextColumn(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	before := strings.Split(m.outline.view(m.paneHeight, false, m.styles), "\n")

	// Focus the sidebar and select the first entry.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm, _ = mm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := mm.(Model)
	after := strings.Split(got.outline.view(got.paneHeight, false, got.styles), "\n")

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

// TestOutlineTitlePrefersFrontmatter checks that a frontmatter title wins
// over the h1 in the sidebar, while the h1 stays in the document body and
// remains excluded from the heading entries.
func TestOutlineTitlePrefersFrontmatter(t *testing.T) {
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
	if m.outline.title != "Fixture" {
		t.Fatalf("sidebar title = %q, want the frontmatter title %q", m.outline.title, "Fixture")
	}
	// The h1 is still the selectable title row (sel == -1), not an entry:
	// only the h2 remains in the entries list.
	if len(m.outline.entries) != 1 || m.outline.entries[0].label != "1. An h2 comes first" {
		t.Fatalf("entries = %+v, want only the numbered h2", m.outline.entries)
	}
	if !strings.Contains(strings.Join(m.contentLines(), "\n"), "The Real Title") {
		t.Fatal("the h1 disappeared from the document body")
	}
	v := m.outline.view(m.paneHeight, false, m.styles)
	if !strings.Contains(strings.Split(v, "\n")[1], "Fixture") {
		t.Fatalf("title row wrong: %q", strings.Split(v, "\n")[1])
	}
}

// TestOutlineTitleFallsBackToH1 checks documents with an h1 but no
// frontmatter title: the h1 is used as before.
func TestOutlineTitleFallsBackToH1(t *testing.T) {
	src := `---
type: runbook
spec: 1
---

# Find and Replace

## Steps
`
	path := filepath.Join(t.TempDir(), "no-title.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newViewerFromDoc(t, 120, 40, path, nil)
	if m.outline.title != "Find and Replace" {
		t.Fatalf("sidebar title = %q, want the h1 %q", m.outline.title, "Find and Replace")
	}
	if len(m.outline.entries) != 1 || m.outline.entries[0].label != "1. Steps" {
		t.Fatalf("entries = %+v, want only the numbered h2", m.outline.entries)
	}
}

// TestTitleRowIsSelectable checks that the title row can hold the sidebar
// cursor and that selecting it scrolls to the very top of the document.
func TestTitleRowIsSelectable(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	// Focus the sidebar and pick the first heading, scrolling off the top.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm, _ = mm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := mm.(Model)
	if got.outline.sel != 0 || got.content.YOffset == 0 {
		t.Fatalf("setup: sel = %d, YOffset = %d; want first heading, scrolled down",
			got.outline.sel, got.content.YOffset)
	}
	unselected := strings.Split(got.outline.view(got.paneHeight, false, got.styles), "\n")[1]

	// Moving up from the first entry lands on the title and jumps home.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = mm.(Model)
	if got.outline.sel != -1 {
		t.Fatalf("sel after k = %d, want -1 (title)", got.outline.sel)
	}
	if got.content.YOffset != 0 {
		t.Fatalf("YOffset after k = %d, want 0 (top of document)", got.content.YOffset)
	}
	selected := strings.Split(got.outline.view(got.paneHeight, false, got.styles), "\n")[1]
	if ansi.Strip(selected) != ansi.Strip(unselected) {
		t.Fatalf("selection shifted the title text: %q -> %q",
			ansi.Strip(unselected), ansi.Strip(selected))
	}
	if selected == unselected {
		t.Fatal("the selected title row should carry a different background")
	}

	// g selects the title from anywhere else in the outline, too.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got = mm.(Model)
	if got.outline.sel != len(got.outline.entries)-1 {
		t.Fatalf("G did not reach the last entry: sel = %d", got.outline.sel)
	}
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = mm.(Model)
	if got.outline.sel != -1 || got.content.YOffset != 0 {
		t.Fatalf("g: sel = %d, YOffset = %d; want title at the top",
			got.outline.sel, got.content.YOffset)
	}

	// Down from the title returns to the first entry.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = mm.(Model)
	if got.outline.sel != 0 {
		t.Fatalf("j from the title = %d, want 0", got.outline.sel)
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
	v := m.outline.view(m.paneHeight, false, m.styles)
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
	if !strings.Contains(ansi.Strip(title), "exmd feature tour") {
		t.Fatal("test setup: row 1 is not the title row")
	}
	if title == h2 {
		t.Fatal("title row should render brighter (bold) than h2 rows")
	}
}

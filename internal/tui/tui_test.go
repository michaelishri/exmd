package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/theme"
)

func newViewer(t *testing.T, w, h int) Model {
	t.Helper()
	return newViewerFromDoc(t, w, h, "../../examples/demo/demo.md", map[string]string{"JOB_NAME": "nightly"})
}

func newViewerFromDoc(t *testing.T, w, h int, path string, overrides map[string]string) Model {
	t.Helper()
	res, err := doc.Load(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	if res.HasErrors() {
		t.Fatalf("%s has errors", path)
	}
	m, _ := New(res, overrides).Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(Model)
}

func TestViewerLayout(t *testing.T) {
	m := newViewer(t, 120, 40)
	if !m.laidOut {
		t.Fatal("not laid out")
	}
	if m.out == nil {
		t.Fatal("no render output")
	}
	// The sidebar is a fixed 50-column column.
	if !m.outline.visible {
		t.Fatal("sidebar should be visible at 120 cols")
	}
	if m.outline.width != 50 {
		t.Fatalf("sidebar width = %d, want 50", m.outline.width)
	}
	if m.content.Width != 120-50-gutterWidth-railWidth {
		t.Fatalf("content width = %d", m.content.Width)
	}
	v := m.View()
	// The reading surface has no header row or viewport frame.
	if strings.Contains(v, "spec v1") {
		t.Fatal("header chrome leaked into the view")
	}
	if strings.Contains(v, "┌") || strings.Contains(v, "└") {
		t.Fatal("the viewport frame should be gone")
	}
	// The document itself is still there (the h1 renders in the content).
	if !strings.Contains(v, "exmd feature tour") {
		t.Fatal("document content missing")
	}
	// No hairline; the sidebar runs to the bottom with the path in its footer.
	lines := strings.Split(v, "\n")
	// The sidebar is painted from row 0, with one blank pad row before the
	// title text on row 1.
	if strings.TrimSpace(ansi.Strip(lines[0])) != "" {
		t.Fatalf("sidebar top pad row is not blank: %q", lines[0])
	}
	if !strings.Contains(ansi.Strip(lines[1]), "exmd feature tour") {
		t.Fatalf("sidebar title missing from row 1: %q", lines[1])
	}
	if strings.Contains(v, "─") {
		t.Fatal("footer separator should be gone")
	}
	footer := ansi.Strip(lines[len(lines)-2])
	if !strings.Contains(footer, "j/k scroll") {
		t.Fatalf("footer missing hints: %q", footer)
	}
	leftWidth := railWidth + m.content.Width + gutterWidth
	if got := string([]rune(footer)[leftWidth:]); !strings.HasPrefix(got, "  ../../examples/demo/demo.md") {
		t.Fatalf("sidebar footer path is not left-aligned: %q", got)
	}
	if last := ansi.Strip(lines[len(lines)-1]); strings.TrimSpace(last) != "" {
		t.Fatalf("missing bottom margin row: %q", last)
	}
	// Viewer viewport: pane height minus the content's two padding rows.
	if m.content.Height != 35 {
		t.Fatalf("viewer viewport height = %d, want 35", m.content.Height)
	}
	if m.paneHeight != 37 {
		t.Fatalf("pane height = %d, want 37", m.paneHeight)
	}
}

// TestSidebarSpansFullHeight checks that the sidebar surface reaches the
// bottom of the viewport, including its footer and bottom blank rows.
func TestSidebarSpansFullHeight(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	lines := strings.Split(m.View(), "\n")
	if len(lines) != m.height {
		t.Fatalf("view has %d rows, want %d", len(lines), m.height)
	}
	panel := "48;2;24;24;30" // theme.Dark.BackgroundPanel
	painted := 0
	for _, line := range lines {
		if strings.Contains(line, panel) {
			painted++
		}
	}
	// Every row except the selected title carries the panel shade.
	if painted < m.height-2 {
		t.Fatalf("sidebar panel covers %d of %d rows", painted, m.height)
	}
}

// TestFooterBandSurface checks that the left footer is painted on a surface
// lighter than the sidebar's raised shade.
func TestFooterBandSurface(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	lines := strings.Split(m.View(), "\n")
	footer := lines[len(lines)-2]
	want := termenv.TrueColor.
		Color(string(theme.Lift(theme.Dark.BackgroundElement, 0.12))).Sequence(true)
	if !strings.Contains(footer, want) {
		t.Fatalf("footer has no lifted surface (%s): %q", want, footer)
	}
}

func TestFooterIsContextAware(t *testing.T) {
	m := newViewer(t, 120, 40)
	footer := func(m Model) string {
		lines := strings.Split(m.View(), "\n")
		return ansi.Strip(lines[len(lines)-2])
	}

	lines := strings.Split(m.View(), "\n")
	if pad := ansi.Strip(lines[len(lines)-3]); strings.TrimSpace(pad) != "" {
		t.Fatalf("row above the footer is not blank: %q", pad)
	}
	if last := ansi.Strip(lines[len(lines)-1]); strings.TrimSpace(last) != "" {
		t.Fatalf("bottom row is not blank: %q", last)
	}

	// Viewer: scrolling hints, inset by two columns from the left edge.
	if f := footer(m); !strings.HasPrefix(f, "  ") ||
		strings.HasPrefix(f, "   ") || !strings.Contains(f, "j/k scroll") ||
		!strings.Contains(f, "examples/demo/demo.md") {
		t.Fatalf("viewer footer = %q", f)
	}

	// Outline focus: navigation + jump hints.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mm.(Model)
	if f := footer(m); !strings.Contains(f, "j/k navigate") ||
		!strings.Contains(f, "enter jump") {
		t.Fatalf("outline footer = %q", f)
	}

	// Block mode: block navigation + run hints.
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}})
	m = mm.(Model)
	if f := footer(m); !strings.Contains(f, "j/k blocks") ||
		!strings.Contains(f, "r run") {
		t.Fatalf("block footer = %q", f)
	}
}

func TestViewerNarrowHidesSidebar(t *testing.T) {
	m := newViewer(t, 80, 30)
	if m.outline.visible {
		t.Fatal("sidebar should be hidden below the minimum width")
	}
}

func TestViewerFocusAndNavigation(t *testing.T) {
	m := newViewer(t, 120, 40)

	// Above the first heading the unified cursor rests on the title row
	// (sel == -1). Tab focuses the sidebar, j moves to the next section and
	// jumps the content to it.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := mm.(Model)
	if got.focus != focusOutline {
		t.Fatal("tab did not move focus to the sidebar")
	}
	if got.outline.sel != -1 {
		t.Fatalf("initial sel = %d, want -1", got.outline.sel)
	}

	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = mm.(Model)
	if got.outline.sel != 0 {
		t.Fatalf("sel = %d, want 0", got.outline.sel)
	}
	wantY := got.outline.entries[0].renderLine - 1
	if got.content.YOffset != wantY {
		t.Fatalf("YOffset = %d, want %d", got.content.YOffset, wantY)
	}

	// k at the first entry moves up onto the title row and returns to the
	// very top of the document.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = mm.(Model)
	if got.outline.sel != -1 {
		t.Fatalf("sel after k = %d, want -1 (title)", got.outline.sel)
	}
	if got.content.YOffset != 0 {
		t.Fatalf("YOffset after k = %d, want 0 (top of document)", got.content.YOffset)
	}

	// t switches to the Blocks tab.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	got = mm.(Model)
	if got.outline.tab != tabBlocks {
		t.Fatal("t did not switch to blocks")
	}
	if len(got.outline.entries) != len(got.doc.Blocks) {
		t.Fatalf("blocks entries = %d, want %d", len(got.outline.entries), len(got.doc.Blocks))
	}

	// Scroll-spy: jumping back to the content focus keeps one cursor.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = mm.(Model)
	if got.focus != focusContent {
		t.Fatal("tab did not return focus to the content")
	}
}

// TestListWalkNoSnapping walks the sidebar entry by entry and asserts the
// cursor advances exactly one step per keypress. Near the document's end
// the content viewport pins at the bottom; the scroll-spy must not
// override cursor-driven moves and snap the selection to the last entry.
func TestListWalkNoSnapping(t *testing.T) {
	m := newViewer(t, 120, 60)
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := mm.(Model)
	prev := got.outline.sel
	for i := 0; i < len(got.outline.entries); i++ {
		mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		got = mm.(Model)
		if got.outline.sel != prev+1 {
			t.Fatalf("step %d: sel = %d, want %d (cursor snapped)", i, got.outline.sel, prev+1)
		}
		prev = got.outline.sel
	}
	// One past the end: the cursor stays.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = mm.(Model)
	if got.outline.sel != len(got.outline.entries)-1 {
		t.Fatalf("sel past the end = %d, want %d", got.outline.sel, len(got.outline.entries)-1)
	}
}

// TestViewerSpyFollowsScroll checks the scroll-driven direction of the
// unified cursor: scrolling the content to the bottom highlights the last
// entry (the first h1 is the title row and is not among the entries).
func TestViewerSpyFollowsScroll(t *testing.T) {
	m := newViewer(t, 120, 40)
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got := mm.(Model)
	// At the bottom of the document the current heading is the last one;
	// the first h1 is the title row and is not among the entries.
	if got.outline.sel != len(got.outline.entries)-1 {
		t.Fatalf("sel = %d, want last entry", got.outline.sel)
	}
	if !got.content.AtBottom() {
		t.Fatal("G did not reach the bottom")
	}
}

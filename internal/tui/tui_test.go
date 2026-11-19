package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/michaelishri/md-exec/internal/doc"
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
	if m.content.Width != 120-50-gutterWidth-railWidth-1 {
		t.Fatalf("content width = %d", m.content.Width)
	}
	v := m.View()
	// The reading surface is content + sidebar only: no header row (title,
	// path, chips), no footer, no viewport frame.
	if strings.Contains(v, "spec v1") || strings.Contains(v, "examples/demo/demo.md") {
		t.Fatal("header chrome leaked into the view")
	}
	if strings.Contains(v, "j/k scroll") || strings.Contains(v, "q quit") {
		t.Fatal("footer should be gone")
	}
	if strings.Contains(v, "┌") || strings.Contains(v, "└") {
		t.Fatal("the viewport frame should be gone")
	}
	// The document itself is still there (the h1 renders in the content).
	if !strings.Contains(v, "md-exec feature tour") {
		t.Fatal("document content missing")
	}
	// The panes float: one blank row above and below the body.
	lines := strings.Split(v, "\n")
	if strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Fatalf("missing blank margin rows: first=%q last=%q", lines[0], lines[len(lines)-1])
	}
	// Viewer viewport: the pane height minus the content's two padding
	// rows; the panes themselves span the full pane height.
	if m.content.Height != 36 {
		t.Fatalf("viewer viewport height = %d, want 36", m.content.Height)
	}
	if m.paneHeight != 38 {
		t.Fatalf("pane height = %d, want 38", m.paneHeight)
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

	// Above the first heading the unified cursor rests on no entry: the
	// h1 is the title row and is not selectable. Tab focuses the sidebar,
	// j moves to the next section and jumps the content to it.
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

	// k at the first entry stays put.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = mm.(Model)
	if got.outline.sel != 0 {
		t.Fatalf("sel after k = %d, want 0", got.outline.sel)
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

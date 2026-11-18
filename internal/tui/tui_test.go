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
	// Sidebar is a quarter of the window, clamped to [24, 40]; 120/4 = 30.
	if !m.outline.visible {
		t.Fatal("sidebar should be visible at 120 cols")
	}
	if m.outline.width != 30 {
		t.Fatalf("sidebar width = %d, want 30", m.outline.width)
	}
	if m.content.Width != 120-30-gutterWidth {
		t.Fatalf("content width = %d", m.content.Width)
	}
	v := m.View()
	if !strings.Contains(v, "md-exec feature tour") {
		t.Fatal("title missing from header")
	}
	if !strings.Contains(v, "Outline") {
		t.Fatal("sidebar title missing")
	}
	if !strings.Contains(v, "spec v1") {
		t.Fatal("spec chip missing")
	}
	// 3/4 pane: content starts at the left edge, no borders anywhere.
	if strings.Contains(v, "┌") || strings.Contains(v, "└") {
		t.Fatal("unexpected box borders — the look is borderless")
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

	// Above the first heading the unified cursor sits on heading 0 (the
	// document title renders at line 0); tab focuses the sidebar, j moves to
	// the next section and jumps the content to it.
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := mm.(Model)
	if got.focus != focusOutline {
		t.Fatal("tab did not move focus to the sidebar")
	}
	if got.outline.sel != 0 {
		t.Fatalf("initial sel = %d, want 0", got.outline.sel)
	}

	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = mm.(Model)
	if got.outline.sel != 1 {
		t.Fatalf("sel = %d, want 1", got.outline.sel)
	}
	wantY := got.outline.entries[1].renderLine - 1
	if got.content.YOffset != wantY {
		t.Fatalf("YOffset = %d, want %d", got.content.YOffset, wantY)
	}

	// k moves back.
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

func TestViewerSpyFollowsScroll(t *testing.T) {
	m := newViewer(t, 120, 40)
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got := mm.(Model)
	// At the bottom of the document the current heading is the last one.
	if got.outline.sel != len(got.doc.Headings)-1 {
		t.Fatalf("sel = %d, want last heading", got.outline.sel)
	}
	if !got.content.AtBottom() {
		t.Fatal("G did not reach the bottom")
	}
}

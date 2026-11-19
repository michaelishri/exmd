package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// focusSidebarAndSelectFirstHeading tabs to the outline and moves the
// cursor to the first entry.
func focusSidebarAndSelectFirstHeading(t *testing.T, m Model) Model {
	t.Helper()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm, _ = mm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	return mm.(Model)
}

// enterBlockModeFromFirstHeading selects the first heading in the sidebar
// and presses enter, starting executable-block selection at the demo's
// first code block.
func enterBlockModeFromFirstHeading(t *testing.T, m Model) Model {
	t.Helper()
	got := focusSidebarAndSelectFirstHeading(t, m)
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return mm.(Model)
}

// TestEnterSelectsFirstBlock checks that enter on a highlighted heading
// starts executable-block selection at the first code block at or after
// that heading, and moves focus to the text area.
func TestEnterSelectsFirstBlock(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := focusSidebarAndSelectFirstHeading(t, m)
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = mm.(Model)

	if got.blockSel != 0 {
		t.Fatalf("blockSel = %d, want 0 (the first block at or after the heading)", got.blockSel)
	}
	if got.focus != focusContent {
		t.Fatal("enter should move focus to the text area")
	}
}

// TestArrowsMoveBlocks checks that the arrows walk the executable blocks
// with clamps at the ends, and g/G jump to the first and last.
func TestArrowsMoveBlocks(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)

	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	if got.blockSel != 1 {
		t.Fatalf("blockSel after down = %d, want 1", got.blockSel)
	}
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	if got.blockSel != 2 {
		t.Fatalf("blockSel after down = %d, want 2", got.blockSel)
	}

	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyUp})
	got = mm.(Model)
	if got.blockSel != 1 {
		t.Fatalf("blockSel after up = %d, want 1", got.blockSel)
	}

	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got = mm.(Model)
	if got.blockSel != len(got.doc.Blocks)-1 {
		t.Fatalf("blockSel after G = %d, want the last", got.blockSel)
	}

	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = mm.(Model)
	if got.blockSel != 0 {
		t.Fatalf("blockSel after g = %d, want 0", got.blockSel)
	}

	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyUp})
	got = mm.(Model)
	if got.blockSel != 0 {
		t.Fatalf("blockSel after up at the start = %d, want 0", got.blockSel)
	}
}

// TestBlockRailSpansBlock checks that a selected code block's rail bar
// spans its whole extent, starting at the badge's row (the first row under
// the content's top padding).
func TestBlockRailSpansBlock(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)

	bm := got.out.Blocks[0]
	// The bar covers the code block (starting below its metadata row),
	// not the badge line above it.
	start := bm.Line + 1 - got.content.YOffset + 1
	end := bm.End - got.content.YOffset + 1
	visible := 0
	for r := 0; r < got.paneHeight; r++ {
		if r >= start && r < end {
			visible++
		}
	}

	v := got.View()
	if n := strings.Count(v, "▌"); n != visible {
		t.Fatalf("rail bar rows = %d, want %d (the block's visible extent)", n, visible)
	}
	// The badge's row carries no rail bar.
	lines := strings.Split(v, "\n")
	badgePane := bm.Line - got.content.YOffset + 1
	if badgePane >= 0 && badgePane < got.paneHeight &&
		strings.Contains(ansi.Strip(lines[1+badgePane]), "▌") {
		t.Fatal("rail should start below the metadata row")
	}
	// The first bar row is the block's first shaded row.
	if start >= 0 && start < got.paneHeight {
		if !strings.Contains(ansi.Strip(lines[1+start]), "▌") {
			t.Fatalf("rail bar missing at the code block's first row: %q", ansi.Strip(lines[1+start]))
		}
	}
}

// TestEscapeReturnsToHeading checks that escape leaves executable-block
// selection and highlights the heading the block belongs to.
func TestEscapeReturnsToHeading(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)

	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = mm.(Model)
	if got.blockSel != -1 {
		t.Fatalf("blockSel = %d, want -1 after esc", got.blockSel)
	}
	if got.outline.sel != 0 {
		t.Fatalf("outline.sel = %d, want 0 (the block's heading highlighted again)", got.outline.sel)
	}
}

// TestRunSelectedBlockInline checks that r executes the highlighted block
// in place, appending its output under the code rather than switching to
// run mode.
func TestRunSelectedBlockInline(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)

	mm, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got = mm.(Model)
	if got.mode != modeView {
		t.Fatal("inline execution should stay in the document view")
	}
	got = pump(t, got, cmd)

	if !got.inline.done {
		t.Fatal("inline run did not finish")
	}
	if got.inline.err != nil || (got.inline.result != nil && got.inline.result.Failed != nil) {
		t.Fatalf("inline run failed: %v / %+v", got.inline.err, got.inline.result)
	}
	joined := strings.Join(got.inline.lines, "\n")
	if !strings.Contains(joined, "author:  Michael Ishri") {
		t.Fatalf("block output missing: %q", joined)
	}

	// The output box is spliced right after the block's last line: a
	// separator, then the output rows.
	lines := got.contentLines()
	bm := got.out.Blocks[0]
	if len(lines) != len(got.out.Lines)+len(got.inline.box) {
		t.Fatalf("content lines = %d, want %d", len(lines), len(got.out.Lines)+len(got.inline.box))
	}
	if !strings.Contains(ansi.Strip(lines[bm.End]), "─") {
		t.Fatalf("no separator at the block's end: %q", ansi.Strip(lines[bm.End]))
	}
	if !strings.Contains(ansi.Strip(lines[bm.End+1]), "author:") {
		t.Fatalf("output not directly under the separator: %q", ansi.Strip(lines[bm.End+1]))
	}
}

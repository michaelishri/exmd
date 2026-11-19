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
	// The badge's pane row: viewport row (bm.Line - YOffset) plus the
	// content's top padding row. The bar covers the block's visible
	// extent.
	start := bm.Line - got.content.YOffset + 1
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
	// The first bar row is the badge's row.
	lines := strings.Split(v, "\n")
	if start >= 0 && start < got.paneHeight {
		if !strings.Contains(ansi.Strip(lines[1+start]), "▌") {
			t.Fatalf("rail bar missing at the badge's row: %q", ansi.Strip(lines[1+start]))
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

// TestRunSelectedBlock checks that r on a highlighted block runs exactly
// that block in run mode.
func TestRunSelectedBlock(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)

	// Move to the second block and run it.
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	mm, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got = mm.(Model)
	if got.mode != modeRun {
		t.Fatal("r did not enter run mode")
	}
	got = pump(t, got, cmd)

	if got.run.result == nil || got.run.result.Failed != nil {
		t.Fatalf("run failed: %+v", got.run.result)
	}
	if got.run.result.BlocksRun != 1 {
		t.Fatalf("blocks run = %d, want 1 (the selected block only)", got.run.result.BlocksRun)
	}
	if !got.run.blocks[1].hasEnd {
		t.Fatal("the selected block did not run")
	}
	if got.run.blocks[0].hasEnd {
		t.Fatal("an unselected block ran")
	}
}

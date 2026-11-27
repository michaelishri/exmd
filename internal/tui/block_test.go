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
// and presses backtick, starting executable-block selection at the demo's
// first code block.
func enterBlockModeFromFirstHeading(t *testing.T, m Model) Model {
	t.Helper()
	got := focusSidebarAndSelectFirstHeading(t, m)
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}})
	return mm.(Model)
}

// TestBacktickSelectsFirstBlock checks that backtick on a highlighted heading
// starts executable-block selection at the first code block at or after
// that heading, and moves focus to the text area.
func TestBacktickSelectsFirstBlock(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := focusSidebarAndSelectFirstHeading(t, m)
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}})
	got = mm.(Model)

	if got.blockSel != 0 {
		t.Fatalf("blockSel = %d, want 0 (the first block at or after the heading)", got.blockSel)
	}
	if got.focus != focusContent {
		t.Fatal("backtick should move focus to the text area")
	}
}

// TestArrowsMoveBlocks checks that the arrows walk the executable blocks
// with clamps at the ends, and g/G jump to the first and last.
// TestEnterJumpsToHeading checks that enter keeps its original meaning
// instead of activating code-block selection.
func TestEnterJumpsToHeading(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := focusSidebarAndSelectFirstHeading(t, m)
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = mm.(Model)
	if got.blockSel != -1 {
		t.Fatalf("enter should not activate block mode (blockSel = %d)", got.blockSel)
	}
}

func TestArrowsMoveBlocks(t *testing.T) {
	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)

	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	if got.blockSel != 1 {
		t.Fatalf("blockSel after down = %d, want 1", got.blockSel)
	}
	if got.outline.sel != got.headingForBlock() {
		t.Fatalf("sidebar sel = %d, want the selected block's heading %d",
			got.outline.sel, got.headingForBlock())
	}
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	if got.blockSel != 2 {
		t.Fatalf("blockSel after down = %d, want 2", got.blockSel)
	}
	if got.outline.sel != got.headingForBlock() {
		t.Fatalf("sidebar sel = %d, want the selected block's heading %d",
			got.outline.sel, got.headingForBlock())
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
		strings.Contains(ansi.Strip(lines[badgePane]), "▌") {
		t.Fatal("rail should start below the metadata row")
	}
	// The first bar row is the block's first shaded row.
	if start >= 0 && start < got.paneHeight {
		if !strings.Contains(ansi.Strip(lines[start]), "▌") {
			t.Fatalf("rail bar missing at the code block's first row: %q", ansi.Strip(lines[start]))
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
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)

	mm, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = mm.(Model)
	// While running, the gutter bar is orange.
	if !strings.Contains(got.View(), got.styles.railRunActive.Render("▌")) {
		t.Fatal("running block's rail should be bright orange")
	}
	got = pump(t, got, cmd)

	if !got.inline[0].done {
		t.Fatal("inline run did not finish")
	}
	if got.inline[0].err != nil || (got.inline[0].result != nil && got.inline[0].result.Failed != nil) {
		t.Fatalf("inline run failed: %v / %+v", got.inline[0].err, got.inline[0].result)
	}
	joined := strings.Join(got.inline[0].lines, "\n")
	if !strings.Contains(joined, "author:  Michael Ishri") {
		t.Fatalf("block output missing: %q", joined)
	}

	// The output box is spliced right after the block's last line: a
	// separator, then the output rows.
	lines := got.contentLines()
	bm := got.out.Blocks[0]
	if len(lines) != len(got.out.Lines)+len(got.inline[0].box) {
		t.Fatalf("content lines = %d, want %d", len(lines), len(got.out.Lines)+len(got.inline[0].box))
	}
	if !strings.Contains(ansi.Strip(lines[bm.End]), "─") {
		t.Fatalf("no separator at the block's end: %q", ansi.Strip(lines[bm.End]))
	}
	if !strings.Contains(ansi.Strip(lines[bm.End+1]), "author:") {
		t.Fatalf("output not directly under the separator: %q", ansi.Strip(lines[bm.End+1]))
	}
	// Output text is indented like the block's code, and the box closes
	// with one empty row.
	leading := func(s string) int {
		v := ansi.Strip(s)
		return len(v) - len(strings.TrimLeft(v, " "))
	}
	var codeLine string
	for i := bm.Line + 1; i < bm.End; i++ {
		if strings.TrimSpace(ansi.Strip(got.out.Lines[i])) != "" {
			codeLine = got.out.Lines[i]
			break
		}
	}
	if codeLine == "" || leading(lines[bm.End+1]) != leading(codeLine) {
		t.Fatalf("output indent %d does not match code indent %d",
			leading(lines[bm.End+1]), leading(codeLine))
	}
	last := lines[bm.End+len(got.inline[0].box)-1]
	if strings.TrimSpace(ansi.Strip(last)) != "" {
		t.Fatalf("no empty last row in the output box: %q", ansi.Strip(last))
	}
	// On success, the rail turns green.
	if !strings.Contains(got.View(), got.styles.railDoneActive.Render("▌")) {
		t.Fatal("successful block's rail should be bright green")
	}

	// Moving the selection elsewhere leaves both the output and the
	// executed block's status bar in place.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	if !strings.Contains(strings.Join(got.contentLines(), "\n"), "author:  Michael Ishri") {
		t.Fatal("output disappeared when moving to another block")
	}
	if !strings.Contains(got.View(), got.styles.railDone.Render("▌")) {
		t.Fatal("status bar disappeared when moving to another block")
	}
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = mm.(Model)
	if !strings.Contains(got.View(), got.styles.railDone.Render("▌")) {
		t.Fatal("status bar disappeared when returning to heading selection")
	}
}

// TestInlineOutputsPersistAcrossRuns checks that running a second block
// does not reset the first block's output or its green status bar.
func TestInlineOutputsPersistAcrossRuns(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m) // selects block 0

	mm, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = pump(t, mm.(Model), cmd)
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown}) // block 1
	got = mm.(Model)
	mm, cmd = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = pump(t, mm.(Model), cmd)

	content := strings.Join(got.contentLines(), "\n")
	if !strings.Contains(content, "author:  Michael Ishri") {
		t.Fatal("first block's output was lost")
	}
	if !strings.Contains(content, "inside this block:") {
		t.Fatal("second block's output is missing")
	}
	if want := len(got.out.Lines) + len(got.inline[0].box) + len(got.inline[1].box); len(got.contentLines()) != want {
		t.Fatalf("content lines = %d, want %d (both boxes spliced)",
			len(got.contentLines()), want)
	}

	// Both blocks still carry their green status bar, at their own
	// display positions (after the earlier box is accounted for).
	viewLines := strings.Split(got.View(), "\n")
	for _, index := range []int{0, 1} {
		wantStyle := got.styles.railDone
		if index == got.blockSel {
			wantStyle = got.styles.railDoneActive
		}
		bm := got.out.Blocks[index]
		start := got.displayLine(bm.Line+1) - got.content.YOffset + 1
		end := got.displayLine(bm.End) - got.content.YOffset + 1
		found := false
		for r := start; r < end && r < got.paneHeight && r+1 < len(viewLines); r++ {
			if strings.Contains(viewLines[r], wantStyle.Render("▌")) {
				found = true
			}
		}
		if !found {
			t.Fatalf("block %d's status bar is missing", index)
		}
	}
}

// TestInlineStateCarriesBetweenBlocks checks that harvested globals survive
// between inline runs in one session (SPEC.md §4.3): step 3 exports WORK,
// step 9 reads it back.
func TestInlineStateCarriesBetweenBlocks(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)
	// Move to step 3's block (index 3): WORK="$(mktemp -d)"; export WORK.
	for i := 0; i < 3; i++ {
		mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyDown})
		got = mm.(Model)
	}
	if got.blockSel != 3 {
		t.Fatalf("blockSel = %d, want 3", got.blockSel)
	}
	mm, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = pump(t, mm.(Model), cmd)
	if !strings.Contains(strings.Join(got.inline[3].lines, "\n"), "workspace:") {
		t.Fatalf("step 3 output missing: %v", got.inline[3].lines)
	}

	// Jump to the last block (step 9) and run it: it echoes WORK.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got = mm.(Model)
	mm, cmd = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = pump(t, mm.(Model), cmd)

	out := strings.Join(got.inline[len(got.doc.Blocks)-1].lines, "\n")
	if strings.Contains(out, "<unset>") || !strings.Contains(out, "workspace: /") {
		t.Fatalf("WORK did not carry from step 3 to step 9: %q", out)
	}
}

// TestInlineLastBlockOutputIsVisible checks that streaming output from the
// final block scrolls into view as it arrives.
func TestInlineLastBlockOutputIsVisible(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewer(t, 120, 40)
	got := enterBlockModeFromFirstHeading(t, m)
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got = mm.(Model)
	index := len(got.doc.Blocks) - 1
	if got.blockSel != index {
		t.Fatalf("blockSel = %d, want the last block %d", got.blockSel, index)
	}

	mm, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = pump(t, mm.(Model), cmd)
	if !got.inline[index].done {
		t.Fatal("last block's inline run did not finish")
	}

	bm := got.out.Blocks[index]
	lastRow := got.displayLine(bm.End-1) + len(got.inline[index].box)
	if got.content.YOffset+got.content.Height <= lastRow {
		t.Fatalf("output not visible: offset=%d height=%d last row=%d",
			got.content.YOffset, got.content.Height, lastRow)
	}
}

// TestInlineFailureRailTurnsRed checks that a failing inline run marks the
// block's gutter bar red.
func TestInlineFailureRailTurnsRed(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)

	m := newViewerFromDoc(t, 120, 40, "../../internal/exec/testdata/fail.md", nil)
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := mm.(Model)
	// The whole short document fits on screen, so the spy starts at the
	// last heading; move to the first one explicitly.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = mm.(Model)
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}})
	got = mm.(Model)
	// Down to the second block, which exits non-zero.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	mm, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = mm.(Model)
	got = pump(t, got, cmd)

	if got.inline[1].status == 0 {
		t.Fatalf("expected a failing block, status = %d", got.inline[1].status)
	}
	if !strings.Contains(got.View(), got.styles.railFailActive.Render("▌")) {
		t.Fatal("failed block's rail should be bright red")
	}
}

// Package tui implements the md-exec viewer: the rendered runbook on the
// left, the document outline as a shaded 50-column sidebar on the right,
// and a footer with context-aware shortcuts and the file path. The visual
// language follows the OpenCode TUI — borderless, separated by background
// shades, color reserved for state. Executable blocks run in place, with
// their output streaming into the document beneath them.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/md-exec/internal/doc"
	"github.com/michaelishri/md-exec/internal/exec"
	"github.com/michaelishri/md-exec/internal/render"
	"github.com/michaelishri/md-exec/internal/theme"
)

const (
	minSidebarWidth = 90
	sidebarCols     = 50
	gutterWidth     = 2
	railWidth       = 1
	rightMargin     = 1
	minViewWidth    = 20
	minViewHeight   = 4
	footerRows      = 3
	paneInsets      = 2
	minContentWidth = 10
)

type focusArea int

const (
	focusContent focusArea = iota
	focusOutline
)

// Model is the bubbletea model of the viewer.
type Model struct {
	doc *doc.Document

	width, height int
	laidOut       bool

	out     *render.Output
	content viewport.Model
	outline outlineModel
	focus   focusArea

	// Cached row strings. Built on resize, reused on every View.
	sep, blankRow, gutterRow string
	shifts                   []int // inserted box lines per rendered line

	overrides map[string]string
	theme     theme.Theme

	// blockSel is the executable block highlighted in the text area
	// (index into the document's blocks); -1 when block selection is off.
	blockSel int

	// inline keeps a persistent output box and status per executed block.
	inline map[int]*inlineState

	// inlineSession carries the harvested global state between inline
	// runs (SPEC.md §4.3): one session for all blocks in this document.
	inlineSession *exec.Session

	// paneHeight is the height of the body panes (rail, gutter, sidebar,
	// content block including its padding rows); the viewport itself shows
	// paneHeight minus the content's two padding rows.
	paneHeight int
}

// New builds the model for a loaded document; overrides are launch-time
// values for declared globals (SPEC.md §2.2).
func New(res *doc.Result, overrides map[string]string) Model {
	m := Model{
		doc:       res.Doc,
		content:   viewport.New(0, 0),
		outline:   newOutline(),
		focus:     focusContent,
		overrides: overrides,
		theme:     t,
		blockSel:  -1,
		inline:    map[int]*inlineState{},
	}
	m.content.MouseWheelEnabled = true
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("md-exec — " + m.doc.HeadingText())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case inlineEventMsg:
		m.applyInlineEvent(msg.index, msg.ev)
		if m.inlineActive() {
			return m, m.inlineListenCmd(m.stateFor(msg.index), msg.index)
		}
		return m, nil

	case inlineDoneMsg:
		m.finalizeInline(msg.index, msg.result)
		return m, nil

	case tea.KeyMsg:
		if cmd, handled := m.handleKey(msg); handled {
			return m, cmd
		}

	case tea.MouseMsg:
		prev := m.content.YOffset
		m.content, _ = m.content.Update(msg)
		if m.content.YOffset != prev {
			m.updateSpy()
		}
		return m, nil
	}

	if m.focus == focusContent {
		prev := m.content.YOffset
		var cmd tea.Cmd
		m.content, cmd = m.content.Update(msg)
		if m.content.YOffset != prev {
			m.updateSpy()
		}
		return m, cmd
	}
	return m, nil
}

// handleKey dispatches a key to the global, outline or block handler. It
// reports whether the key was consumed.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if cmd, handled := m.handleGlobalKey(msg); handled {
		return cmd, true
	}
	if m.focus == focusOutline {
		return m.handleOutlineKey(msg)
	}
	return m.handleBlockKey(msg)
}

// handleGlobalKey processes keys that work in every focus area.
func (m *Model) handleGlobalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.shutdownInline()
		return tea.Quit, true

	case "r":
		if m.inlineActive() || m.blockSel < 0 {
			return nil, true
		}
		// Execute the highlighted block in place, under the code.
		return m.beginInlineRun(), true

	case "s":
		// §4.7: the engine terminates the current process group.
		for _, st := range m.inline {
			if st != nil && st.active && st.cancel != nil {
				st.cancel()
			}
		}
		return nil, true

	case "v", "esc":
		if m.blockSel >= 0 {
			// Leave block selection. The heading is highlighted again.
			m.exitBlockMode()
		}
		return nil, true

	case "tab":
		if m.focus == focusContent && m.outline.visible {
			m.focus = focusOutline
		} else {
			m.focus = focusContent
		}
		return nil, true

	case "shift+tab":
		m.focus = focusContent
		return nil, true

	case "t":
		if m.out == nil {
			return nil, true
		}
		if m.outline.tab == tabHeadings {
			m.outline.tab = tabBlocks
		} else {
			m.outline.tab = tabHeadings
		}
		m.outline.rebuild(m.doc, m.out, m.outline.width)
		m.jumpToSelected()
		return nil, true
	}
	return nil, false
}

// handleOutlineKey moves the sidebar selection. The outline consumes every
// key while it has focus.
func (m *Model) handleOutlineKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		if m.outline.move(1) {
			m.jumpToSelected()
		}
	case "k", "up":
		if m.outline.move(-1) {
			m.jumpToSelected()
		}
	case "g":
		if m.outline.first() {
			m.jumpToSelected()
		}
	case "G":
		if m.outline.last() {
			m.jumpToSelected()
		}
	case "enter":
		// Enter jumps the content to the highlighted heading.
		m.jumpToSelected()
	case "`":
		// Enter block selection at the highlighted heading.
		m.enterBlockMode()
	}
	return nil, true
}

// handleBlockKey walks executable blocks, or scrolls the content when no
// block is selected.
func (m *Model) handleBlockKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.blockSel >= 0 {
		switch msg.String() {
		case "down", "j":
			if m.blockSel < len(m.out.Blocks)-1 {
				m.selectBlock(m.blockSel + 1)
			}
			return nil, true
		case "up", "k":
			if m.blockSel > 0 {
				m.selectBlock(m.blockSel - 1)
			}
			return nil, true
		case "g":
			m.selectBlock(0)
			return nil, true
		case "G":
			m.selectBlock(len(m.out.Blocks) - 1)
			return nil, true
		}
	}

	// Backtick selects the first executable block at or after the
	// sidebar's highlighted heading.
	if m.blockSel < 0 && msg.String() == "`" && len(m.out.Blocks) > 0 {
		m.enterBlockMode()
		return nil, true
	}

	switch msg.String() {
	case "g":
		m.content.GotoTop()
		m.updateSpy()
		return nil, true
	case "G":
		m.content.GotoBottom()
		m.updateSpy()
		return nil, true
	}
	return nil, false // forwarded to the viewport
}

// resize relayouts the window and re-renders at the new width, re-anchoring
// the scroll position to the entry the reader was on.
func (m *Model) resize(w, h int) {
	m.width, m.height = w, h
	m.relayout()
}

func (m *Model) relayout() {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		return
	}

	m.outline.visible = w >= minSidebarWidth
	sidebarW := 0
	if m.outline.visible {
		sidebarW = sidebarCols
	}
	contentW := w
	if m.outline.visible {
		contentW = w - sidebarW - gutterWidth - railWidth - rightMargin
	}
	contentW = max(minContentWidth, contentW)

	// One blank row above the panes, then the hairline + footer.
	bodyH := max(1, h-footerRows)
	m.paneHeight = bodyH
	m.blankRow = " "
	m.gutterRow = strings.Repeat(" ", gutterWidth-1) + sideBorderStyle.Render("│")
	m.sep = sepStyle.Render(strings.Repeat("─", max(1, w)))

	m.content.Width = contentW
	// The text area carries 1 padding row at its top and bottom — the
	// same insets as the sidebar — so the viewport shows the body
	// height minus those two rows.
	m.content.Height = max(1, bodyH-paneInsets)

	m.out = render.Render(m.doc, contentW, m.colorEnabled())

	if m.out != nil {
		for index := range m.inline {
			m.rebuildInlineBox(index)
		}
		m.setContent()
		m.outline.rebuild(m.doc, m.out, sidebarW)
	}

	if m.blockSel >= 0 {
		m.revealBlock()
	} else if m.laidOut && len(m.outline.entries) > 0 {
		sel := clamp(m.outline.sel, -1, len(m.outline.entries)-1)
		m.outline.sel = sel
		m.jumpToSelected()
	} else {
		m.content.SetYOffset(0)
		m.updateSpy()
	}
	m.laidOut = true
}

// jumpToSelected scrolls the content so the selected sidebar entry sits at
// the top of the viewport. It is cursor-driven: the scroll-spy must not
// re-derive the selection from the resulting scroll position (near the
// document's end the viewport pins at the bottom, which would otherwise
// snap the cursor to the last entry).
func (m *Model) jumpToSelected() {
	if m.out == nil {
		return
	}
	if m.outline.sel < 0 {
		// The title row: jump to the very top of the document.
		m.content.SetYOffset(0)
		return
	}
	if m.outline.sel >= len(m.outline.entries) {
		return
	}
	y := clamp(m.displayLine(m.outline.entries[m.outline.sel].renderLine)-1, 0, m.maxScroll())
	m.content.SetYOffset(y)
}

// enterBlockMode selects the first executable block at or after the
// sidebar's highlighted heading — the document's first when none follows —
// and moves focus to the text area, where the arrows walk the blocks.
// Nothing scrolls: the indicator moves to the block where it stands.
func (m *Model) enterBlockMode() {
	if m.out == nil || len(m.out.Blocks) == 0 {
		return
	}
	m.blockSel = 0
	if m.outline.sel >= 0 && m.outline.sel < len(m.outline.entries) {
		line := m.outline.entries[m.outline.sel].renderLine
		for i, b := range m.out.Blocks {
			if b.Line >= line {
				m.blockSel = i
				break
			}
		}
	}
	m.focus = focusContent
	m.selectBlock(m.blockSel)
}

// selectBlock moves the block cursor, reveals it when necessary, and keeps
// the sidebar highlight on the heading whose section owns that block.
func (m *Model) selectBlock(index int) {
	m.blockSel = index
	m.revealBlock()
	if heading := m.headingForBlock(); heading >= 0 {
		m.outline.sel = heading
	}
}

// exitBlockMode leaves executable-block selection and highlights the
// heading the selected block belongs to.
func (m *Model) exitBlockMode() {
	idx := m.headingForBlock()
	m.blockSel = -1
	if idx >= 0 {
		m.outline.sel = idx
	}
	m.jumpToSelected()
}

// headingForBlock returns the outline entry index of the heading containing
// the selected block (the last heading at or above its badge line), or -1
// when the block sits in the title's section.
func (m *Model) headingForBlock() int {
	if !m.validBlock(m.blockSel) {
		return -1
	}
	line := m.out.Blocks[m.blockSel].Line
	idx := -1
	for i, e := range m.outline.entries {
		if e.renderLine <= line {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// revealBlock scrolls the content only as far as needed to bring the
// selected block into view — if it is already fully visible, nothing
// moves: the indicator travels, the text stays put.
func (m *Model) revealBlock() {
	if !m.validBlock(m.blockSel) {
		return
	}
	bm := m.out.Blocks[m.blockSel]
	top := m.content.YOffset
	bottom := top + m.content.Height
	// Compare in display coordinates, accounting for an inline output box
	// inserted earlier in the document.
	line, end := m.displayLine(bm.Line), m.displayLine(bm.End)
	switch {
	case line >= top && end <= bottom:
		return // already fully visible
	case line < top:
		// Above the viewport: pin its badge to the top.
		m.content.SetYOffset(clamp(line, 0, m.maxScroll()))
	default:
		// Below the viewport: pin its last line to the bottom.
		m.content.SetYOffset(clamp(end-m.content.Height, 0, m.maxScroll()))
	}
}

func (m *Model) maxScroll() int {
	if m.out == nil {
		return 0
	}
	return max(0, len(m.contentLines())-m.content.Height)
}

// updateSpy mirrors the content scroll position into the sidebar cursor:
// one unified "current position" (SPEC.md §4.8 observation, applied to
// navigation).
func (m *Model) updateSpy() {
	if m.out == nil {
		return
	}
	off := m.content.YOffset + 1
	idx := -1
	if m.outline.tab == tabHeadings {
		// Entries carry the rendered line of their heading. The first h1
		// has no entry, so nothing is selected in its section.
		lines := make([]int, len(m.outline.entries))
		for i, e := range m.outline.entries {
			lines[i] = m.displayLine(e.renderLine)
		}
		idx = lastAtOrBefore(lines, off)
		if m.content.AtBottom() && len(lines) > 0 {
			idx = len(lines) - 1
		}
	} else {
		lines := make([]int, len(m.out.Blocks))
		for i, b := range m.out.Blocks {
			lines[i] = m.displayLine(b.Line)
		}
		idx = lastAtOrBefore(lines, off)
		if m.content.AtBottom() && len(lines) > 0 {
			idx = len(lines) - 1
		}
	}
	m.outline.sel = idx
}

// lastAtOrBefore returns the index of the last value <= off, or -1.
func lastAtOrBefore(values []int, off int) int {
	idx := -1
	for i, v := range values {
		if v > off {
			break
		}
		idx = i
	}
	return idx
}

// View renders the full screen: blank row, body, hairline, footer.
func (m Model) View() string {
	if !m.laidOut {
		return "Loading…"
	}
	if m.width < minViewWidth || m.height < minViewHeight {
		return m.content.View()
	}
	var sb strings.Builder
	// The viewer floats the panes with a blank row above, then a hairline
	// and footer: context-aware shortcuts on the left, file path on the
	// right.
	sb.WriteByte('\n')
	sb.WriteString(m.bodyView())
	sb.WriteByte('\n')
	sb.WriteString(m.sepView())
	sb.WriteByte('\n')
	sb.WriteString(m.footerView())
	return sb.String()
}

// footerView is the bottom line: shortcuts for the current context on the
// left, the runbook's file path on the right.
func (m Model) footerView() string {
	return composeLeftRight(m.keyHints(), infoStyle.Render(m.doc.Path), m.width)
}

// keyHints reports only the keys relevant to the current context.
func (m Model) keyHints() string {
	var hints []string
	switch {
	case m.inlineActive():
		hints = []string{"s stop", "q quit"}
	case m.blockSel >= 0:
		hints = []string{"j/k blocks", "r run", "s stop", "esc back", "q quit"}
	case m.focus == focusOutline:
		hints = []string{"j/k navigate", "enter jump", "` blocks", "tab content", "q quit"}
	default:
		hints = []string{"j/k scroll", "tab outline", "` blocks", "r run", "q quit"}
	}
	return infoStyle.Render(strings.Join(hints, " · "))
}

func (m Model) bodyView() string {
	if m.out == nil {
		return ""
	}
	content := m.content.View()
	if !m.outline.visible {
		return content
	}
	gutter := make([]string, m.paneHeight)
	margin := make([]string, m.paneHeight)
	for i := range gutter {
		// The sidebar's left border line is always primary; focus is
		// expressed by the strip's background lift.
		gutter[i] = m.gutterRow
		margin[i] = m.blankRow
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.railView(),
		m.paddedContent(),
		strings.Join(gutter, "\n"),
		m.outlineView(),
		strings.Join(margin, "\n"),
	)
}

// paddedContent wraps the viewport with the pane's top and bottom padding
// rows — the same one-row insets the sidebar carries internally — so the
// document's first and last visible lines never touch the pane's edges.
func (m Model) paddedContent() string {
	rows := make([]string, m.paneHeight)
	for i := range rows {
		rows[i] = m.blankRow
	}
	copy(rows[1:], strings.Split(m.content.View(), "\n"))
	return strings.Join(rows, "\n")
}

// railView builds the one-column rail at the left edge of the text area.
// With an executable block selected it carries a primary bar spanning the
// whole block (its badge through its last line); otherwise a bar beside the
// selected heading's line. The bar tracks the selection whether it came
// from the cursor or the scroll-spy, and renders nothing when nothing is
// selected. Rows span the pane; the content's top padding row shifts bars
// one row down from the viewport row.
func (m Model) railView() string {
	rows := make([]string, m.paneHeight)
	for i := range rows {
		rows[i] = m.blankRow
	}
	if m.out == nil {
		return strings.Join(rows, "\n")
	}

	mark := func(start, end int, style lipgloss.Style) {
		for r := start; r < end; r++ {
			if r >= 0 && r < m.paneHeight {
				rows[r] = style.Render("▌")
			}
		}
	}
	blockRows := func(index int) (int, int) {
		bm := m.out.Blocks[index]
		start := m.displayLine(bm.Line+1) - m.content.YOffset + 1
		return start, m.displayLine(bm.End) - m.content.YOffset + 1
	}

	// The selection bar: the highlighted block, or the heading the
	// scroll-spy is on. Drawn first so the execution status bar below
	// takes precedence when the two overlap.
	if m.blockSel >= 0 && m.blockSel < len(m.out.Blocks) {
		start, end := blockRows(m.blockSel)
		mark(start, end, gutterBarStyle)
	} else if m.outline.sel >= 0 && m.outline.sel < len(m.outline.entries) {
		// The heading's rendered line index (0-based in out.Lines) as a
		// pane row: the jump scrolls to renderLine-1, and the pane's top
		// padding row shifts it one further.
		r := m.displayLine(m.outline.entries[m.outline.sel].renderLine) - m.content.YOffset + 1
		mark(r, r+1, gutterBarStyle)
	}

	// Every executed block keeps its status bar while the reader moves.
	for index, st := range m.inline {
		if st == nil || !m.validBlock(index) || (!st.active && !st.done) {
			continue
		}
		style := railDoneStyle
		switch {
		case st.active:
			style = railRunStyle
		case st.err != nil || st.status != 0:
			style = railFailStyle
		}
		if index == m.blockSel {
			// Brighten the status hue on the block holding the cursor so
			// it stays visible among other executed blocks.
			switch {
			case st.active:
				style = railRunActiveStyle
			case st.err != nil || st.status != 0:
				style = railFailActiveStyle
			default:
				style = railDoneActiveStyle
			}
		}
		start, end := blockRows(index)
		mark(start, end, style)
	}
	return strings.Join(rows, "\n")
}

// setContent installs the document lines, with the inline output box spliced
// under its block, into the viewport.
func (m *Model) setContent() {
	m.updateShifts()
	m.content.SetContent(strings.Join(m.contentLines(), "\n"))
}

// updateShifts caches how many output rows each rendered line gains from
// inline boxes. One pass replaces a scan of every box on every lookup.
func (m *Model) updateShifts() {
	if m.out == nil {
		m.shifts = nil
		return
	}
	shifts := make([]int, len(m.out.Lines)+1)
	for index, st := range m.inline {
		if st == nil || len(st.box) == 0 || !m.validBlock(index) {
			continue
		}
		for line := m.out.Blocks[index].End; line < len(shifts); line++ {
			shifts[line] += len(st.box)
		}
	}
	m.shifts = shifts
}

// contentLines returns the rendered document with the inline output box
// inserted at the end of the executing block.
func (m Model) contentLines() []string {
	if m.out == nil {
		return nil
	}
	inserts := m.inlineInsertions()
	out := make([]string, 0, len(m.out.Lines))
	for i, l := range m.out.Lines {
		out = append(out, l)
		out = append(out, inserts[i]...)
	}
	return out
}

// displayLine maps a rendered line index to its position after all inline
// output boxes spliced before it.
func (m Model) displayLine(line int) int {
	if len(m.shifts) == 0 {
		return line
	}
	if line < 0 {
		return line
	}
	if line >= len(m.shifts) {
		line = len(m.shifts) - 1
	}
	return line + m.shifts[line]
}

func (m Model) outlineView() string {
	return m.outline.view(m.paneHeight, m.focus == focusOutline)
}

func (m Model) sepView() string {
	return m.sep
}

// composeLeftRight joins a left and right segment on one row, truncating the
// left when the row is too narrow.
func composeLeftRight(left, right string, width int) string {
	lw, rw := ansi.StringWidth(left), ansi.StringWidth(right)
	gap := width - lw - rw - 2
	if gap < 1 {
		maxLeft := width - rw - 2
		if maxLeft < 1 {
			return ansi.Truncate(left, width, "…")
		}
		left = ansi.Truncate(left, maxLeft, "…")
		lw = ansi.StringWidth(left)
		gap = width - lw - rw - 2
		if gap < 1 {
			return left
		}
	}
	return left + strings.Repeat(" ", gap) + right
}

// validBlock reports whether i selects a rendered executable block.
func (m Model) validBlock(i int) bool {
	return m.out != nil && i >= 0 && i < len(m.out.Blocks)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// colorEnabled reports whether content may carry ANSI color: the terminal
// must support color and the active theme must not be notty.
func (m Model) colorEnabled() bool {
	return lipgloss.ColorProfile() != termenv.Ascii && m.theme.Name != theme.Notty.Name
}

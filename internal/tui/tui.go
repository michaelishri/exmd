// Package tui implements the md-exec viewer and run mode: the rendered
// runbook on the left, the document outline as a shaded 50-column sidebar
// on the right. The visual language follows the OpenCode TUI — borderless,
// separated by background shades, color reserved for state. The viewer
// floats the panes with one blank row above and below and a one-column
// margin right of the sidebar; run mode keeps a status bar because it
// carries the run state.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/md-exec/internal/doc"
	"github.com/michaelishri/md-exec/internal/render"
	"github.com/michaelishri/md-exec/internal/theme"
)

const (
	minSidebarWidth = 90
	sidebarCols     = 50
	gutterWidth     = 2
	railWidth       = 1
)

type focusArea int

const (
	focusContent focusArea = iota
	focusOutline
)

type mode int

const (
	modeView mode = iota
	modeRun
)

// Model is the bubbletea model of the viewer and runner.
type Model struct {
	doc  *doc.Document
	path string

	width, height int
	laidOut       bool

	out          *render.Output
	content      viewport.Model
	outline      outlineModel
	focus        focusArea
	renderErr    string
	sidebarWidth int

	mode      mode
	run       runState
	overrides map[string]string
	theme     theme.Theme

	// blockSel is the executable block highlighted in the text area
	// (index into the document's blocks); -1 when block selection is off.
	blockSel int

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
		path:      res.Doc.Path,
		content:   viewport.New(0, 0),
		outline:   newOutline(),
		focus:     focusContent,
		mode:      modeView,
		overrides: overrides,
		theme:     t,
		blockSel:  -1,
		run:       runState{globals: map[string]string{}},
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

	case runEventMsg:
		m.applyRunEvent(msg.ev)
		if m.run.active {
			return m, m.listenCmd()
		}
		return m, nil

	case runDoneMsg:
		m.finalizeRun(msg.result, nil)
		return m, nil

	case runErrMsg:
		m.finalizeRun(nil, msg.err)
		return m, nil

	case runClosedMsg:
		return m, nil // drained concurrently with runDoneMsg; no-op

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

	if m.mode == modeView && m.focus == focusContent {
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

// handleKey processes global and focus-specific keys. It reports whether the
// key was consumed.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.shutdownRun()
		return tea.Quit, true

	case "r":
		if m.run.active {
			return nil, true
		}
		if m.mode != modeRun && m.blockSel >= 0 {
			// Run just the highlighted executable block.
			return m.beginBlockRun(), true
		}
		if m.mode != modeRun {
			if len(m.doc.Blocks) == 0 {
				return nil, true
			}
			// Entering run mode always starts fresh.
			return m.beginRun(true), true
		}
		// In run mode: continue remaining, or restart a finished run.
		if m.run.finished || m.run.err != nil ||
			(m.run.result != nil && m.run.result.Failed != nil) {
			return m.beginRun(true), true
		}
		if m.run.session != nil && m.run.session.Remaining() > 0 {
			return m.runRemainingCmd(), true
		}
		return nil, true

	case "n":
		if m.run.active {
			return nil, true
		}
		if m.mode != modeRun {
			if len(m.doc.Blocks) == 0 {
				return nil, true
			}
			// n from the viewer steps straight into run mode.
			return m.beginRun(false), true
		}
		if m.run.session == nil || m.run.session.Remaining() == 0 ||
			(m.run.result != nil && m.run.result.Failed != nil) {
			return nil, true
		}
		return m.stepCmd(), true

	case "s":
		if m.mode == modeRun && m.run.active && m.run.cancel != nil {
			// §4.7: the engine terminates the current process group and
			// the run stops there.
			m.run.cancel()
		}
		return nil, true

	case "v", "esc":
		if m.blockSel >= 0 {
			// Leave executable-block selection; the heading the block
			// belongs to is highlighted again.
			m.exitBlockMode()
			return nil, true
		}
		if m.mode == modeRun && !m.run.active {
			m.mode = modeView
			m.relayout()
			return nil, true
		}
		return nil, true

	case "tab":
		if m.mode == modeRun {
			return nil, true // sidebar is not interactive in run mode
		}
		if m.focus == focusContent && m.outline.visible {
			m.focus = focusOutline
		} else {
			m.focus = focusContent
		}
		return nil, true

	case "shift+tab":
		if m.mode == modeRun {
			return nil, true
		}
		m.focus = focusContent
		return nil, true

	case "t":
		if m.mode == modeRun || m.out == nil {
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

	if m.mode == modeRun {
		// While a run is active, only scrolling works.
		if m.run.active {
			switch msg.String() {
			case "j", "down", "k", "up", "g", "G", "pgup", "pgdown", "ctrl+u", "ctrl+d":
				prev := m.content.YOffset
				m.content, _ = m.content.Update(msg)
				_ = prev
				return nil, true
			}
			return nil, true // consume everything else during a run
		}
		switch msg.String() {
		case "j", "down", "k", "up", "g", "G", "pgup", "pgdown", "ctrl+u", "ctrl+d":
			prev := m.content.YOffset
			m.content, _ = m.content.Update(msg)
			_ = prev
			return nil, true
		}
		return nil, true
	}

	if m.focus == focusOutline {
		switch msg.String() {
		case "j", "down":
			if m.outline.move(1) {
				m.jumpToSelected()
			}
			return nil, true
		case "k", "up":
			if m.outline.move(-1) {
				m.jumpToSelected()
			}
			return nil, true
		case "g":
			if m.outline.first() {
				m.jumpToSelected()
			}
			return nil, true
		case "G":
			if m.outline.last() {
				m.jumpToSelected()
			}
			return nil, true
		case "enter":
			// Enter executable-block selection: the first code block at or
			// after the highlighted heading, arrows walking the blocks.
			m.enterBlockMode()
			return nil, true
		}
		return nil, true // outline focus consumes keys
	}

	// Executable-block selection: the arrows walk the code blocks, g/G
	// jump to the first and last, and r runs the highlighted block.
	if m.blockSel >= 0 {
		switch msg.String() {
		case "down", "j":
			if m.blockSel < len(m.out.Blocks)-1 {
				m.blockSel++
				m.revealBlock()
			}
			return nil, true
		case "up", "k":
			if m.blockSel > 0 {
				m.blockSel--
				m.revealBlock()
			}
			return nil, true
		case "g":
			m.blockSel = 0
			m.revealBlock()
			return nil, true
		case "G":
			m.blockSel = len(m.out.Blocks) - 1
			m.revealBlock()
			return nil, true
		}
	}

	// Enter selects the first executable block at or after the sidebar's
	// highlighted heading (the document's first when none follows).
	if m.blockSel < 0 && msg.String() == "enter" && len(m.out.Blocks) > 0 {
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
	m.sidebarWidth = sidebarW
	contentW := w
	if m.outline.visible {
		if m.mode == modeRun {
			contentW = w - sidebarW - gutterWidth
		} else {
			// 2-col gutter, 1-col rail, 1-col right margin.
			contentW = w - sidebarW - gutterWidth - railWidth - 1
		}
	}
	contentW = max(10, contentW)

	// Viewer: one blank row above and below the panes. Run mode: the
	// hairline + status bar.
	chrome := 2
	bodyH := max(1, h-chrome)
	m.paneHeight = bodyH

	m.content.Width = contentW
	if m.mode == modeRun {
		m.content.Height = bodyH
	} else {
		// The text area carries 1 padding row at its top and bottom — the
		// same insets as the sidebar — so the viewport shows the body
		// height minus those two rows.
		m.content.Height = max(1, bodyH-2)
	}

	if m.mode == modeRun {
		m.laidOut = true
		m.setRunContent()
		return
	}

	out, err := render.Render(m.doc, contentW, m.colorEnabled())
	if err != nil {
		m.renderErr = err.Error()
	} else {
		m.renderErr = ""
		m.out = out
	}

	if m.out != nil {
		m.content.SetContent(strings.Join(m.out.Lines, "\n"))
		m.outline.rebuild(m.doc, m.out, m.sidebarWidth)
	}

	if m.blockSel >= 0 {
		m.revealBlock()
	} else if m.laidOut && len(m.outline.entries) > 0 {
		sel := clamp(m.outline.sel, 0, len(m.outline.entries)-1)
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
	if m.out == nil || m.outline.sel < 0 || m.outline.sel >= len(m.outline.entries) {
		return
	}
	y := clamp(m.outline.entries[m.outline.sel].renderLine-1, 0, m.maxScroll())
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
	m.revealBlock()
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
	if m.out == nil || m.blockSel < 0 || m.blockSel >= len(m.out.Blocks) {
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
	if m.out == nil || m.blockSel < 0 || m.blockSel >= len(m.out.Blocks) {
		return
	}
	bm := m.out.Blocks[m.blockSel]
	top := m.content.YOffset
	bottom := top + m.content.Height
	switch {
	case bm.Line >= top && bm.End <= bottom:
		return // already fully visible
	case bm.Line < top:
		// Above the viewport: pin its badge to the top.
		m.content.SetYOffset(clamp(bm.Line, 0, m.maxScroll()))
	default:
		// Below the viewport: pin its last line to the bottom.
		m.content.SetYOffset(clamp(bm.End-m.content.Height, 0, m.maxScroll()))
	}
}
func (m *Model) maxScroll() int {
	if m.out == nil {
		return 0
	}
	return max(0, len(m.out.Lines)-m.content.Height)
}

// updateSpy mirrors the content scroll position into the sidebar cursor:
// one unified "current position" (SPEC.md §4.8 observation, applied to
// navigation).
func (m *Model) updateSpy() {
	if m.out == nil {
		return
	}
	off := m.content.YOffset
	idx := -1
	if m.outline.tab == tabHeadings {
		// Entries carry the rendered line of their heading; the first h1
		// has no entry (it is the title row), so nothing is selected while
		// the reader is inside its section.
		for i, e := range m.outline.entries {
			if e.renderLine <= off+1 {
				idx = i
			} else {
				break
			}
		}
		if m.content.AtBottom() && len(m.outline.entries) > 0 {
			idx = len(m.outline.entries) - 1
		}
	} else {
		for i, b := range m.out.Blocks {
			if b.Line <= off+1 {
				idx = i
			} else {
				break
			}
		}
		if m.content.AtBottom() && len(m.out.Blocks) > 0 {
			idx = len(m.out.Blocks) - 1
		}
	}
	m.outline.sel = idx
}

// View renders the full screen: header, body, hairline, status.
func (m Model) View() string {
	if !m.laidOut {
		return "Loading…"
	}
	if m.width < 20 || m.height < 4 {
		return m.content.View()
	}
	var sb strings.Builder
	if m.mode == modeRun {
		// Run mode keeps the hairline and status bar: they carry the run
		// state (running, stopped, failed, done).
		sb.WriteString(m.bodyView())
		sb.WriteByte('\n')
		sb.WriteString(m.sepView())
		sb.WriteByte('\n')
		sb.WriteString(m.runStatus())
		return sb.String()
	}
	// The viewer floats the panes with one blank row above and below; the
	// sidebar's left border marks its focus.
	sb.WriteByte('\n')
	sb.WriteString(m.bodyView())
	sb.WriteByte('\n')
	return sb.String()
}

func (m Model) bodyView() string {
	if m.mode == modeRun {
		content := m.content.View()
		if !m.outline.visible {
			return content
		}
		gutter := make([]string, m.content.Height)
		for i := range gutter {
			gutter[i] = strings.Repeat(" ", gutterWidth)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top,
			content,
			strings.Join(gutter, "\n"),
			m.globalsView(m.content.Height),
		)
	}

	if m.renderErr != "" {
		return errStyle.Render("render error: " + m.renderErr)
	}
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
		gutter[i] = strings.Repeat(" ", gutterWidth-1) + sideBorderStyle.Render("│")
		margin[i] = " "
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
		rows[i] = " "
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
		rows[i] = " "
	}
	if m.out == nil {
		return strings.Join(rows, "\n")
	}

	// Block selection: the bar spans the entire code block.
	if m.blockSel >= 0 && m.blockSel < len(m.out.Blocks) {
		bm := m.out.Blocks[m.blockSel]
		start := bm.Line - m.content.YOffset + 1
		end := bm.End - m.content.YOffset + 1
		for r := 0; r < m.paneHeight; r++ {
			if r >= start && r < end {
				rows[r] = gutterBarStyle.Render("▌")
			}
		}
		return strings.Join(rows, "\n")
	}

	if m.outline.sel < 0 || m.outline.sel >= len(m.outline.entries) {
		return strings.Join(rows, "\n")
	}

	// The selected heading's rendered line index (0-based in out.Lines)
	// as a pane row: the jump scrolls to renderLine-1, so the heading text
	// sits one row below the viewport top — and the pane's padding row
	// shifts it one further.
	r := m.outline.entries[m.outline.sel].renderLine - m.content.YOffset + 1
	if 0 <= r && r < m.paneHeight {
		rows[r] = gutterBarStyle.Render("▌")
	}
	return strings.Join(rows, "\n")
}

func (m Model) outlineView() string {
	return m.outline.view(m.paneHeight, m.focus == focusOutline)
}

func (m Model) sepView() string {
	return sepStyle.Render(strings.Repeat("─", max(1, m.width)))
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

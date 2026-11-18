// Package tui implements the md-exec viewer and run mode: the rendered
// runbook on the left, the document outline as a shaded quarter-width column
// on the right, and a single-line status bar. The visual language follows
// the OpenCode TUI — borderless, separated by background shades, color
// reserved for state.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/md-exec/internal/doc"
	"github.com/michaelishri/md-exec/internal/render"
)

const (
	minSidebarWidth = 90
	gutterWidth     = 2
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
	help         help.Model
	showHelp     bool
	keys         keyMap
	renderErr    string
	sidebarWidth int

	mode      mode
	run       runState
	overrides map[string]string
}

// New builds the model for a loaded document; overrides are launch-time
// values for declared globals (SPEC.md §2.2).
func New(res *doc.Result, overrides map[string]string) Model {
	m := Model{
		doc:       res.Doc,
		path:      res.Doc.Path,
		content:   viewport.New(0, 0),
		outline:   newOutline(),
		help:      help.New(),
		keys:      defaultKeyMap(),
		focus:     focusContent,
		mode:      modeView,
		overrides: overrides,
		run:       runState{globals: map[string]string{}},
	}
	m.content.MouseWheelEnabled = true
	tuneHelp(&m.help)
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
		return tea.Quit, true

	case "?":
		m.showHelp = !m.showHelp
		m.relayout()
		return nil, true

	case "r":
		if m.mode != modeRun && !m.run.active && len(m.doc.Blocks) > 0 {
			m.mode = modeRun
			m.resetRun()
			m.relayout()
			return m.startRunCmd(), true
		}
		if m.mode == modeRun && !m.run.active {
			// Re-run.
			m.resetRun()
			m.relayout()
			return m.startRunCmd(), true
		}
		return nil, true

	case "v", "esc":
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
			m.jumpToSelected()
			return nil, true
		}
		return nil, true // outline focus consumes keys
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
		sidebarW = clamp(w/4, 24, 40)
	}
	m.sidebarWidth = sidebarW
	contentW := w
	if m.outline.visible {
		contentW = w - sidebarW - gutterWidth
	}
	contentW = max(10, contentW)

	helpLines := lipgloss.Height(m.statusView())
	bodyH := h - 3 - (helpLines - 1)
	bodyH = max(1, bodyH)

	m.content.Width = contentW
	m.content.Height = bodyH

	if m.mode == modeRun {
		m.laidOut = true
		m.setRunContent()
		return
	}

	out, err := render.Render(m.doc, contentW, colorEnabled())
	if err != nil {
		m.renderErr = err.Error()
	} else {
		m.renderErr = ""
		m.out = out
	}

	if m.out != nil {
		m.content.SetContent(strings.Join(m.out.Lines, "\n"))
		m.outline.rebuild(m.doc, m.out, sidebarW)
	}

	if m.laidOut && len(m.outline.entries) > 0 {
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
// the top of the viewport.
func (m *Model) jumpToSelected() {
	if m.out == nil || m.outline.sel < 0 || m.outline.sel >= len(m.outline.entries) {
		return
	}
	y := clamp(m.outline.entries[m.outline.sel].renderLine-1, 0, m.maxScroll())
	m.content.SetYOffset(y)
	m.updateSpy()
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
		for i, h := range m.out.Headings {
			if h.Line <= off+1 {
				idx = i
			} else {
				break
			}
		}
		if m.content.AtBottom() && len(m.out.Headings) > 0 {
			idx = len(m.out.Headings) - 1
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
	sb.WriteString(m.headerView())
	sb.WriteByte('\n')
	sb.WriteString(m.bodyView())
	sb.WriteByte('\n')
	sb.WriteString(m.sepView())
	sb.WriteByte('\n')
	sb.WriteString(m.statusView())
	return sb.String()
}

func (m Model) headerView() string {
	left := titleStyle.Render(m.doc.HeadingText()) + " " + pathStyle.Render(m.path)
	chips := []string{
		chipStyle.Render(fmt.Sprintf("spec v%d", m.doc.SpecVersion)),
		chipStyle.Render(fmt.Sprintf("%d globals", len(m.doc.Globals))),
		chipStyle.Render(fmt.Sprintf("%d blocks", len(m.doc.Blocks))),
	}
	return composeLeftRight(left, strings.Join(chips, " "), m.width)
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
	gutter := make([]string, m.content.Height)
	for i := range gutter {
		gutter[i] = strings.Repeat(" ", gutterWidth)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		content,
		strings.Join(gutter, "\n"),
		m.outlineView(),
	)
}

func (m Model) outlineView() string {
	return m.outline.view(m.content.Height)
}

func (m Model) sepView() string {
	return sepStyle.Render(strings.Repeat("─", max(1, m.width)))
}

// statusView is the bottom bar: muted key hints or run state on the left,
// position info on the right.
func (m Model) statusView() string {
	if m.mode == modeRun {
		return m.runStatus()
	}
	m.help.ShowAll = m.showHelp
	left := m.help.View(m.keys)
	total := 0
	if m.out != nil {
		total = len(m.out.Lines)
	}
	info := infoStyle.Render(fmt.Sprintf("L %d/%d · %d%%",
		min(m.content.YOffset+1, max(total, 1)),
		max(total, 1),
		int(m.content.ScrollPercent()*100),
	))
	return composeLeftRight(left, info, m.width)
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

func colorEnabled() bool {
	return lipgloss.ColorProfile() != termenv.Ascii
}

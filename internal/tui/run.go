package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/md-exec/internal/doc"
	"github.com/michaelishri/md-exec/internal/exec"
)

// Run mode streams block execution into the content pane while the sidebar
// becomes a live globals table. State arrives as engine events (SPEC.md
// §4.8); everything rendered here derives from them. `n` steps one block,
// `r` runs the rest, `s` stops (the engine terminates the current process
// group, §4.7).

// tea messages for the run pipeline.
type runEventMsg struct{ ev exec.Event }
type runDoneMsg struct{ result *exec.RunResult }
type runErrMsg struct{ err error }
type runClosedMsg struct{}

// eventChanBuffer is sized so that a synchronous test driver cannot
// deadlock on a full channel; the live program drains concurrently.
const eventChanBuffer = 4096

const maxBlockLines = 500

// blockRun is one block's execution record.
type blockRun struct {
	block   *doc.Block
	status  int
	hasEnd  bool
	running bool
	failed  bool
	lines   []string
	pending string
}

func (br *blockRun) write(chunk []byte) {
	text := br.pending + string(chunk)
	parts := strings.Split(text, "\n")
	br.pending = parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		switch {
		case len(br.lines) < maxBlockLines:
			br.lines = append(br.lines, ansi.Strip(p))
		case len(br.lines) == maxBlockLines:
			br.lines = append(br.lines, "… (output truncated)")
		}
	}
}

func (br *blockRun) flush() {
	if strings.TrimSpace(br.pending) != "" && len(br.lines) < maxBlockLines {
		br.lines = append(br.lines, ansi.Strip(br.pending))
	}
	br.pending = ""
}

// runState is the mode's whole state.
type runState struct {
	active   bool // a RunNext is in flight
	finished bool // whole document completed
	blocks   []blockRun
	warnings []string
	globals  map[string]string
	result   *exec.RunResult
	err      error
	ch       chan exec.Event
	lastLen  int // content length at last render, for follow-bottom

	session *exec.Session
	ctx     context.Context
	cancel  context.CancelFunc
}

func (m *Model) resetRun() {
	if m.run.session != nil {
		m.run.session.Close()
		m.run.session = nil
	}
	if m.run.cancel != nil {
		m.run.cancel()
		m.run.cancel = nil
	}
	blocks := make([]blockRun, len(m.doc.Blocks))
	for i, b := range m.doc.Blocks {
		blocks[i] = blockRun{block: b, status: -1}
	}
	m.run = runState{blocks: blocks, globals: map[string]string{}}
}

// beginRun enters run mode with a fresh session; runAll launches every
// remaining block, otherwise the run starts empty (step mode).
func (m *Model) beginRun(runAll bool) tea.Cmd {
	m.mode = modeRun
	m.resetRun()
	// Run mode keeps the hairline + status bar for run state, so the pane
	// heights differ from the viewer's.
	m.relayout()

	s, err := exec.NewSession(m.doc, exec.Options{Overrides: m.overrides})
	if err != nil {
		m.run.err = err
		m.run.finished = true
		m.setRunContent()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.run.session, m.run.ctx, m.run.cancel = s, ctx, cancel
	if runAll {
		return m.runRemainingCmd()
	}
	return nil
}

// runRemainingCmd runs every remaining block of the session, emitting
// events through the channel; it ends at the first failure or cancellation
// (§4.7).
func (m *Model) runRemainingCmd() tea.Cmd {
	ch := make(chan exec.Event, eventChanBuffer)
	m.run.ch = ch
	m.run.active = true
	emit := func(ev exec.Event) { ch <- ev }

	runCmd := func() tea.Msg {
		for {
			br := m.run.session.RunNext(m.run.ctx, emit)
			if br.Done || br.Status != 0 {
				break
			}
		}
		close(ch)
		return runDoneMsg{m.run.session.Result()}
	}
	return tea.Batch(runCmd, m.listenCmd())
}

// stepCmd runs exactly one block.
func (m *Model) stepCmd() tea.Cmd {
	ch := make(chan exec.Event, eventChanBuffer)
	m.run.ch = ch
	m.run.active = true
	emit := func(ev exec.Event) { ch <- ev }

	step := func() tea.Msg {
		m.run.session.RunNext(m.run.ctx, emit)
		close(ch)
		return runDoneMsg{m.run.session.Result()}
	}
	return tea.Batch(step, m.listenCmd())
}

func (m *Model) listenCmd() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.run.ch
		if !ok {
			return runClosedMsg{}
		}
		return runEventMsg{ev}
	}
}

// applyRunEvent folds an engine event into the run state and refreshes the
// viewport.
func (m *Model) applyRunEvent(ev exec.Event) {
	if !m.run.active {
		return
	}
	switch ev.Kind {
	case exec.EventBlockStart:
		if br := m.blockRun(ev.Block); br != nil {
			br.running = true
		}
	case exec.EventOutput:
		if br := m.blockRun(ev.Block); br != nil {
			br.write(ev.Data)
		}
	case exec.EventBlockEnd:
		if br := m.blockRun(ev.Block); br != nil {
			br.running = false
			br.hasEnd = true
			br.status = ev.Status
			br.failed = ev.Status != 0
			br.flush()
		}
	case exec.EventWarning:
		m.run.warnings = append(m.run.warnings, ev.Message)
	case exec.EventGlobals:
		if ev.Globals != nil {
			m.run.globals = ev.Globals
		}
	}
	m.setRunContent()
}

func (m *Model) blockRun(b *doc.Block) *blockRun {
	if b == nil || b.Index-1 < 0 || b.Index-1 >= len(m.run.blocks) {
		return nil
	}
	return &m.run.blocks[b.Index-1]
}

// finalizeRun drains any buffered events, then records the outcome.
func (m *Model) finalizeRun(result *exec.RunResult, err error) {
drain:
	for {
		select {
		case ev, ok := <-m.run.ch:
			if !ok {
				break drain
			}
			m.applyRunEvent(ev)
		default:
			break drain
		}
	}
	for i := range m.run.blocks {
		m.run.blocks[i].flush()
	}
	m.run.active = false
	m.run.result = result
	m.run.err = err
	m.run.finished = err == nil && result != nil &&
		result.Failed == nil && !result.Interrupted &&
		m.run.session != nil && m.run.session.Remaining() == 0
	m.setRunContent()
}

// setRunContent refreshes the viewport's run-mode content, pinning to the
// bottom while the reader is already there (live output streaming).
func (m *Model) setRunContent() {
	lines := m.runViewLines()
	was := m.content.YOffset+m.content.Height >= m.run.lastLen
	m.content.SetContent(strings.Join(lines, "\n"))
	if was {
		m.content.GotoBottom()
	}
	m.run.lastLen = len(lines)
}

// runViewLines builds the run-mode lines for the content viewport.
func (m *Model) runViewLines() []string {
	var lines []string
	total := len(m.run.blocks)
	for i := range m.run.blocks {
		br := &m.run.blocks[i]
		lines = append(lines, m.blockHeader(br, i, total))
		for _, l := range br.lines {
			lines = append(lines, "  "+l)
		}
		lines = append(lines, "")
	}
	for _, w := range m.run.warnings {
		lines = append(lines, warnStyle.Render("  ⚠ "+w))
	}
	if m.run.err != nil {
		lines = append(lines, errStyle.Render("  "+m.run.err.Error()))
	}
	if len(lines) == 0 {
		lines = append(lines, "Nothing to run.")
	}
	return lines
}

func (m *Model) blockHeader(br *blockRun, i, total int) string {
	label := fmt.Sprintf("[%d/%d] %s — line %d", i+1, total, br.block.Lang, br.block.Line)
	if br.block.Kind == doc.KindScript {
		label += " · " + br.block.ScriptPath
	}
	var header string
	switch {
	case br.running:
		header = accentStyle.Render("▸ ") + infoStyle.Render(label)
	case br.failed:
		header = errStyle.Render("✖ ") + infoStyle.Render(label) +
			errStyle.Render(fmt.Sprintf("  (status %d)", br.status))
	case br.hasEnd:
		header = successStyle.Render("✓ ") + infoStyle.Render(label)
	default:
		header = infoStyle.Render("· " + label)
	}
	return header
}

// globalsView renders the sidebar in run mode: the live globals table.
func (m *Model) globalsView(height int) string {
	rows := make([]string, height)
	rows[0] = tabTitleStyle.Width(m.sidebarWidth).Render(" Globals")

	names := make([]string, 0, len(m.run.globals))
	for name := range m.run.globals {
		names = append(names, name)
	}
	sort.Strings(names)

	nameW := 0
	for _, n := range names {
		if len(n) > nameW {
			nameW = len(n)
		}
	}
	nameW = min(nameW, 20)

	for r := 1; r < height; r++ {
		idx := r - 1
		if idx >= len(names) {
			rows[r] = rowStyle.Width(m.sidebarWidth).Render("")
			continue
		}
		name := names[idx]
		val := strings.ReplaceAll(m.run.globals[name], "\n", "⏎")
		row := " " + name + strings.Repeat(" ", max(nameW-len(name)+2, 1)) + val
		rows[r] = rowStyle.Width(m.sidebarWidth).Render(ansi.Truncate(row, m.sidebarWidth-2, "…"))
	}
	return strings.Join(rows, "\n")
}

// runStatus is the status bar in run mode.
func (m *Model) runStatus() string {
	var left string
	switch {
	case m.run.active:
		left = accentStyle.Render("running… (s to stop)")
	case m.run.err != nil:
		left = errStyle.Render("run failed")
	case m.run.result != nil && m.run.result.Failed != nil && m.run.result.Interrupted:
		left = warnStyle.Render(fmt.Sprintf("stopped at block %d · r to continue, v to go back",
			m.run.result.Failed.Index))
	case m.run.result != nil && m.run.result.Failed != nil:
		left = errStyle.Render(fmt.Sprintf("failed at block %d (status %d) · v to go back",
			m.run.result.Failed.Index, m.run.result.ExitStatus))
	case m.run.finished:
		left = successStyle.Render(fmt.Sprintf("done · %d blocks", m.run.result.BlocksRun))
	case m.run.result != nil && m.run.result.BlocksRun > 0:
		remaining := 0
		if m.run.session != nil {
			remaining = m.run.session.Remaining()
		}
		left = infoStyle.Render(fmt.Sprintf("after block %d/%d · n next · r run rest · v back",
			m.run.result.BlocksRun, m.run.result.BlocksRun+remaining))
	default:
		left = infoStyle.Render(fmt.Sprintf("r to run · n to step · %d blocks", len(m.run.blocks)))
	}
	right := ""
	if len(m.run.warnings) > 0 {
		right = warnStyle.Render(fmt.Sprintf("%d warning(s)", len(m.run.warnings)))
	}
	return composeLeftRight(left, right, m.width)
}

// shutdownRun cancels an in-flight run before the program exits (§4.7):
// the engine terminates the current process group; give it a moment.
func (m *Model) shutdownRun() {
	if m.run.cancel != nil {
		m.run.cancel()
	}
	if m.run.active {
		time.Sleep(300 * time.Millisecond)
	}
	if m.run.session != nil {
		m.run.session.Close()
		m.run.session = nil
	}
}

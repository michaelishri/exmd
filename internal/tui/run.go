package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/md-exec/internal/doc"
	"github.com/michaelishri/md-exec/internal/exec"
)

// Run mode streams block execution into the content pane while the sidebar
// becomes a live globals table. State arrives as engine events (SPEC.md
// §4.8); everything rendered here derives from them.

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
	active   bool
	done     bool
	blocks   []blockRun
	warnings []string
	globals  map[string]string
	result   *exec.RunResult
	err      error
	ch       chan exec.Event
	lastLen  int // content length at last render, for follow-bottom
}

func (m *Model) resetRun() {
	blocks := make([]blockRun, len(m.doc.Blocks))
	for i, b := range m.doc.Blocks {
		blocks[i] = blockRun{block: b, status: -1}
	}
	m.run = runState{blocks: blocks, globals: map[string]string{}}
}

// startRunCmd launches the engine in a command goroutine. Events flow
// through the channel; the listen command turns each into a message.
func (m *Model) startRunCmd() tea.Cmd {
	ch := make(chan exec.Event, eventChanBuffer)
	m.run.ch = ch
	m.run.active = true
	runCmd := func() tea.Msg {
		result, err := exec.Run(m.doc, exec.Options{
			Overrides: m.overrides,
			Events:    func(ev exec.Event) { ch <- ev },
		})
		close(ch)
		if err != nil {
			return runErrMsg{err}
		}
		return runDoneMsg{result}
	}
	return tea.Batch(runCmd, m.listenCmd())
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
	if m.run.done {
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
	m.run.done = true
	m.run.result = result
	m.run.err = err
	m.setRunContent()
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
		left = accentStyle.Render("running…")
	case m.run.err != nil:
		left = errStyle.Render("run failed")
	case m.run.result != nil && m.run.result.Failed != nil:
		left = errStyle.Render(fmt.Sprintf("failed at block %d (status %d) · v to go back",
			m.run.result.Failed.Index, m.run.result.ExitStatus))
	case m.run.done:
		left = successStyle.Render(fmt.Sprintf("done · %d blocks", m.run.result.BlocksRun))
	default:
		left = infoStyle.Render(fmt.Sprintf("r to run · %d blocks", len(m.run.blocks)))
	}
	right := ""
	if len(m.run.warnings) > 0 {
		right = warnStyle.Render(fmt.Sprintf("%d warning(s)", len(m.run.warnings)))
	}
	return composeLeftRight(left, right, m.width)
}

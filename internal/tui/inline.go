package tui

import (
	"context"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/md-exec/internal/exec"
	"github.com/michaelishri/md-exec/internal/render"
)

// eventChanBuffer is sized so that a synchronous test driver cannot
// deadlock on a full channel; the live program drains concurrently.
const eventChanBuffer = 4096

// maxBlockLines caps how much output one block's inline box keeps.
const maxBlockLines = 500

// Tea messages for inline block execution. index identifies the block so
// several runs can keep their own output and status.
type inlineEventMsg struct {
	index int
	ev    exec.Event
}
type inlineDoneMsg struct {
	index  int
	result *exec.RunResult
}
type inlineErrMsg struct {
	index int
	err   error
}

// inlineState is one block's execution in place: its live output, final
// status, and the rendered box shown under the block.
type inlineState struct {
	active  bool
	done    bool
	lines   []string
	pending string
	status  int
	hasEnd  bool
	result  *exec.RunResult
	err     error
	box     []string // cached box lines
	ch      chan exec.Event
	ctx     context.Context
	cancel  context.CancelFunc
	session *exec.Session
}

// shutdownInline cancels any in-flight inline runs before quitting: the
// engine terminates each process group (§4.7).
func (m *Model) shutdownInline() {
	for _, st := range m.inline {
		if st == nil {
			continue
		}
		if st.cancel != nil {
			st.cancel()
		}
		if st.active {
			time.Sleep(300 * time.Millisecond)
		}
		if st.session != nil {
			st.session.Close()
			st.session = nil
		}
	}
}

// inlineActive reports whether any block is currently executing inline.
func (m *Model) inlineActive() bool {
	for _, st := range m.inline {
		if st != nil && st.active {
			return true
		}
	}
	return false
}

// stateFor returns the persistent execution state for a block.
func (m *Model) stateFor(index int) *inlineState {
	st := m.inline[index]
	if st == nil {
		st = &inlineState{}
		m.inline[index] = st
	}
	return st
}

// beginInlineRun executes the highlighted block in place. The document view
// stays put; output arrives as events and is appended to that block's box.
func (m *Model) beginInlineRun() tea.Cmd {
	if m.inlineActive() || m.blockSel < 0 || m.blockSel >= len(m.doc.Blocks) {
		return nil
	}
	index := m.blockSel
	st := m.stateFor(index)
	st.active, st.done = true, false
	st.lines, st.pending, st.result, st.err = nil, "", nil, nil
	st.status, st.hasEnd, st.box = 0, false, nil

	s, err := exec.NewSession(m.doc, exec.Options{Overrides: m.overrides})
	if err != nil {
		st.err, st.active, st.done = err, false, true
		m.rebuildInlineBox(index)
		m.setContent()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.session, st.ctx, st.cancel = s, ctx, cancel
	// Show the separator immediately, before the first output event.
	m.rebuildInlineBox(index)
	m.setContent()

	ch := make(chan exec.Event, eventChanBuffer)
	st.ch = ch
	emit := func(ev exec.Event) { ch <- ev }

	runCmd := func() tea.Msg {
		s.RunBlock(ctx, index, emit)
		close(ch)
		return inlineDoneMsg{index: index, result: s.Result()}
	}
	return tea.Batch(runCmd, m.inlineListenCmd(index))
}

// inlineListenCmd waits for the next event from the given block's engine.
func (m *Model) inlineListenCmd(index int) tea.Cmd {
	return func() tea.Msg {
		st := m.inline[index]
		if st == nil || st.ch == nil {
			return nil
		}
		ev, ok := <-st.ch
		if !ok {
			return nil
		}
		return inlineEventMsg{index: index, ev: ev}
	}
}

// applyInlineEvent folds one engine observation into a block's output.
func (m *Model) applyInlineEvent(index int, ev exec.Event) {
	st := m.stateFor(index)
	switch ev.Kind {
	case exec.EventOutput:
		st.write(ev.Data)
	case exec.EventBlockEnd:
		st.status, st.hasEnd = ev.Status, true
	}
	m.rebuildInlineBox(index)
	m.refreshInlineContent(index)
}

// write accumulates output chunks into complete lines, stripping ANSI like
// the run view does.
func (st *inlineState) write(chunk []byte) {
	text := st.pending + string(chunk)
	parts := strings.Split(text, "\n")
	st.pending = parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		if len(st.lines) < maxBlockLines {
			st.lines = append(st.lines, ansi.Strip(p))
		}
	}
}

// finalizeInline records one block's outcome and flushes trailing output.
func (m *Model) finalizeInline(index int, result *exec.RunResult, err error) {
	st := m.stateFor(index)
	// Drain buffered observations before finishing: the synchronous test
	// driver can deliver the done message before the listener drains them.
drain:
	for {
		select {
		case ev, ok := <-st.ch:
			if !ok {
				break drain
			}
			switch ev.Kind {
			case exec.EventOutput:
				st.write(ev.Data)
			case exec.EventBlockEnd:
				st.status, st.hasEnd = ev.Status, true
			}
		default:
			break drain
		}
	}
	if err != nil {
		st.err = err
	}
	if st.pending != "" {
		st.lines = append(st.lines, ansi.Strip(st.pending))
		st.pending = ""
	}
	st.result = result
	st.active = false
	st.done = true
	if st.session != nil {
		st.session.Close()
		st.session = nil
	}
	m.rebuildInlineBox(index)
	m.refreshInlineContent(index)
}

// refreshInlineContent re-sets the viewport content when output grows,
// following the bottom when the reader was already there.
func (m *Model) refreshInlineContent(index int) {
	was := m.content.AtBottom()
	m.setContent()
	if was {
		m.content.GotoBottom()
		return
	}
	// A run whose output is streaming into the selected block keeps its
	// last output row visible, scrolling only as far as necessary.
	if m.blockSel != index || m.out == nil || index >= len(m.out.Blocks) {
		return
	}
	st := m.stateFor(index)
	if len(st.box) == 0 {
		return
	}
	last := m.displayLine(m.out.Blocks[index].End-1) + len(st.box)
	want := clamp(last-m.content.Height+1, 0, m.maxScroll())
	if m.content.YOffset < want {
		m.content.SetYOffset(want)
	}
}

// rebuildInlineBox renders a block's output box: a single hairline separates
// its code from the output, with neither area outlined.
func (m *Model) rebuildInlineBox(index int) {
	st := m.stateFor(index)
	if !st.active && !st.done {
		st.box = nil
		return
	}
	surface := lipgloss.NewStyle().Background(lipgloss.Color(render.ExecColor()))
	edge := surface.Foreground(lipgloss.Color("#87878f"))
	body := surface.Foreground(lipgloss.Color("#e4e4e9"))
	w := m.content.Width

	indent := m.inlineIndent(index)
	prefix := strings.Repeat(" ", max(0, indent-1)) // one plain column is already there

	lines := []string{" " + edge.Render(strings.Repeat("─", w-1))}
	for _, l := range st.lines {
		room := w - 1 - len(prefix)
		if len(l) > room {
			l = l[:room]
		}
		lines = append(lines, " "+body.Render(prefix+l+strings.Repeat(" ", room-len(l))))
	}
	// Close with one empty row.
	lines = append(lines, " "+body.Render(strings.Repeat(" ", w-1)))
	st.box = lines
}

// inlineIndent returns the leading-column count of a block's code text so
// its output lines can line up with it.
func (m Model) inlineIndent(index int) int {
	if m.out == nil || index < 0 || index >= len(m.out.Blocks) {
		return 2
	}
	bm := m.out.Blocks[index]
	for i := bm.Line + 1; i < bm.End; i++ {
		l := ansi.Strip(m.out.Lines[i])
		if strings.TrimSpace(l) != "" {
			return len(l) - len(strings.TrimLeft(l, " "))
		}
	}
	return 2
}

// inlineInsertions returns the rendered line positions (after each block's
// last line) where output boxes are spliced, in document order.
func (m Model) inlineInsertions() [][]string {
	if m.out == nil {
		return nil
	}
	type insertion struct {
		at    int
		lines []string
	}
	var items []insertion
	for index, st := range m.inline {
		if st == nil || len(st.box) == 0 || index < 0 || index >= len(m.out.Blocks) {
			continue
		}
		items = append(items, insertion{at: m.out.Blocks[index].End - 1, lines: st.box})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].at < items[j].at })

	inserts := make([][]string, len(m.out.Lines))
	for _, it := range items {
		inserts[it.at] = it.lines
	}
	return inserts
}

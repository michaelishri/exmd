package tui

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/exec"
	"github.com/michaelishri/exmd/internal/termsafe"
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

// inlineState is one block's execution in place: its live output, final
// status, and the rendered box shown under the block.
type inlineState struct {
	active    bool
	done      bool
	stale     bool
	lines     []string
	pending   string
	status    int
	result    *exec.RunResult
	err       error
	box       []string // cached box lines
	rendered  []string // rendered output rows, one per line in lines
	boxWidth  int      // width used for rendered
	boxIndent int      // indent used for rendered
	urlIndex  int      // next URL to copy from this block's output
	ch        chan exec.Event
	finished  chan struct{} // closed when the engine run returns
	cancel    context.CancelFunc
}

// shutdownGrace bounds how long shutdown waits for a cancelled run. The
// engine's worst case is two grace periods (SIGTERM, then SIGKILL) plus the
// harvest, so this is deliberately generous.
var shutdownGrace = 5 * time.Second

// shutdownInline cancels in-flight runs and waits for them to finish before
// closing the session: a Session is not safe for concurrent use. Any run still
// active is finalized here, so its partial output and interrupted status are
// in the snapshot written on quit.
func (m *Model) shutdownInline() {
	var active []int
	for index, st := range m.inline {
		if st == nil {
			continue
		}
		if st.cancel != nil {
			st.cancel()
		}
		if st.active && st.finished != nil {
			active = append(active, index)
		}
	}
	stillRunning := false
	for _, index := range active {
		st := m.inline[index]
		finished := waitInline(st, shutdownGrace)
		if !st.active {
			continue // finalized through the normal event pipeline
		}
		if finished && st.result != nil {
			m.finalizeInline(index, st.result)
			continue
		}
		// An unfinished run still owns st.result and the session; leave
		// both alone rather than race the engine goroutine.
		stillRunning = stillRunning || !finished
		st.active = false
		st.done = true
		st.err = errors.New("run interrupted on exit")
	}
	if m.inlineSession != nil {
		if !stillRunning {
			m.inlineSession.Close()
		}
		m.inlineSession = nil
	}
}

// waitInline waits for a run to return, draining its event channel so a full
// buffer cannot block the engine before it exits. It reports whether the run
// finished within the timeout.
func waitInline(st *inlineState, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-st.finished:
			return true
		case _, ok := <-st.ch:
			if !ok {
				st.ch = nil
			}
		case <-timer.C:
			return false
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

// stopActiveRuns cancels every in-flight inline run (SPEC.md §4.7). The
// engine terminates each block's process group; finalization happens later
// when inlineDoneMsg arrives.
func (m *Model) stopActiveRuns() {
	for _, st := range m.inline {
		if st != nil && st.active && st.cancel != nil {
			st.cancel()
		}
	}
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
	st.active, st.done, st.stale = true, false, false
	st.lines, st.pending, st.result, st.err = nil, "", nil, nil
	st.status, st.box, st.urlIndex = 0, nil, 0

	// One session for all inline runs carries the harvest delta between
	// blocks (SPEC.md §4.3): running #3 then #9 sees #3's WORK.
	if m.inlineSession == nil {
		var (
			s   *exec.Session
			err error
		)
		if m.resuming {
			s, err = exec.NewSessionFromState(m.doc,
				exec.Options{Overrides: m.overrides}, m.restoredState)
		} else {
			s, err = exec.NewSession(m.doc, exec.Options{Overrides: m.overrides})
		}
		if err != nil {
			st.err, st.active, st.done = err, false, true
			m.rebuildInlineBox(index)
			m.setContent()
			return nil
		}
		m.inlineSession = s
	}
	// A rerun invalidates the saved checkpoint for this block and all later
	// runs; those outputs stay but become stale.
	m.invalidateRunsFor(index)
	s := m.inlineSession
	ctx, cancel := context.WithCancel(context.Background())
	st.cancel = cancel
	// Show the separator immediately, before the first output event.
	m.rebuildInlineBox(index)
	m.setContent()
	m.saveSession() // record that this run is active

	ch := make(chan exec.Event, eventChanBuffer)
	finished := make(chan struct{})
	st.ch, st.finished = ch, finished
	emit := func(ev exec.Event) { ch <- ev }
	m.engineBusy = true

	runCmd := func() tea.Msg {
		defer close(finished)
		s.RunBlock(ctx, index, emit)
		st.result = s.Result()
		close(ch)
		return nil
	}
	return tea.Batch(runCmd, m.inlineListenCmd(st, index))
}

// inlineListenCmd delivers the next event, then reports completion once the
// channel is drained and closed. This gives the channel a single owner: no
// event can be lost between the listener and finalization.
func (m *Model) inlineListenCmd(st *inlineState, index int) tea.Cmd {
	ch := st.ch
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return inlineDoneMsg{index: index, result: st.result}
		}
		return inlineEventMsg{index: index, ev: ev}
	}
}

// applyEvent folds one engine observation into a block's output.
func (st *inlineState) applyEvent(ev exec.Event) {
	switch ev.Kind {
	case exec.EventOutput:
		st.write(ev.Data)
	case exec.EventBlockEnd:
		st.status = ev.Status
	}
}

// applyInlineEvent folds one engine observation into a block's output.
func (m *Model) applyInlineEvent(index int, ev exec.Event) {
	st := m.stateFor(index)
	if st.done {
		return
	}
	st.applyEvent(ev)
	m.rebuildInlineBox(index)
	m.refreshInlineContent(index)
	m.saveSessionThrottled()
}

// write accumulates output chunks into complete lines, dropping escape sequences
// and control characters so output cannot steer the terminal.
func (st *inlineState) write(chunk []byte) {
	text := st.pending + string(chunk)
	parts := strings.Split(text, "\n")
	st.pending = parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		if len(st.lines) < maxBlockLines {
			st.lines = append(st.lines, termsafe.Strip(p))
		}
	}
}

// copiedText returns the block's output as plain text: every complete line,
// plus the trailing partial line still accumulating during a run. Copying
// uses these stored lines, not the rendered box, so the text is neither
// truncated to the pane width nor decorated with the output box's chrome.
func (st *inlineState) copiedText() string {
	text := strings.Join(st.lines, "\n")
	if st.pending != "" {
		if text != "" {
			text += "\n"
		}
		text += st.pending
	}
	return text
}

// finalizeInline records one block's outcome and flushes trailing output.
// The listener has already drained the event channel at this point.
func (m *Model) finalizeInline(index int, result *exec.RunResult) {
	st := m.stateFor(index)
	if st.pending != "" {
		st.lines = append(st.lines, termsafe.Strip(st.pending))
		st.pending = ""
	}
	st.result = result
	st.active = false
	st.done = true
	m.engineBusy = false
	m.recordRun(index)
	m.rebuildInlineBox(index)
	m.refreshInlineContent(index)
	m.saveSession()
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
	if m.blockSel != index || !m.validBlock(index) {
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
	w := m.content.Width
	indent := m.inlineIndent(index)
	// A resize or a new run invalidates the rendered rows. Otherwise only
	// the lines added since the last event need styling.
	if st.boxWidth != w || st.boxIndent != indent || len(st.rendered) > len(st.lines) {
		st.rendered = nil
		st.boxWidth, st.boxIndent = w, indent
	}

	surface := lipgloss.NewStyle().Background(lipgloss.Color(m.theme.ExecPanel))
	body := surface.Foreground(lipgloss.Color(m.theme.Text))
	prefix := strings.Repeat(" ", max(0, indent-1)) // one plain column is already there
	room := max(0, w-1-len(prefix))
	for i := len(st.rendered); i < len(st.lines); i++ {
		l := st.lines[i]
		if len(l) > room {
			l = l[:room]
		}
		st.rendered = append(st.rendered,
			" "+body.Render(prefix+l+strings.Repeat(" ", room-len(l))))
	}

	lines := make([]string, 0, len(st.rendered)+2)
	lines = append(lines, " "+execEdgeStyle.Render(strings.Repeat("─", max(0, w-1))))
	lines = append(lines, st.rendered...)
	lines = append(lines, " "+body.Render(strings.Repeat(" ", max(0, w-1))))
	st.box = lines
}

// inlineIndent returns the leading-column count of a block's code text so
// its output lines can line up with it.
func (m Model) inlineIndent(index int) int {
	if !m.validBlock(index) {
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
		if st == nil || len(st.box) == 0 || !m.validBlock(index) {
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

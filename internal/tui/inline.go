package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/md-exec/internal/exec"
	"github.com/michaelishri/md-exec/internal/render"
)

// Tea messages for inline block execution.
type inlineEventMsg struct{ ev exec.Event }
type inlineDoneMsg struct{ result *exec.RunResult }
type inlineErrMsg struct{ err error }

// inlineState is one block executing in place inside the document view: its
// live output, final status, and the rendered box shown under the block.
type inlineState struct {
	active  bool
	index   int // index into the document's blocks
	lines   []string
	pending string
	status  int
	hasEnd  bool
	done    bool
	result  *exec.RunResult
	err     error
	box     []string // cached box lines
	ch      chan exec.Event
	ctx     context.Context
	cancel  context.CancelFunc
	session *exec.Session
}

// beginInlineRun executes the highlighted executable block in place. The
// document view stays put; output arrives as events and is appended to a box
// under the block.
func (m *Model) beginInlineRun() tea.Cmd {
	if m.inline.active || m.blockSel < 0 || m.blockSel >= len(m.doc.Blocks) {
		return nil
	}
	index := m.blockSel
	st := &m.inline
	st.active, st.index, st.done = true, index, false
	st.lines, st.pending, st.result, st.err = nil, "", nil, nil
	st.status, st.hasEnd, st.box = 0, false, nil

	s, err := exec.NewSession(m.doc, exec.Options{Overrides: m.overrides})
	if err != nil {
		st.err = err
		st.active, st.done = false, true
		m.rebuildInlineBox()
		m.setContent()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.session, st.ctx, st.cancel = s, ctx, cancel
	// Show the separator and an empty output row immediately, so pressing
	// r has visible feedback even before the first output event.
	m.rebuildInlineBox()
	m.setContent()

	ch := make(chan exec.Event, eventChanBuffer)
	st.ch = ch
	emit := func(ev exec.Event) { ch <- ev }

	runCmd := func() tea.Msg {
		s.RunBlock(ctx, index, emit)
		close(ch)
		return inlineDoneMsg{result: s.Result()}
	}
	return tea.Batch(runCmd, m.inlineListenCmd())
}

// inlineListenCmd waits for the next engine event, like the run mode's
// listenCmd.
func (m *Model) inlineListenCmd() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.inline.ch
		if !ok {
			return nil
		}
		return inlineEventMsg{ev}
	}
}

// applyInlineEvent folds one engine observation into the inline output.
func (m *Model) applyInlineEvent(ev exec.Event) {
	st := &m.inline
	switch ev.Kind {
	case exec.EventOutput:
		st.write(ev.Data)
	case exec.EventBlockEnd:
		st.status, st.hasEnd = ev.Status, true
	}
	m.rebuildInlineBox()
	m.refreshInlineContent()
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

// finalizeInline records the outcome and flushes any trailing output.
func (m *Model) finalizeInline(result *exec.RunResult, err error) {
	st := &m.inline
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
	m.rebuildInlineBox()
	m.refreshInlineContent()
}

// refreshInlineContent re-sets the viewport content when output grows,
// following the bottom when the reader was already there.
func (m *Model) refreshInlineContent() {
	was := m.content.AtBottom()
	m.setContent()
	if was {
		m.content.GotoBottom()
	}
}

// rebuildInlineBox renders the output box attached under the block: a
// border separates it from the code above.
func (m *Model) rebuildInlineBox() {
	st := &m.inline
	if !st.active && !st.done {
		st.box = nil
		return
	}
	// A single hairline separates the code above from its output below —
	// neither area gets an outline around it.
	surface := lipgloss.NewStyle().Background(lipgloss.Color(render.ExecColor()))
	edge := surface.Foreground(lipgloss.Color("#87878f"))
	body := surface.Foreground(lipgloss.Color("#e4e4e9"))
	w := m.content.Width

	// Output text lines up with the code text above it.
	indent := m.inlineIndent()
	prefix := strings.Repeat(" ", max(0, indent-1)) // one plain column is already there

	lines := []string{" " + edge.Render(strings.Repeat("─", w-1))}
	for _, l := range st.lines {
		room := w - 1 - len(prefix)
		if len(l) > room {
			l = l[:room]
		}
		lines = append(lines, " "+body.Render(prefix+l+strings.Repeat(" ", room-len(l))))
	}
	// Close the box with one empty row.
	lines = append(lines, " "+body.Render(strings.Repeat(" ", w-1)))
	st.box = lines
}

// inlineIndent returns the leading-column count of the executing block's
// code text, so its output lines can line up with it.
func (m Model) inlineIndent() int {
	if m.out == nil || m.inline.index < 0 || m.inline.index >= len(m.out.Blocks) {
		return 2
	}
	bm := m.out.Blocks[m.inline.index]
	for i := bm.Line + 1; i < bm.End; i++ {
		l := ansi.Strip(m.out.Lines[i])
		if strings.TrimSpace(l) != "" {
			return len(l) - len(strings.TrimLeft(l, " "))
		}
	}
	return 2
}

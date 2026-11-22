package tui

import (
	"sort"
	"time"

	"github.com/michaelishri/exmd/internal/session"
)

// saveInterval bounds how often streaming output touches the disk. State
// transitions (run start, run end, quit) save immediately; output chunks and
// cursor movement are coalesced to this interval.
const saveInterval = 250 * time.Millisecond

// writeSession is a seam for tests that do not want to touch the disk.
var writeSession = session.Save

// saveSession writes the current viewer state to the document's snapshot.
// Save failures are reported in the footer but never disrupt the session.
func (m *Model) saveSession() {
	m.captureSnapshot()
	if m.snap == nil {
		return
	}
	if err := writeSession(m.doc.Path, m.snap); err != nil {
		m.flash = "cannot save session: " + err.Error()
		return
	}
	m.lastSave = time.Now()
}

// saveSessionThrottled coalesces frequent changes into at most one write per
// saveInterval.
func (m *Model) saveSessionThrottled() {
	if time.Since(m.lastSave) < saveInterval {
		return
	}
	m.saveSession()
}

// captureSnapshot folds the live model state into the snapshot that will be
// written. It runs only on the Update goroutine, never from engine callbacks:
// exec.Session is not safe for concurrent use.
func (m *Model) captureSnapshot() {
	if m.snap == nil {
		s, err := session.New(m.doc, m.overrides)
		if err != nil {
			return
		}
		m.snap = s
	}
	s := m.snap
	s.BlockSel = m.blockSel

	// While a run is in flight the engine goroutine owns the session; only
	// touch it once the run has ended (the pre-run save already recorded the
	// cursor and globals for an interrupted snapshot).
	if m.inlineSession != nil && !m.engineBusy {
		st := m.inlineSession.State()
		s.Globals = st.Globals
		s.Absent = st.Absent
		s.Next = st.Next
		s.BlocksRun = st.BlocksRun
		s.Warnings = st.Warnings
		s.Interrupted = st.Interrupted
		s.FailedIndex = st.LastFailedIndex
		s.ExitStatus = st.LastStatus
	}

	var blocks []session.BlockSnapshot
	for index, st := range m.inline {
		if st == nil {
			continue
		}
		bs := session.BlockSnapshot{
			Index:  index,
			Lines:  append([]string(nil), st.lines...),
			Status: st.status,
			Done:   st.done,
			Active: st.active,
		}
		if st.err != nil {
			bs.Err = st.err.Error()
		}
		blocks = append(blocks, bs)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Index < blocks[j].Index })
	s.Blocks = blocks
}

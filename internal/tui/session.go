package tui

import (
	"sort"
	"time"

	"github.com/michaelishri/exmd/internal/exec"
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
// Nothing is written until the session has started (see Model.sessionStarted).
func (m *Model) saveSession() {
	if !m.saveEnabled || !m.sessionStarted {
		return
	}
	m.captureSnapshot()
	if m.snap == nil {
		return
	}
	if err := writeSession(m.doc.Path, m.snap); err != nil {
		m.flash = "cannot save session: " + err.Error()
		return
	}
	m.lastSave = time.Now()
	m.saved = true
}

// invalidateRunsFor drops the saved checkpoint for this block and everything
// after it. Their outputs stay visible but become stale, because a rerun can
// change the globals they were produced with.
func (m *Model) invalidateRunsFor(index int) {
	if m.snap == nil || index < 0 || index >= len(m.blockKeys) {
		return
	}
	stale := m.snap.TruncateRunsAt(m.blockKeys[index])
	set := make(map[string]bool, len(stale))
	for _, key := range stale {
		set[key] = true
	}
	for i, key := range m.blockKeys {
		if i == index || !set[key] {
			continue
		}
		if st := m.inline[i]; st != nil {
			st.stale = true
		}
	}
}

// recordRun appends the state observed after a completed run.
func (m *Model) recordRun(index int) {
	if m.snap == nil || m.inlineSession == nil ||
		index < 0 || index >= len(m.blockKeys) {
		return
	}
	var state exec.SessionState = m.inlineSession.State()
	m.snap.RecordRun(m.blockKeys[index], state)
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
	s.Keys = append([]string(nil), m.blockKeys...)
	if m.blockSel >= 0 && m.blockSel < len(m.blockKeys) {
		s.BlockSelKey = m.blockKeys[m.blockSel]
	} else {
		s.BlockSelKey = ""
	}

	// While a run is in flight the engine goroutine owns the session; only
	// touch it once the run has ended (the pre-run save already recorded the
	// cursor and globals for an interrupted snapshot).
	if m.inlineSession != nil && !m.engineBusy {
		st := m.inlineSession.State()
		s.Globals = st.Globals
		s.Absent = st.Absent
		s.Next = st.Next
	}

	var blocks []session.BlockSnapshot
	for index, st := range m.inline {
		if st == nil {
			continue
		}
		bs := session.BlockSnapshot{
			Index:   index,
			Lines:   append([]string(nil), st.lines...),
			Dropped: st.dropped,
			Status:  st.status,
			Done:    st.done,
			Active:  st.active,
			Stale:   st.stale,
		}
		if index < len(m.blockKeys) {
			bs.Key = m.blockKeys[index]
		}
		if st.err != nil {
			bs.Err = st.err.Error()
		}
		blocks = append(blocks, bs)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Index < blocks[j].Index })
	s.Blocks = blocks
}

package exec

import (
	"fmt"
	"os"
	"sort"

	"github.com/michaelishri/exmd/internal/doc"
)

// SessionState is a restorable snapshot of a Session's document state (§4).
// It carries only legal §4 inputs — globals, the block cursor, and outcome
// bookkeeping — never a live process or leaked shell state. A snapshot saved
// on one run can seed a fresh Session on another with NewSessionFromState.
type SessionState struct {
	// Restored distinguishes an empty restored state from a fresh session.
	Restored bool

	Globals map[string]string // declared globals currently present
	Absent  []string          // declared globals currently absent
	Next    int               // index of the next block to run

	BlocksRun       int
	Warnings        []string
	LastFailedIndex int // -1 when no block has failed
	LastStatus      int
	Interrupted     bool
}

// State snapshots the session's current §4 state.
func (s *Session) State() SessionState {
	st := SessionState{
		Restored:        true,
		Globals:         s.Globals(),
		Next:            s.next,
		BlocksRun:       s.blocksRun,
		Warnings:        s.Warnings(),
		LastFailedIndex: -1,
		LastStatus:      s.lastStatus,
		Interrupted:     s.interrupted,
	}
	for name := range s.declared {
		if _, present := s.globals[name]; !present {
			st.Absent = append(st.Absent, name)
		}
	}
	sort.Strings(st.Absent)
	if s.lastFailed != nil {
		for i, b := range s.d.Blocks {
			if b == s.lastFailed {
				st.LastFailedIndex = i
				break
			}
		}
	}
	return st
}

// NewSessionFromState builds a fresh Session seeded with a saved state. All
// staging is recreated; the host environment is snapshotted from the current
// process (§4.2), not from the saved run — only declared globals survive a
// resume, exactly as if they had arrived via --set and block harvests.
func NewSessionFromState(d *doc.Document, opts Options, st SessionState) (*Session, error) {
	if !st.Restored {
		return newSession(d, opts, nil)
	}
	if st.Next < 0 || st.Next > len(d.Blocks) {
		return nil, fmt.Errorf("resume: next block %d is outside the document", st.Next)
	}
	if st.LastFailedIndex < -1 || st.LastFailedIndex >= len(d.Blocks) {
		return nil, fmt.Errorf("resume: failed block %d is outside the document", st.LastFailedIndex)
	}
	for name := range st.Globals {
		if _, declared := d.Globals[name]; !declared {
			return nil, fmt.Errorf("resume: global %q is not declared", name)
		}
	}
	seen := make(map[string]bool, len(st.Absent))
	for _, name := range st.Absent {
		if _, declared := d.Globals[name]; !declared {
			return nil, fmt.Errorf("resume: global %q is not declared", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("resume: global %q is both present and absent", name)
		}
		seen[name] = true
	}
	for name := range st.Globals {
		if seen[name] {
			return nil, fmt.Errorf("resume: global %q is both present and absent", name)
		}
	}
	return newSession(d, opts, &st)
}

// newSession stages the harnesses and applies the optional saved state.
func newSession(d *doc.Document, opts Options, st *SessionState) (*Session, error) {
	overrides := opts.Overrides
	if overrides == nil {
		overrides = map[string]string{}
	}
	present, diags := d.ResolveGlobals(overrides)
	for _, dg := range diags {
		if dg.Severity == doc.SeverityError {
			return nil, fmt.Errorf("launch-time overrides: %s", dg.Message)
		}
	}

	hostEnv := opts.HostEnv
	if hostEnv == nil {
		hostEnv = os.Environ()
	}
	declared := make(map[string]struct{}, len(d.Globals))
	for name := range d.Globals {
		declared[name] = struct{}{}
	}

	tmpDir, err := os.MkdirTemp("", "mdexec-")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	if err := writeHarnesses(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	s := &Session{
		d:        d,
		opts:     opts,
		declared: declared,
		globals:  present,
		hostEnv:  hostEnv,
		tmpDir:   tmpDir,
	}
	if st != nil {
		s.globals = make(map[string]string, len(st.Globals))
		for name, value := range st.Globals {
			s.globals[name] = value
		}
		for _, name := range st.Absent {
			delete(s.globals, name)
		}
		s.next = st.Next
		s.blocksRun = st.BlocksRun
		s.warnings = append([]string(nil), st.Warnings...)
		s.lastStatus = st.LastStatus
		s.interrupted = st.Interrupted
		if st.LastFailedIndex >= 0 && st.LastFailedIndex < len(d.Blocks) {
			s.lastFailed = d.Blocks[st.LastFailedIndex]
		}
		// The initial globals observation already happened in the saved run.
		s.emittedInitial = true
	}
	return s, nil
}

package exec

import (
	"fmt"
	"os"
	"sort"

	"github.com/michaelishri/exmd/internal/doc"
)

// SessionState is the §4 state a resumed session needs: declared globals and
// the block cursor. It deliberately excludes outcome bookkeeping — a resumed
// session starts fresh, with no inherited failure or interruption status.
type SessionState struct {
	Globals map[string]string // declared globals currently present
	Absent  []string          // declared globals currently absent
	Next    int               // index of the next block to run
}

// State snapshots the session's current §4 state.
func (s *Session) State() SessionState {
	st := SessionState{
		Globals: s.Globals(),
		Next:    s.next,
	}
	for name := range s.declared {
		if _, present := s.globals[name]; !present {
			st.Absent = append(st.Absent, name)
		}
	}
	sort.Strings(st.Absent)
	return st
}

// NewSessionFromState builds a fresh Session seeded with a saved state. All
// staging is recreated; the host environment is snapshotted from the current
// process (§4.2), not from the saved run — only declared globals survive a
// resume, exactly as if they had arrived via --set and block harvests.
func NewSessionFromState(d *doc.Document, opts Options, st SessionState) (*Session, error) {
	if err := ValidateState(d, st); err != nil {
		return nil, fmt.Errorf("resume: %w", err)
	}
	return newSession(d, opts, &st)
}

// ValidateState checks that a saved state can be replayed against d. It is
// shared by the engine's restore path and the tool's snapshot validation.
func ValidateState(d *doc.Document, st SessionState) error {
	if st.Next < 0 || st.Next > len(d.Blocks) {
		return fmt.Errorf("next block %d is outside the document", st.Next)
	}
	for name := range st.Globals {
		if _, declared := d.Globals[name]; !declared {
			return fmt.Errorf("global %q is not declared", name)
		}
	}
	seen := make(map[string]bool, len(st.Absent))
	for _, name := range st.Absent {
		if _, declared := d.Globals[name]; !declared {
			return fmt.Errorf("global %q is not declared", name)
		}
		if seen[name] {
			return fmt.Errorf("global %q is listed as absent twice", name)
		}
		seen[name] = true
	}
	for name := range st.Globals {
		if seen[name] {
			return fmt.Errorf("global %q is both present and absent", name)
		}
	}
	return nil
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
		// The initial globals observation already happened in the saved run.
		s.emittedInitial = true
	}
	return s, nil
}

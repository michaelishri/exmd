package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/exec"
)

// BlockKeys returns a stable content identity for every executable block.
// Prose, line numbers and document position are deliberately excluded; script
// references include the referenced file's bytes, so editing a script changes
// the key of every block that uses it.
func BlockKeys(d *doc.Document) ([]string, error) {
	counts := make(map[string]int)
	keys := make([]string, len(d.Blocks))
	for i, b := range d.Blocks {
		h := sha256.New()
		writePart(h, []byte(b.Kind.String()))
		writePart(h, []byte(b.Lang))
		if b.Kind == doc.KindScript {
			writePart(h, []byte(b.ScriptPath))
			data, err := os.ReadFile(b.ScriptAbs)
			if err != nil {
				return nil, fmt.Errorf("script %s: %w", b.ScriptPath, err)
			}
			writePart(h, data)
		} else {
			writePart(h, []byte(b.Body))
		}
		base := hex.EncodeToString(h.Sum(nil))
		counts[base]++
		// Identical blocks need distinct identities; the occurrence keeps
		// them stable when unrelated blocks are inserted.
		keys[i] = fmt.Sprintf("%s-%d", base[:16], counts[base])
	}
	return keys, nil
}

// InitialState is the §4 state a fresh session would have for this document.
func InitialState(d *doc.Document, overrides map[string]string) (exec.SessionState, error) {
	present, diags := d.ResolveGlobals(overrides)
	for _, dg := range diags {
		if dg.Severity == doc.SeverityError {
			return exec.SessionState{}, fmt.Errorf("launch-time overrides: %s", dg.Message)
		}
	}
	st := exec.SessionState{Globals: present}
	for name := range d.Globals {
		if _, ok := present[name]; !ok {
			st.Absent = append(st.Absent, name)
		}
	}
	sort.Strings(st.Absent)
	return st, nil
}

// Plan matches a snapshot against a possibly edited document. It returns the
// state to restore and per-block presentation, marking outputs whose runs came
// after the first invalid block as stale.
func (s *Snapshot) Plan(d *doc.Document, overrides map[string]string) (*RestorePlan, error) {
	if err := s.Validate(d, overrides); err != nil {
		return nil, err
	}
	keys, err := BlockKeys(d)
	if err != nil {
		return nil, err
	}
	validKeys := make(map[string]bool, len(keys))
	for _, key := range keys {
		validKeys[key] = true
	}

	oldByKey := make(map[string]BlockSnapshot, len(s.Blocks))
	for _, bs := range s.Blocks {
		if bs.Key != "" {
			oldByKey[bs.Key] = bs
		}
	}

	// Find the first completed run whose block no longer exists.
	cut := -1
	for i, run := range s.Runs {
		if !validKeys[run.Key] {
			cut = i
			break
		}
	}

	base, err := InitialState(d, overrides)
	if err != nil {
		return nil, err
	}
	state := base
	switch {
	case cut == -1 && len(s.Runs) > 0:
		state = cloneState(s.Runs[len(s.Runs)-1].After)
	case cut > 0:
		state = cloneState(s.Runs[cut-1].After)
	}
	state = filterState(d, state)

	validRunKeys := make(map[string]bool)
	for i, run := range s.Runs {
		if cut >= 0 && i >= cut {
			break
		}
		validRunKeys[run.Key] = true
	}
	next := len(keys)
	for i, key := range keys {
		if !validRunKeys[key] {
			next = i
			break
		}
	}
	state.Next = next
	if err := exec.ValidateState(d, state); err != nil {
		return nil, fmt.Errorf("saved state cannot be replayed: %w", err)
	}

	staleKeys := make(map[string]bool)
	if cut >= 0 {
		for _, run := range s.Runs[cut:] {
			staleKeys[run.Key] = true
		}
	}

	plan := &RestorePlan{
		State:  state,
		Blocks: make(map[int]BlockRestore),
	}
	for i, key := range keys {
		bs, ok := oldByKey[key]
		if !ok {
			continue
		}
		plan.Blocks[i] = BlockRestore{
			Lines:   append([]string(nil), bs.Lines...),
			Dropped: bs.Dropped,
			Status:  bs.Status,
			Done:    bs.Done,
			Active:  bs.Active,
			Err:     bs.Err,
			Stale:   staleKeys[key],
		}
	}

	oldKeys := s.Keys
	if len(oldKeys) == 0 {
		for key := range oldByKey {
			oldKeys = append(oldKeys, key)
		}
	}
	newKeys := make(map[string]bool, len(keys))
	for _, key := range keys {
		newKeys[key] = true
	}
	for _, key := range oldKeys {
		if key != "" && !newKeys[key] {
			plan.Changed = append(plan.Changed, key)
		}
	}
	for key := range staleKeys {
		if newKeys[key] {
			plan.Stale = append(plan.Stale, key)
		}
	}
	sort.Strings(plan.Changed)
	sort.Strings(plan.Stale)

	plan.BlockSel = -1
	for i, key := range keys {
		if key == s.BlockSelKey {
			plan.BlockSel = i
			break
		}
	}
	if plan.BlockSel < 0 && s.BlockSelKey == "" && s.BlockSel >= 0 && s.BlockSel < len(keys) {
		plan.BlockSel = s.BlockSel
	}
	return plan, nil
}

// TruncateRunsAt removes the named run and everything after it, returning the
// keys of the discarded runs so the caller can mark their outputs stale.
func (s *Snapshot) TruncateRunsAt(key string) []string {
	var stale []string
	for i, run := range s.Runs {
		if run.Key != key {
			continue
		}
		for _, later := range s.Runs[i:] {
			stale = append(stale, later.Key)
		}
		s.Runs = s.Runs[:i]
		break
	}
	return stale
}

// RecordRun appends a completed run checkpoint and updates the rolled-forward
// state stored at the top level.
func (s *Snapshot) RecordRun(key string, st exec.SessionState) {
	st = cloneState(st)
	s.Runs = append(s.Runs, RunSnapshot{Key: key, After: cloneState(st)})
	s.Globals = cloneMap(st.Globals)
	s.Absent = append([]string(nil), st.Absent...)
	s.Next = st.Next
}

func cloneState(st exec.SessionState) exec.SessionState {
	return exec.SessionState{
		Globals: cloneMap(st.Globals),
		Absent:  append([]string(nil), st.Absent...),
		Next:    st.Next,
	}
}

func filterState(d *doc.Document, st exec.SessionState) exec.SessionState {
	out := cloneState(st)
	for name := range out.Globals {
		if _, ok := d.Globals[name]; !ok {
			delete(out.Globals, name)
		}
	}
	absent := out.Absent[:0]
	for _, name := range out.Absent {
		if _, ok := d.Globals[name]; !ok {
			continue
		}
		if _, present := out.Globals[name]; present {
			continue
		}
		absent = append(absent, name)
	}
	out.Absent = absent
	sort.Strings(out.Absent)
	if out.Next < 0 || out.Next > len(d.Blocks) {
		out.Next = len(d.Blocks)
	}
	return out
}

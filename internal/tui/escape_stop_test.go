package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func escKey() tea.KeyMsg { return tea.KeyPressMsg{Code: tea.KeyEsc} }

// TestEscapeStopsRunningBlock checks that Esc owns the stop action and
// keeps the block selected while it is running.
func TestEscapeStopsRunningBlock(t *testing.T) {
	m := newViewer(t, 120, 40)
	m.blockSel = 0
	st := m.stateFor(0)
	st.active = true
	cancelled := 0
	st.cancel = func() { cancelled++ }

	mm, _ := m.Update(escKey())
	m = mm.(Model)
	if cancelled != 1 {
		t.Fatalf("cancel called %d times, want 1", cancelled)
	}
	if m.blockSel != 0 {
		t.Fatalf("blockSel = %d, want the running block to stay selected", m.blockSel)
	}
}

// TestEscapeAgainLeavesBlockMode checks the second press: with no run in
// flight Esc returns to the heading as before.
func TestEscapeAgainLeavesBlockMode(t *testing.T) {
	m := newViewer(t, 120, 40)
	m.blockSel = 0
	st := m.stateFor(0)
	st.active = true
	st.cancel = func() {}

	mm, _ := m.Update(escKey())
	m = mm.(Model)
	st.active = false // engine finalized the stopped run
	mm, _ = m.Update(escKey())
	m = mm.(Model)
	if m.blockSel != -1 {
		t.Fatalf("blockSel = %d, want -1 after the second esc", m.blockSel)
	}
}

// TestSKeyNoLongerStopsRuns pins the removal of the old stop key.
func TestSKeyNoLongerStopsRuns(t *testing.T) {
	m := newViewer(t, 120, 40)
	m.blockSel = 0
	st := m.stateFor(0)
	st.active = true
	cancelled := false
	st.cancel = func() { cancelled = true }

	mm, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: string('s')})
	_ = mm
	if cancelled {
		t.Fatal("s still stops a running block")
	}
}

// TestEscapeInterruptsRealRun checks the whole path: Esc cancels the engine
// process group, which stops well before the block's own sleep finishes.
func TestEscapeInterruptsRealRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newViewerFromDoc(t, 120, 40, "testdata/sleep.md", nil)
	m.blockSel = 0
	mm, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	started := cmd()
	batch, ok := started.(tea.BatchMsg)
	if !ok {
		t.Fatalf("run did not start: %T", started)
	}
	for _, c := range batch {
		go func(c tea.Cmd) { _ = c() }(c)
	}
	time.Sleep(50 * time.Millisecond)

	mm, _ = mm.Update(escKey())
	m = mm.(Model)
	st := m.stateFor(0)
	select {
	case <-st.finished:
	case <-time.After(800 * time.Millisecond):
		t.Fatal("esc did not stop the running block")
	}
	m.shutdownInline()
	st = m.stateFor(0)
	if !st.done || st.result == nil || !st.result.Interrupted {
		t.Fatalf("stopped run was not recorded as interrupted: %+v", st)
	}
}

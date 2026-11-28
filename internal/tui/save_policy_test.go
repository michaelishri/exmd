package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/michaelishri/exmd/internal/session"
)

// TestOpeningWithoutRunningKeepsSavedSession: opening a document, moving
// around and quitting must not replace the session saved for it.
func TestOpeningWithoutRunningKeepsSavedSession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	m := acceptanceViewer(t, path, nil)
	m = runBlockAt(t, m, 0)
	m.shutdownInline()
	m.saveSession()
	before := savedSnapshotFor(t, path)

	fresh := acceptanceViewer(t, path, nil)
	fresh.selectBlock(1)
	fresh = pressKey(fresh, runeKey('r'))
	fresh.Update(tea.KeyPressMsg{Code: 'q', Text: string('q')})
	if fresh.SessionSaved() {
		t.Fatal("a viewer that ran nothing reported a save")
	}
	after := savedSnapshotFor(t, path)
	if len(after.Runs) != len(before.Runs) || len(after.Runs) == 0 {
		t.Fatalf("saved session replaced: runs %d → %d", len(before.Runs), len(after.Runs))
	}
}

// TestFirstRunStartsSaving: the first run in a fresh viewer replaces the
// old session and saving continues from there.
func TestFirstRunStartsSaving(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	m := acceptanceViewer(t, path, nil)
	if s, _ := session.Load(path); s != nil {
		t.Fatal("snapshot written before anything ran")
	}
	m = runBlockAt(t, m, 0)
	if !m.SessionSaved() {
		t.Fatal("run did not save the session")
	}
	if s := savedSnapshotFor(t, path); len(s.Runs) != 1 {
		t.Fatalf("saved runs = %d, want 1", len(s.Runs))
	}
}

// TestWithoutSessionSavingNeverWrites covers --no-session and the
// save_sessions: false preference.
func TestWithoutSessionSavingNeverWrites(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	m := acceptanceViewer(t, path, nil).WithoutSessionSaving()
	m = runBlockAt(t, m, 0)
	m.shutdownInline()
	m.saveSession()
	if m.SessionSaved() {
		t.Fatal("reported a save with saving off")
	}
	if s, err := session.Load(path); s != nil || err != nil {
		t.Fatalf("snapshot written with saving off: %v %v", s, err)
	}
}

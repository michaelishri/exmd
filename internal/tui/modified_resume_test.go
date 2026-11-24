package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestReloadChangedBlockMarksStale(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	m := acceptanceViewer(t, path, nil)
	for i := 0; i < 3; i++ {
		m = runBlockAt(t, m, i)
	}
	if m.inline[0] == nil || m.inline[2] == nil ||
		!strings.Contains(strings.Join(m.inline[2].lines, "\n"), "three done") {
		t.Fatal("setup: blocks did not run")
	}

	edited := strings.Replace(acceptanceSource,
		`echo "two sees [$GREETING] [$TOKEN]"`, `echo "two changed"`, 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}

	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mm.(Model)
	if !strings.Contains(string(m.doc.Source), "two changed") {
		t.Fatal("reload did not pick up the edited document")
	}
	if st := m.inline[1]; st != nil && len(st.lines) > 0 {
		t.Fatalf("changed block kept its old output: %v", st.lines)
	}
	if st := m.inline[2]; st == nil || !st.stale {
		t.Fatalf("later output was not marked stale: %+v", st)
	}
	if !strings.Contains(m.flash, "reloaded") {
		t.Fatalf("no reload summary: %q", m.flash)
	}
	if !strings.Contains(m.View(), "stale") {
		t.Fatal("stale badge missing from the view")
	}
}

func TestReloadParseErrorKeepsDocument(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	m := acceptanceViewer(t, path, nil)
	before := string(m.doc.Source)

	if err := os.WriteFile(path, []byte("---\ntype: runbook\nglobals: not-a-mapping\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mm.(Model)
	if string(m.doc.Source) != before {
		t.Fatal("invalid reload replaced the current document")
	}
	if !strings.Contains(m.flash, "reload failed") {
		t.Fatalf("no reload error: %q", m.flash)
	}
}

func TestReloadBlockedWhileRunning(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := acceptanceViewer(t, acceptanceDoc(t), nil)
	m.stateFor(0).active = true
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mm.(Model)
	if !strings.Contains(m.flash, "stop the running block") {
		t.Fatalf("reload was not blocked: %q", m.flash)
	}
}

func TestRerunMarksLaterOutputsStale(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := acceptanceViewer(t, acceptanceDoc(t), nil)
	for i := 0; i < 3; i++ {
		m = runBlockAt(t, m, i)
	}
	m = runBlockAt(t, m, 0)
	if st := m.inline[0]; st == nil || st.stale {
		t.Fatalf("rerun block should be current: %+v", st)
	}
	for i := 1; i < 3; i++ {
		if st := m.inline[i]; st == nil || !st.stale {
			t.Fatalf("later block %d was not marked stale: %+v", i, st)
		}
	}
	if m.snap == nil || len(m.snap.Runs) != 1 {
		t.Fatalf("run history not truncated: %+v", m.snap)
	}
}

func TestRDoesNotRunBlock(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := enterBlockModeFromFirstHeading(t, acceptanceViewer(t, acceptanceDoc(t), nil))
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mm.(Model)
	if st := m.inline[0]; st != nil && len(st.lines) > 0 {
		t.Fatalf("r unexpectedly ran the block: %v", st.lines)
	}
	if !strings.Contains(m.flash, "reloaded") {
		t.Fatalf("r should still reload the document: %q", m.flash)
	}
}

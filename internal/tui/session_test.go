package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/exec"
	"github.com/michaelishri/exmd/internal/session"
)

const resumeFixture = "testdata/resume.md"

func resumeViewer(t *testing.T, overrides map[string]string) Model {
	t.Helper()
	return newViewerFromDoc(t, 120, 40, resumeFixture, overrides)
}

func runBlockAt(t *testing.T, m Model, index int) Model {
	t.Helper()
	m.blockSel = index
	mm, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return pump(t, mm.(Model), cmd)
}

func savedSnapshot(t *testing.T) *session.Snapshot {
	t.Helper()
	s, err := session.Load(resumeFixture)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("no snapshot was saved")
	}
	return s
}

func TestSessionAutoSavedThenRestored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := resumeViewer(t, nil)
	m = runBlockAt(t, m, 0)
	if st := m.inline[0]; !st.done || st.err != nil {
		t.Fatalf("block 0 did not finish: %+v", st)
	}

	// A completed run saves immediately — no explicit save call needed.
	s := savedSnapshot(t)
	if s.Next != 1 {
		t.Fatalf("saved cursor = %d", s.Next)
	}
	if s.Globals["GREETING"] != "hello from one" {
		t.Fatalf("saved globals = %+v", s.Globals)
	}
	if len(s.Blocks) != 1 || !s.Blocks[0].Done || !strings.Contains(strings.Join(s.Blocks[0].Lines, "\n"), "one done") {
		t.Fatalf("saved output = %+v", s.Blocks)
	}
	if s.BlockSel != 0 {
		t.Fatalf("saved block selection = %d, want 0", s.BlockSel)
	}

	res, err := doc.Load(resumeFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(res.Doc, nil); err != nil {
		t.Fatalf("fresh snapshot did not validate: %v", err)
	}
	restored := NewWithSnapshot(res, nil, s)
	mm, _ := restored.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	restored = mm.(Model)
	if restored.blockSel != 0 {
		t.Fatalf("restored selection = %d", restored.blockSel)
	}
	if !restored.inline[0].done || !strings.Contains(strings.Join(restored.inline[0].lines, "\n"), "one done") {
		t.Fatalf("restored output missing: %+v", restored.inline[0])
	}

	// A dependent block sees the harvested global without re-running block 0.
	restored = runBlockAt(t, restored, 1)
	joined := strings.Join(restored.inline[1].lines, "\n")
	if !strings.Contains(joined, "two sees [hello from one]") {
		t.Fatalf("restored globals not carried: %q", joined)
	}
	if !strings.Contains(strings.Join(restored.inline[0].lines, "\n"), "one done") {
		t.Fatal("re-running a block lost earlier output")
	}
}

func TestRestoredActiveRunIsMarkedStopped(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	res, err := doc.Load(resumeFixture)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.New(res.Doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := session.BlockKeys(res.Doc)
	if err != nil {
		t.Fatal(err)
	}
	s.BlockSel = 0
	s.BlockSelKey = keys[0]
	s.Blocks = []session.BlockSnapshot{{
		Key: keys[0], Index: 0, Lines: []string{"partial output"}, Active: true,
	}}

	m := NewWithSnapshot(res, nil, s)
	st := m.inline[0]
	if st == nil {
		t.Fatal("restored block state missing")
	}
	if st.active || !st.done {
		t.Fatalf("active run should be stopped after restore: %+v", st)
	}
	if !strings.Contains(strings.Join(st.lines, "\n"), "partial output") {
		t.Fatalf("partial output lost: %+v", st.lines)
	}
	if !strings.Contains(m.flash, "interrupted") {
		t.Fatalf("no reaction for interrupted run: %q", m.flash)
	}
}

func TestQuitFinalizesActiveRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := resumeViewer(t, nil)
	m.sessionStarted = true // a run is in flight
	st := m.stateFor(0)
	st.active = true
	st.done = false
	st.lines = []string{"partial output"}
	st.finished = make(chan struct{})
	close(st.finished)
	st.result = &exec.RunResult{Interrupted: true}

	m.shutdownInline()
	if st.active || !st.done {
		t.Fatalf("active run not finalized on quit: %+v", st)
	}
	s := savedSnapshot(t)
	if len(s.Blocks) != 1 || s.Blocks[0].Active || !strings.Contains(strings.Join(s.Blocks[0].Lines, "\n"), "partial output") {
		t.Fatalf("final snapshot lost partial output: %+v", s.Blocks)
	}
}

func TestCursorMovementIsSaved(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := resumeViewer(t, nil)
	m.sessionStarted = true  // a block has run
	m.lastSave = time.Time{} // force the throttled write to happen now
	m.selectBlock(1)
	s := savedSnapshot(t)
	if s.BlockSel != 1 {
		t.Fatalf("saved selection = %d, want 1", s.BlockSel)
	}
}

func TestSaveFailureIsReported(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	prev := writeSession
	writeSession = func(string, *session.Snapshot) error { return errors.New("disk full") }
	t.Cleanup(func() { writeSession = prev })

	m := resumeViewer(t, nil)
	m.sessionStarted = true
	m.saveSession()
	if !strings.Contains(m.flash, "cannot save session") {
		t.Fatalf("no error reaction: %q", m.flash)
	}
}

func TestSnapshotPathUsesDocumentDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := session.Path(filepath.Join("testdata", "resume.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".yaml") || !strings.Contains(path, "exmd"+string(filepath.Separator)+"sessions") {
		t.Fatalf("unexpected snapshot path: %s", path)
	}
}

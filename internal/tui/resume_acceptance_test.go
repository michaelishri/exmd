package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/session"
)

const acceptanceSource = `---
type: runbook
globals: {GREETING, TOKEN: "default"}
---

## one

` + "```bash:exec\n" + `export GREETING="hello from one"
echo "one done"
` + "```\n" + `
## two

` + "```bash:exec\n" + `echo "two sees [$GREETING] [$TOKEN]"
` + "```\n" + `
## three

` + "```bash:exec\n" + `echo "three done"
` + "```\n"

func acceptanceDoc(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runbook.md")
	if err := os.WriteFile(path, []byte(acceptanceSource), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func acceptanceViewer(t *testing.T, path string, overrides map[string]string) Model {
	t.Helper()
	return newViewerFromDoc(t, 120, 40, path, overrides)
}

// A user runs a block, quits, and reopens the runbook with resume. The next
// block must see the exported global without the first block re-running.
func TestAcceptanceResumeFromExportedGlobal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)

	m := acceptanceViewer(t, path, nil)
	m = runBlockAt(t, m, 0)
	m.shutdownInline()
	m.saveSession()

	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("session was not saved on quit")
	}
	if err := s.Validate(res.Doc, nil); err != nil {
		t.Fatalf("resume refused: %v", err)
	}
	restored := NewWithSnapshot(res, nil, s)
	mm, _ := restored.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	restored = runBlockAt(t, mm.(Model), 1)
	out := strings.Join(restored.inline[1].lines, "\n")
	if !strings.Contains(out, "two sees [hello from one]") {
		t.Fatalf("resumed run lost the global: %q", out)
	}
	if restored.inline[0] == nil || !restored.inline[0].done {
		t.Fatal("previous block's status was not restored")
	}
}

// Quitting for real while a block runs must save the interrupted state: the
// engine's process group is cancelled, the run is finalized, and the snapshot
// is written before the program exits.
func TestAcceptanceQuitMidRunSavesRealSnapshot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := "testdata/sleep.md"

	m := newViewerFromDoc(t, 120, 40, path, nil)
	m.blockSel = 0
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Run the tea cmds in goroutines, as the Bubble Tea runtime does, so the
	// block is still in flight when the quit key arrives.
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range batch {
			go func(c tea.Cmd) { c() }(c)
		}
	}
	time.Sleep(50 * time.Millisecond)
	mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	s := savedSnapshotFor(t, path)
	if len(s.Blocks) != 1 || s.Blocks[0].Active || !s.Blocks[0].Done {
		t.Fatalf("in-flight run was not saved as stopped: %+v", s.Blocks)
	}
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(res.Doc, nil); err != nil {
		t.Fatalf("snapshot from quit cannot be resumed: %v", err)
	}
	restored := NewWithSnapshot(res, nil, s)
	mm2, _ := restored.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	restored = runBlockAt(t, mm2.(Model), 0)
	if out := strings.Join(restored.inline[0].lines, "\n"); !strings.Contains(out, "slow done") {
		t.Fatalf("interrupted block could not be re-run: %q", out)
	}
}

// A snapshot taken while a block was still running must come back as a
// stopped block with its partial output, ready to be run again.
func TestAcceptanceResumeAfterInterruption(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	res, err := doc.Load(path)
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
	s.Next = 1 // the engine advances before a block executes
	s.BlockSel = 0
	s.BlockSelKey = keys[0]
	s.Blocks = []session.BlockSnapshot{{
		Key: keys[0], Index: 0, Lines: []string{"partial output"}, Active: true,
	}}
	if err := session.Save(path, s); err != nil {
		t.Fatal(err)
	}

	loaded, err := session.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithSnapshot(res, nil, loaded)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = mm.(Model)
	if m.inline[0].active || !m.inline[0].done {
		t.Fatalf("interrupted block came back active: %+v", m.inline[0])
	}
	if !strings.Contains(strings.Join(m.inline[0].lines, "\n"), "partial output") {
		t.Fatal("partial output lost")
	}
	m = runBlockAt(t, m, 0)
	if !strings.Contains(strings.Join(m.inline[0].lines, "\n"), "one done") {
		t.Fatalf("interrupted block could not be re-run: %+v", m.inline[0].lines)
	}
}

// Prose-only edits do not invalidate a snapshot.
func TestAcceptanceProseEditStillResumes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.New(res.Doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Save(path, s); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(acceptanceSource+"\nedited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(changed.Doc, nil); err != nil {
		t.Fatalf("edited document should resume: %v", err)
	}
	plan, err := s.Plan(changed.Doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changed) != 0 || len(plan.Stale) != 0 {
		t.Fatalf("prose edit invalidated blocks: changed=%v stale=%v", plan.Changed, plan.Stale)
	}
}

// Editing a referenced script changes the executable block's identity.
func TestAcceptanceEditedScriptInvalidatesBlock(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	script := filepath.Join(dir, "helper.sh")
	if err := os.WriteFile(script, []byte("echo old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "runbook.md")
	src := "---\ntype: runbook\n---\n\n```bash:exec\n@helper.sh\n```\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.New(res.Doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Save(path, s); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("echo new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Plan(res.Doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changed) != 1 {
		t.Fatalf("edited script did not invalidate its block: %v", plan.Changed)
	}
}

// Launch-time overrides are part of the snapshot's identity.
func TestAcceptanceOverridesRequired(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"TOKEN": "nightly"}
	s, err := session.New(res.Doc, want)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(res.Doc, want); err != nil {
		t.Fatalf("matching overrides refused: %v", err)
	}
	for _, got := range []map[string]string{nil, {"TOKEN": "prod"}} {
		if err := s.Validate(res.Doc, got); err == nil {
			t.Fatalf("different overrides accepted: %v", got)
		}
	}
}

// A truncated or hand-edited snapshot is reported instead of silently
// starting a fresh session.
func TestAcceptanceCorruptSnapshotRejected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	snapPath, err := session.Path(path)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := session.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapPath, []byte("version: [oops\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = session.Load(path)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("corrupt snapshot not reported: %v", err)
	}
}

// Continuous saving keeps a single latest snapshot with the cumulative
// globals and output of every completed block.
func TestAcceptanceLatestSnapshotWins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	m := acceptanceViewer(t, path, nil)
	m = runBlockAt(t, m, 0)
	runBlockAt(t, m, 1)

	s := savedSnapshotFor(t, path)
	if len(s.Blocks) != 2 || s.Next != 2 {
		t.Fatalf("snapshot did not accumulate: blocks=%d next=%d", len(s.Blocks), s.Next)
	}
	if s.Globals["GREETING"] != "hello from one" {
		t.Fatalf("globals not accumulated: %+v", s.Globals)
	}
	if len(s.Blocks) != 2 {
		t.Fatalf("saved %d blocks, want 2", len(s.Blocks))
	}
	dir, err := session.Dir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d snapshot files, want one per document", len(entries))
	}
}

// The highlighted block is part of the saved session, so resuming returns
// the user to the place they were working.
func TestAcceptanceCursorRestored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := acceptanceDoc(t)
	m := acceptanceViewer(t, path, nil)
	m.sessionStarted = true // a block has run
	m.lastSave = time.Time{}
	m.selectBlock(2)

	res, err := doc.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := savedSnapshotFor(t, path)
	restored := NewWithSnapshot(res, nil, s)
	if restored.blockSel != 2 {
		t.Fatalf("restored selection = %d, want 2", restored.blockSel)
	}
}

func savedSnapshotFor(t *testing.T, path string) *session.Snapshot {
	t.Helper()
	s, err := session.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("no snapshot saved")
	}
	return s
}

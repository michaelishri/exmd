package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// pump executes a tea.Cmd chain synchronously, feeding every produced
// message back through Update until the queue drains.
func pump(t *testing.T, m Model, initial tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{initial}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		nm, cmd := m.Update(msg)
		m = nm.(Model)
		if cmd != nil {
			queue = append(queue, cmd)
		}
	}
	return m
}

func TestRunMode(t *testing.T) {
	m := newViewer(t, 120, 40)
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := mm.(Model)
	if got.mode != modeRun {
		t.Fatal("r did not switch to run mode")
	}
	if !got.run.active {
		t.Fatal("run not active after r")
	}

	got = pump(t, got, cmd)

	if got.run.active {
		t.Fatal("run still active")
	}
	if !got.run.done {
		t.Fatal("run not done")
	}
	if got.run.result == nil || got.run.result.Failed != nil {
		t.Fatalf("demo run failed: %+v", got.run.result)
	}
	if got.run.result.BlocksRun != len(got.doc.Blocks) {
		t.Fatalf("blocks run = %d, want %d", got.run.result.BlocksRun, len(got.doc.Blocks))
	}

	// Every block shows a completion glyph; none failed.
	content := strings.Join(got.runViewLines(), "\n")
	if strings.Contains(content, "✖") {
		t.Fatalf("unexpected failure glyph:\n%s", content)
	}
	if !strings.Contains(content, "in the next block:") {
		t.Fatal("block output missing from run view")
	}

	// Live globals table: DEPLOY_ID harvested, WORK unset by cleanup.
	if got.run.globals["DEPLOY_ID"] == "" {
		t.Fatalf("DEPLOY_ID missing: %v", got.run.globals)
	}
	if _, present := got.run.globals["WORK"]; present {
		t.Fatalf("WORK should be unset: %v", got.run.globals)
	}
	sidebar := got.globalsView(got.content.Height)
	if !strings.Contains(sidebar, "DEPLOY_ID") {
		t.Fatalf("globals sidebar missing DEPLOY_ID:\n%s", sidebar)
	}

	// Status bar reports completion.
	status := got.runStatus()
	if !strings.Contains(status, "done ·") {
		t.Fatalf("status = %q, want done summary", status)
	}

	// v returns to the viewer.
	mm, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got = mm.(Model)
	if got.mode != modeView {
		t.Fatal("v did not return to the viewer")
	}
}

func TestRunModeFailure(t *testing.T) {
	// A failing document: block 2 exits 3.
	m := newViewerFromDoc(t, 120, 40, "../../internal/exec/testdata/fail.md", nil)
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := mm.(Model)

	got = pump(t, got, cmd)

	if got.run.result == nil || got.run.result.Failed == nil {
		t.Fatal("expected failure")
	}
	if got.run.result.ExitStatus != 3 {
		t.Fatalf("status = %d, want 3", got.run.result.ExitStatus)
	}
	if got.run.result.BlocksRun != 2 {
		t.Fatalf("blocks run = %d, want 2", got.run.result.BlocksRun)
	}
	content := strings.Join(got.runViewLines(), "\n")
	if !strings.Contains(content, "✖ [2/3] bash — line 15") {
		t.Fatalf("failure header missing:\n%s", content)
	}
	status := got.runStatus()
	if !strings.Contains(status, "failed at block 2 (status 3)") {
		t.Fatalf("status = %q", status)
	}
}

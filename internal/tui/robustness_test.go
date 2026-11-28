package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/michaelishri/exmd/internal/doc"
)

// TestCtrlCQuitsFromSettings: ctrl+c must quit even while the settings panel
// owns the keyboard, on both of its pages.
func TestCtrlCQuitsFromSettings(t *testing.T) {
	for _, page := range []string{"menu", "themes"} {
		m := newViewer(t, 120, 40)
		m = pressKey(m, runeKey(','))
		if page == "themes" {
			m = pressKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
		}
		_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		if cmd == nil {
			t.Fatalf("%s: ctrl+c returned no command", page)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s: ctrl+c did not quit", page)
		}
	}
}

// TestReloadMissingScriptReportsError: a script that is missing when the
// viewer opens (or goes missing later) makes reload report the problem
// instead of crashing.
func TestReloadMissingScriptReportsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	src := "---\ntype: runbook\n---\n# T\n\n```bash:exec\n@missing.sh\n```\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newViewerFromDoc(t, 120, 40, path, nil)
	m = pressKey(m, runeKey('r'))
	if !strings.Contains(m.flash, "reload failed") {
		t.Fatalf("flash = %q, want a reload failure", m.flash)
	}
}

// TestKeysBeforeLayoutAreSafe: keys can arrive before the first window size
// message; none of them may crash, including in a resumed session that
// starts with a block selected.
func TestKeysBeforeLayoutAreSafe(t *testing.T) {
	res, err := doc.Load("../../examples/demo/demo.md")
	if err != nil {
		t.Fatal(err)
	}
	keys := []tea.KeyMsg{
		runeKey('`'), runeKey('j'), runeKey('k'), runeKey('g'), runeKey('G'),
		runeKey('c'), runeKey('u'), runeKey('t'), runeKey(','), runeKey('v'),
		tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyEsc},
		tea.KeyPressMsg{Code: tea.KeyTab}, tea.KeyPressMsg{Code: tea.KeyDown},
		tea.KeyPressMsg{Code: tea.KeyUp},
	}
	for _, sel := range []int{-1, 0} {
		for _, k := range keys {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("blockSel %d, key %q: panic: %v", sel, k.String(), r)
					}
				}()
				m := New(res, map[string]string{"JOB_NAME": "x"})
				m.blockSel = sel
				m.Update(k)
			}()
		}
	}
}

// TestQuitWhileRunOutlivesGrace: when a stopped block outlives the shutdown
// grace, quitting must not touch the state the engine goroutine still owns
// (run with -race).
func TestQuitWhileRunOutlivesGrace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	old := shutdownGrace
	shutdownGrace = 100 * time.Millisecond
	defer func() { shutdownGrace = old }()

	path := filepath.Join(t.TempDir(), "doc.md")
	src := "---\ntype: runbook\n---\n# T\n\n```bash:exec\ntrap '' TERM\nsleep 3\n```\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newViewerFromDoc(t, 120, 40, path, nil)
	m.blockSel = 0
	mm, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mm.(Model)
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("run did not start")
	}
	for _, c := range batch {
		go func(c tea.Cmd) { _ = c() }(c)
	}
	time.Sleep(100 * time.Millisecond)

	st := m.stateFor(0)
	m.shutdownInline()
	if st.active || st.err == nil || !strings.Contains(st.err.Error(), "interrupted") {
		t.Fatalf("run not marked interrupted: active=%v err=%v", st.active, st.err)
	}
	if m.inlineSession != nil {
		t.Fatal("session still attached after shutdown")
	}
	select {
	case <-st.finished:
	case <-time.After(6 * time.Second):
		t.Fatal("engine never finished the killed block")
	}
}

// TestNoSidewaysScrolling: the document is laid out to the pane width, so
// left, right, h and l must leave every frame unchanged.
func TestNoSidewaysScrolling(t *testing.T) {
	m := newViewer(t, 120, 40)
	before := m.View().Content
	for _, k := range []tea.KeyPressMsg{
		{Code: tea.KeyRight}, {Code: tea.KeyLeft},
		{Code: 'l', Text: "l"}, {Code: 'h', Text: "h"},
	} {
		m = pressKey(m, k)
		if m.View().Content != before {
			t.Fatalf("%s shifted the document", k.String())
		}
	}
	mm, _ := m.Update(tea.MouseWheelMsg{X: 10, Y: 10, Button: tea.MouseWheelRight})
	if mm.(Model).View().Content != before {
		t.Fatal("a sideways wheel shifted the document")
	}
}

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/theme"
)

// TestOutputKeepsMostRecentLines: past the cap, the newest lines stay and
// the box says how many earlier ones were dropped.
func TestOutputKeepsMostRecentLines(t *testing.T) {
	m := newViewer(t, 120, 40)
	st := m.stateFor(0)
	st.active = true
	for i := 1; i <= maxBlockLines+250; i++ {
		st.write([]byte(fmt.Sprintf("line %d\n", i)))
		if i%100 == 0 {
			m.rebuildInlineBox(0) // rendered rows must follow the dropped lines
		}
	}
	if len(st.lines) != maxBlockLines || st.dropped != 250 {
		t.Fatalf("kept %d, dropped %d; want %d and 250", len(st.lines), st.dropped, maxBlockLines)
	}
	if st.lines[0] != "line 251" || st.lines[len(st.lines)-1] != fmt.Sprintf("line %d", maxBlockLines+250) {
		t.Fatalf("kept the wrong lines: first %q, last %q", st.lines[0], st.lines[len(st.lines)-1])
	}
	m.rebuildInlineBox(0)
	box := ansi.Strip(strings.Join(st.box, "\n"))
	if !strings.Contains(box, "… 250 earlier line(s) not shown") {
		t.Fatalf("no dropped-lines marker in box")
	}
	if !strings.Contains(box, "line 251 ") || strings.Contains(box, "line 250 ") {
		t.Fatalf("rendered rows out of step with stored lines")
	}
}

// TestOutputLongLineIsBounded: a line with no newline cannot grow without
// limit; it is cut, marked, and the next line is unaffected.
func TestOutputLongLineIsBounded(t *testing.T) {
	st := &inlineState{}
	chunk := []byte(strings.Repeat("x", 64<<10))
	for i := 0; i < 160; i++ { // 10 MiB in 64 KiB writes
		st.write(chunk)
	}
	if len(st.pending) > maxLineBytes {
		t.Fatalf("pending grew to %d bytes", len(st.pending))
	}
	st.write([]byte("tail\nnext\n"))
	if len(st.lines) != 2 || !strings.HasSuffix(st.lines[0], "x…") || len(st.lines[0]) > maxLineBytes+len("…") {
		t.Fatalf("long line not capped: %d lines, first %d bytes", len(st.lines), len(st.lines[0]))
	}
	if st.lines[1] != "next" {
		t.Fatalf("line after the long one = %q", st.lines[1])
	}
}

// TestOutputLineCleaning covers carriage returns, CRLF, escape sequences,
// and multi-byte characters split across writes.
func TestOutputLineCleaning(t *testing.T) {
	st := &inlineState{}
	st.write([]byte("10%\r50%\r100%\n"))
	st.write([]byte("crlf\r\n"))
	st.write([]byte("\x1b[32mok\x1b[0m\r\x1b[Kdone\n"))
	st.write([]byte("日本"[:4]))
	st.write([]byte("日本"[4:] + "語\n"))
	want := []string{"100%", "crlf", "done", "日本語"}
	if strings.Join(st.lines, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %q, want %q", st.lines, want)
	}
}

// TestOutputRowsFitTheirCells: tabs, wide characters and cut points never
// make a row wider or narrower than the box, and never split a character.
func TestOutputRowsFitTheirCells(t *testing.T) {
	for _, line := range []string{
		"a\tb\tc", "\t\tindented", "日本語の出力がとても長い行です日本語の出力がとても長い行です",
		"emoji 😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀😀", strings.Repeat("ab\t", 40), "",
	} {
		for _, width := range []int{1, 7, 20, 41} {
			got := fitCells(line, width)
			if w := ansi.StringWidth(got); w != width {
				t.Errorf("fitCells(%q, %d) width %d", line, width, w)
			}
			if !utf8.ValidString(got) || strings.ContainsRune(got, '\t') {
				t.Errorf("fitCells(%q, %d) = %q", line, width, got)
			}
		}
	}
	if got := fitCells("a\tb", 20); !strings.HasPrefix(got, "a       b") {
		t.Errorf("tab stop: %q", got)
	}
}

// TestLivePartialLineIsShown: a progress bar written without a newline is
// visible while the block runs.
func TestLivePartialLineIsShown(t *testing.T) {
	m := newViewer(t, 120, 40)
	st := m.stateFor(0)
	st.active = true
	st.write([]byte("downloading 10%\rdownloading 42%"))
	m.rebuildInlineBox(0)
	if box := ansi.Strip(strings.Join(st.box, "\n")); !strings.Contains(box, "downloading 42%") || strings.Contains(box, "10%") {
		t.Fatalf("live line not shown as the terminal would:\n%s", box)
	}
}

// TestURLCacheFollowsOutput: the cached URL list is refreshed when output
// changes.
func TestURLCacheFollowsOutput(t *testing.T) {
	st := &inlineState{}
	st.write([]byte("see https://a.example\n"))
	if urls := st.outputURLs(); len(urls) != 1 {
		t.Fatalf("urls = %v", urls)
	}
	st.write([]byte("and https://b.example\n"))
	if urls := st.outputURLs(); len(urls) != 2 {
		t.Fatalf("urls after more output = %v", urls)
	}
}

// TestThemeChangeRepaintsOutput: output rows rendered under one theme are
// redrawn with the new theme's colors.
func TestThemeChangeRepaintsOutput(t *testing.T) {
	defer SetTheme(theme.Dark)
	defer useProfile(colorprofile.TrueColor)()
	m := newViewer(t, 120, 40)
	st := m.stateFor(0)
	st.done = true
	st.write([]byte("hello\n"))
	m.rebuildInlineBox(0)
	before := st.rendered[0]
	th, err := theme.Resolve("catppuccin-latte", true)
	if err != nil {
		t.Fatal(err)
	}
	m.applyTheme(th)
	if st.rendered[0] == before {
		t.Fatal("output row kept the old theme's colors")
	}
}

// TestHeavyOutputStaysResponsive is the acceptance test for streaming: a
// block printing 100,000 lines must finish promptly through the viewer's
// real event pipeline, with the box holding the most recent lines.
func TestHeavyOutputStaysResponsive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "doc.md")
	src := "---\ntype: runbook\n---\n# Heavy\n\nSome prose.\n\n```bash:exec\nfor ((i=1; i<=100000; i++)); do echo \"row $i\"; done\n```\n\n## After\n\nMore prose.\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newViewerFromDoc(t, 120, 40, path, nil)
	m.blockSel = 0
	start := time.Now()
	mm, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mm.(Model)
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatal("run did not start")
	}
	go func() { _ = batch[0]() }() // the engine
	next := batch[1]
	for next != nil {
		msg := next()
		mm, next = m.Update(msg)
		m = mm.(Model)
		if _, done := msg.(inlineDoneMsg); done {
			break
		}
	}
	elapsed := time.Since(start)
	st := m.stateFor(0)
	if !st.done || st.status != 0 {
		t.Fatalf("run did not finish cleanly: done=%v status=%d err=%v", st.done, st.status, st.err)
	}
	if last := st.lines[len(st.lines)-1]; last != "row 100000" || st.dropped != 100000-maxBlockLines {
		t.Fatalf("last line %q, dropped %d", last, st.dropped)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("100,000 lines took %v", elapsed)
	}
	t.Logf("100,000 lines in %v", elapsed)
}

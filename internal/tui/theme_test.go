package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/theme"
)

// TestSetThemeSetsViewerStyles checks that a model built after SetTheme
// captures that theme and derives its styles from it. The test
// forces an ANSI profile so the dark theme is expected to carry color even
// though the test process has no TTY.
func TestSetThemeSetsViewerStyles(t *testing.T) {
	defer useProfile(colorprofile.ANSI)()
	defer SetTheme(theme.Dark)

	SetTheme(theme.Dark)
	if s := newViewer(t, 120, 40).styles.info.Render("x"); s == "x" {
		t.Fatal("dark theme rendered no color despite a color profile")
	}

	SetTheme(theme.Notty)
	m := newViewer(t, 120, 40)
	if s := m.styles.info.Render("x"); s != "x" {
		t.Fatalf("notty info style = %q, want plain text", s)
	}
	if m.theme.Name != theme.Notty.Name {
		t.Fatalf("model theme = %q, want %q", m.theme.Name, theme.Notty.Name)
	}

	// A fresh New after restoring dark captures the dark theme.
	SetTheme(theme.Dark)
	m = newViewer(t, 120, 40)
	if m.theme.Name != theme.Dark.Name {
		t.Fatalf("model theme = %q, want %q", m.theme.Name, theme.Dark.Name)
	}
}

// TestViewPaintsCanvas verifies that the finished frame carries the theme's
// background on every row — including blank rows, the sidebar and the
// footer — so no cell falls back to the terminal's palette.
func TestViewPaintsCanvas(t *testing.T) {
	defer useProfile(colorprofile.TrueColor)()
	defer SetTheme(theme.Dark)
	SetTheme(theme.Dark)

	m := newViewer(t, 120, 40)
	view := m.View().Content
	canvas := "\x1b[48;2;14;14;17m"
	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("view has %d rows, want %d", len(lines), m.height)
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, canvas) {
			t.Errorf("row %d has no canvas background: %q", i, line)
		}
		if w := ansi.StringWidth(line); w != m.width {
			t.Errorf("row %d width = %d, want %d", i, w, m.width)
		}
	}
}

// TestNottyViewIsPlain checks that the notty theme emits no SGR sequences,
// even on a color-capable terminal.
func TestNottyViewIsPlain(t *testing.T) {
	defer useProfile(colorprofile.TrueColor)()
	defer SetTheme(theme.Dark)
	SetTheme(theme.Notty)

	m := newViewer(t, 120, 40)
	got := m.View().Content
	if strings.Contains(got, "\x1b[38;") || strings.Contains(got, "\x1b[48;") {
		t.Fatalf("notty view contains colors: %q", got)
	}
}

// TestInlineBoxUsesThemeSurface checks that inline output shares the
// executable surface from the theme rather than a package constant.
func TestInlineBoxUsesThemeSurface(t *testing.T) {
	defer useProfile(colorprofile.TrueColor)()
	defer SetTheme(theme.Dark)
	SetTheme(theme.Dark)

	m := newViewer(t, 120, 40)
	st := m.stateFor(0)
	st.active = true
	st.lines = []string{"hello"}
	m.inline[0] = st
	m.rebuildInlineBox(0)

	exec := trueColorSeq(theme.Dark.ExecPanel, true)
	found := false
	for _, line := range st.box {
		if strings.Contains(line, exec) {
			found = true
		}
	}
	if !found {
		t.Fatalf("inline box has no theme exec surface: %q", st.box)
	}
}

// TestThemePreviewIsLocalToTheViewer: previewing a theme in one viewer must
// not restyle another, nor change the theme new viewers start with.
func TestThemePreviewIsLocalToTheViewer(t *testing.T) {
	defer useProfile(colorprofile.TrueColor)()
	defer SetTheme(theme.Dark)
	SetTheme(theme.Dark)

	a, b := newViewer(t, 120, 40), newViewer(t, 120, 40)
	before := a.View().Content
	b = pressKey(b, runeKey(','))
	b = pressKey(b, tea.KeyPressMsg{Code: tea.KeyEnter})
	b = pressKey(b, tea.KeyPressMsg{Code: tea.KeyDown})
	if b.theme.Name == theme.Dark.Name {
		t.Fatal("preview did not change the previewing viewer")
	}
	if a.View().Content != before {
		t.Fatal("another viewer was restyled by the preview")
	}
	if newViewer(t, 120, 40).theme.Name != theme.Dark.Name {
		t.Fatal("preview changed the theme new viewers start with")
	}
}

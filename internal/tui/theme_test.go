package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/exmd/internal/theme"
)

// TestSetThemeRebuildsStyles checks that SetTheme re-points the derived
// styles and that a model built afterwards captures the theme. The test
// forces an ANSI profile so the dark theme is expected to carry color even
// though the test process has no TTY.
func TestSetThemeRebuildsStyles(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(prevProfile)
	defer SetTheme(theme.Dark)

	SetTheme(theme.Dark)
	if s := infoStyle.Render("x"); s == "x" {
		t.Fatal("dark theme rendered no color despite a color profile")
	}

	SetTheme(theme.Notty)
	if s := infoStyle.Render("x"); s != "x" {
		t.Fatalf("notty infoStyle = %q, want plain text", s)
	}

	m := newViewer(t, 120, 40)
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
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)
	defer SetTheme(theme.Dark)
	SetTheme(theme.Dark)

	m := newViewer(t, 120, 40)
	view := m.View()
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
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)
	defer SetTheme(theme.Dark)
	SetTheme(theme.Notty)

	m := newViewer(t, 120, 40)
	got := m.View()
	if strings.Contains(got, "\x1b[38;") || strings.Contains(got, "\x1b[48;") {
		t.Fatalf("notty view contains colors: %q", got)
	}
}

// TestInlineBoxUsesThemeSurface checks that inline output shares the
// executable surface from the theme rather than a package constant.
func TestInlineBoxUsesThemeSurface(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)
	defer SetTheme(theme.Dark)
	SetTheme(theme.Dark)

	m := newViewer(t, 120, 40)
	st := m.stateFor(0)
	st.active = true
	st.lines = []string{"hello"}
	m.inline[0] = st
	m.rebuildInlineBox(0)

	exec := "48;2;30;32;48m"
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

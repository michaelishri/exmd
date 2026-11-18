package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/michaelishri/md-exec/internal/theme"
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
	if s := pathStyle.Render("x"); s == "x" {
		t.Fatal("dark theme rendered no color despite a color profile")
	}

	SetTheme(theme.Notty)
	if s := pathStyle.Render("x"); s != "x" {
		t.Fatalf("notty pathStyle = %q, want plain text", s)
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

package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/michaelishri/exmd/internal/config"
	"github.com/michaelishri/exmd/internal/theme"
)

func pressKey(m Model, msg tea.KeyMsg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestSettingsOpenAndPages(t *testing.T) {
	defer SetTheme(theme.Dark)
	m := newViewer(t, 120, 40)

	m = pressKey(m, runeKey(','))
	if !m.settings.open {
		t.Fatal("comma did not open settings")
	}
	if m.settings.page != settingsMenu {
		t.Fatalf("page = %v, want menu", m.settings.page)
	}
	view := m.View()
	if !strings.Contains(view, "Settings") {
		t.Fatal("settings panel missing from view")
	}
	if strings.Contains(view, m.doc.Path) {
		t.Fatal("file path should be hidden while settings own the pane")
	}
	if hints := m.keyHints(); !strings.Contains(hints, "enter open") {
		t.Fatalf("menu hints = %q", hints)
	}

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.settings.page != settingsThemePage {
		t.Fatalf("page = %v, want theme list", m.settings.page)
	}
	if !strings.Contains(m.View(), "Settings / Theme") {
		t.Fatal("theme list missing from view")
	}
	if hints := m.keyHints(); !strings.Contains(hints, "preview") {
		t.Fatalf("theme hints = %q", hints)
	}
}

// TestSettingsFooterBand checks that the theme page gets a distinct sidebar
// footer band, and that pages without hints leave the whole pane to settings.
func TestSettingsFooterBand(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)
	defer SetTheme(theme.Dark)

	m := newViewer(t, 120, 40)
	leftWidth := railWidth + m.content.Width + gutterWidth
	sidebar := func(m Model, fromEnd int) string {
		lines := strings.Split(m.View(), "\n")
		return string([]rune(lines[len(lines)-fromEnd])[leftWidth:])
	}

	m = pressKey(m, runeKey(','))
	band := sidebar(m, 2)
	if !strings.Contains(ansi.Strip(band), "enter open") {
		t.Fatalf("menu page footer hints missing: %q", band)
	}
	want := termenv.TrueColor.
		Color(string(theme.Lift(theme.Dark.BackgroundPanel, 0.10))).Sequence(true)
	if !strings.Contains(sidebar(m, 3), want) || !strings.Contains(band, want) {
		t.Fatalf("menu footer band has no distinct shade (%s): %q", want, band)
	}

	// The left footer must not repeat the sidebar hints.
	lines := strings.Split(m.View(), "\n")
	left := string([]rune(ansi.Strip(lines[len(lines)-2]))[:leftWidth])
	if strings.Contains(left, "enter open") {
		t.Fatalf("settings hints duplicated in the left footer: %q", left)
	}

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	band = sidebar(m, 2)
	if !strings.Contains(ansi.Strip(band), "preview") {
		t.Fatalf("theme page footer hints missing: %q", band)
	}
	if n := strings.Count(ansi.Strip(m.View()), "preview"); n != 1 {
		t.Fatalf("theme hints appear %d times, want 1", n)
	}
	if !strings.Contains(sidebar(m, 3), want) || !strings.Contains(band, want) {
		t.Fatalf("theme footer band has no distinct shade (%s): %q", want, band)
	}
}

func TestSettingsLivePreviewAndRevert(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prevProfile)
	defer SetTheme(theme.Dark)
	m := newViewer(t, 120, 40)
	original := m.theme.Name

	m = pressKey(m, runeKey(','))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyDown})

	if m.theme.Name == original {
		t.Fatal("arrow key did not preview a different theme")
	}
	if got := m.settings.themes[m.settings.cursor].Name; got != m.theme.Name {
		t.Fatalf("cursor theme = %q, active theme = %q", got, m.theme.Name)
	}
	if s := m.styles.info.Render("x"); s == "x" {
		t.Fatal("preview did not rebuild the viewer styles")
	}

	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.settings.open {
		t.Fatal("esc did not close settings")
	}
	if m.theme.Name != original {
		t.Fatalf("theme after esc = %q, want %q", m.theme.Name, original)
	}
}

func TestSettingsCommitSaves(t *testing.T) {
	defer SetTheme(theme.Dark)
	old := saveConfig
	defer func() { saveConfig = old }()

	var saved config.Config
	called := false
	saveConfig = func(c config.Config) error {
		called, saved = true, c
		return nil
	}

	m := newViewer(t, 120, 40)
	m = pressKey(m, runeKey(','))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyDown})
	want := m.theme.Name
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !called {
		t.Fatal("theme was not saved")
	}
	if saved.Theme != want {
		t.Fatalf("saved theme = %q, want %q", saved.Theme, want)
	}
	if m.settings.open {
		t.Fatal("enter did not close settings")
	}
	if m.theme.Name != want {
		t.Fatalf("theme after enter = %q, want %q", m.theme.Name, want)
	}
}

func TestSettingsSaveFailureKeepsSessionTheme(t *testing.T) {
	defer SetTheme(theme.Dark)
	old := saveConfig
	defer func() { saveConfig = old }()
	saveConfig = func(config.Config) error { return errors.New("disk full") }

	m := newViewer(t, 120, 40)
	m = pressKey(m, runeKey(','))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyDown})
	want := m.theme.Name
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.theme.Name != want {
		t.Fatalf("theme after failed save = %q, want %q", m.theme.Name, want)
	}
	if !strings.Contains(m.flash, "session") {
		t.Fatalf("flash = %q, want a save warning", m.flash)
	}
}

func TestSettingsForcesSidebarOnNarrowTerminal(t *testing.T) {
	defer SetTheme(theme.Dark)
	m := newViewer(t, 60, 20)
	if m.outline.visible {
		t.Fatal("sidebar should be hidden at 60 columns")
	}
	m = pressKey(m, runeKey(','))
	if !m.outline.visible {
		t.Fatal("settings did not open the sidebar slot")
	}
	if !strings.Contains(m.View(), "Settings") {
		t.Fatal("settings panel missing on narrow terminal")
	}
}

// TestSettingsCommitKeepsOtherPreferences: saving a theme must not erase the
// other keys in the config file.
func TestSettingsCommitKeepsOtherPreferences(t *testing.T) {
	defer SetTheme(theme.Dark)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	off := false
	if err := config.Save(config.Config{Theme: "dark", SaveSessions: &off}); err != nil {
		t.Fatal(err)
	}
	m := newViewer(t, 120, 40)
	m = pressKey(m, runeKey(','))
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyDown})
	want := m.theme.Name
	pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != want || c.SessionsEnabled() {
		t.Fatalf("config after commit = %+v, want theme %q with sessions off", c, want)
	}
}

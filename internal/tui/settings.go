package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/michaelishri/exmd/internal/config"
	"github.com/michaelishri/exmd/internal/theme"
)

type settingsPage int

const (
	settingsMenu settingsPage = iota
	settingsThemePage
)

// settingsModel holds the settings panel state. "original" is the theme in
// effect when the panel was opened, so Esc can restore it without a save.
type settingsModel struct {
	open     bool
	page     settingsPage
	cursor   int
	offset   int
	original theme.Theme
	themes   []theme.Theme
}

// saveConfig is a variable so tests can intercept writes.
var saveConfig = config.Save

func newSettingsModel(current theme.Theme) settingsModel {
	s := settingsModel{original: current}
	for _, name := range theme.Names() {
		if name == "auto" {
			continue
		}
		th, err := theme.Resolve(name, true)
		if err != nil {
			continue
		}
		if th.Name == current.Name {
			s.cursor = len(s.themes)
		}
		s.themes = append(s.themes, th)
	}
	return s
}

// openSettings captures the current theme and forces the sidebar slot open
// so the panel has somewhere to render.
func (m *Model) openSettings() {
	m.settings = newSettingsModel(m.theme)
	m.flash = ""
	m.settings.open = true
	m.relayout()
}

// closeSettings restores the theme from before the panel was opened.
func (m *Model) closeSettings() {
	if !m.settings.open {
		return
	}
	m.settings.open = false
	m.applyTheme(m.settings.original)
}

// applyTheme re-points every layer at th: the package-level lipgloss styles,
// this model's theme, and the rendered document, outline and inline boxes.
func (m *Model) applyTheme(th theme.Theme) {
	SetTheme(th)
	m.theme = th
	m.relayout()
}

// settingsFooterText is the hint shown in the sidebar footer while settings
// are open, so it is never repeated in the left footer band.
func (m Model) settingsFooterText() string {
	if !m.settings.open {
		return ""
	}
	if m.settings.page == settingsThemePage {
		return "↑↓ preview · enter save · esc cancel"
	}
	return "enter open · esc close"
}

// previewTheme moves the cursor and immediately repaints the whole app with
// the highlighted theme.
func (m *Model) previewTheme(index int) {
	if len(m.settings.themes) == 0 {
		return
	}
	m.settings.cursor = clamp(index, 0, len(m.settings.themes)-1)
	m.applyTheme(m.settings.themes[m.settings.cursor])

	rows := max(1, m.paneHeight-4)
	if m.settings.cursor < m.settings.offset {
		m.settings.offset = m.settings.cursor
	}
	if m.settings.cursor >= m.settings.offset+rows {
		m.settings.offset = m.settings.cursor - rows + 1
	}
}

// commitTheme keeps the previewed theme and saves it. A write failure still
// leaves the new theme active for this session.
func (m *Model) commitTheme() {
	if len(m.settings.themes) == 0 {
		return
	}
	th := m.settings.themes[m.settings.cursor]
	m.settings.open = false
	m.applyTheme(th)
	// Keep every other saved preference: only the theme changes here.
	cfg, _ := config.Load()
	cfg.Theme = th.Name
	if err := saveConfig(cfg); err != nil {
		m.flash = "theme kept for this session: " + err.Error()
	} else {
		m.flash = "theme set to " + th.Name
	}
}

// handleSettingsKey routes keys while the panel is open. ctrl+c passes
// through so the user can always quit.
func (m *Model) handleSettingsKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if msg.String() == "ctrl+c" {
		return nil, false
	}
	if m.settings.page == settingsMenu {
		switch msg.String() {
		case "esc", "v", ",":
			m.closeSettings()
		case "enter", "j", "down", "tab":
			m.settings.page = settingsThemePage
		}
		return nil, true
	}

	switch msg.String() {
	case "esc":
		m.closeSettings()
	case "j", "down":
		m.previewTheme(m.settings.cursor + 1)
	case "k", "up":
		m.previewTheme(m.settings.cursor - 1)
	case "enter":
		m.commitTheme()
	case "tab", "shift+tab":
		m.settings.page = settingsMenu
	}
	return nil, true
}

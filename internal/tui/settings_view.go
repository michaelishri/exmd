package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// settingsView renders the settings panel in the sidebar slot. Colors come
// from the active theme, so moving the cursor restyles the panel as well as
// the document behind it.
func (m Model) settingsView(height int) string {
	if height <= 0 {
		return ""
	}
	width := m.outline.width
	if width <= 0 {
		width = sidebarCols
	}

	base := lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color(string(m.theme.BackgroundPanel))).
		Foreground(lipgloss.Color(string(m.theme.Text)))
	title := base.Bold(true).Foreground(lipgloss.Color(string(m.theme.Bright)))
	selected := lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color(string(m.theme.Primary))).
		Foreground(lipgloss.Color(string(m.theme.Background))).
		Bold(true)

	rows := make([]string, height)
	blank := base.Render("")
	for i := range rows {
		rows[i] = blank
	}

	switch m.settings.page {
	case settingsMenu:
		if len(rows) > 1 {
			rows[1] = title.Render(" Settings")
		}
		if len(rows) > 3 {
			rows[3] = selected.Render("  Theme")
		}
	case settingsThemePage:
		if len(rows) > 1 {
			rows[1] = title.Render(" Settings / Theme")
		}
		visible := max(1, height-4)
		for i := 0; i < visible; i++ {
			index := m.settings.offset + i
			if index >= len(m.settings.themes) || 3+i >= len(rows) {
				break
			}
			label := fmt.Sprintf("  %2d. %s", index+1, m.settings.themes[index].Name)
			if index == m.settings.cursor {
				rows[3+i] = selected.Render(label)
			} else {
				rows[3+i] = base.Render(label)
			}
		}
	}
	return strings.Join(rows, "\n")
}

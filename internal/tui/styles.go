package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"

	"github.com/michaelishri/md-exec/internal/theme"
)

// t is the active theme. A --theme flag can re-point these later; the styles
// below are built from it once.
var t = theme.Dark

// OpenCode-adjacent visual language: no borders — separation comes from
// background shades — muted chrome, and color reserved for state.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Text))

	pathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextMuted))

	chipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextMuted)).
			Background(lipgloss.Color(t.BackgroundElement)).
			Padding(0, 1)

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextMuted)).
			Background(lipgloss.Color(t.BackgroundPanel))

	selRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Text)).
			Background(lipgloss.Color(t.BackgroundElement))

	markerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Primary)).
			Background(lipgloss.Color(t.BackgroundElement)).
			Bold(true)

	tabTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Primary)).
			Background(lipgloss.Color(t.BackgroundPanel)).
			Bold(true)

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextMuted)).
			Background(lipgloss.Color(t.BackgroundPanel))

	sepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Border))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextMuted))

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Error))

	accentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Primary)).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Success))

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Warning))
)

// tuneHelp mutes the help model's palette so the status bar whispers.
func tuneHelp(h *help.Model) {
	s := h.Styles
	s.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted))
	s.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted))
	s.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border))
	s.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted))
	s.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted))
	s.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border))
	s.Ellipsis = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted))
	h.Styles = s
}

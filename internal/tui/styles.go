package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/michaelishri/md-exec/internal/theme"
)

// t is the active theme; the styles below are derived from it and rebuilt by
// SetTheme.
var t = theme.Dark

// OpenCode-adjacent visual language: no borders — separation comes from
// background shades — muted chrome, and color reserved for state.
var (
	rowStyle      lipgloss.Style
	selRowStyle   lipgloss.Style
	markerStyle   lipgloss.Style
	tabTitleStyle lipgloss.Style

	emptyStateStyle lipgloss.Style
	sepStyle        lipgloss.Style
	infoStyle       lipgloss.Style
	errStyle        lipgloss.Style
	accentStyle     lipgloss.Style
	successStyle    lipgloss.Style
	warnStyle       lipgloss.Style
)

func init() { buildStyles() }

// SetTheme re-points the active theme and rebuilds every derived style. Call
// it before New: the model captures the theme once.
func SetTheme(th theme.Theme) {
	t = th
	buildStyles()
}

func buildStyles() {
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
}

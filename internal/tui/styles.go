package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"

	"github.com/michaelishri/md-exec/internal/theme"
)

// t is the active theme; the styles below are derived from it and rebuilt by
// SetTheme.
var t = theme.Dark

// OpenCode-adjacent visual language: no borders — separation comes from
// background shades — muted chrome, and color reserved for state.
var (
	titleStyle lipgloss.Style
	pathStyle  lipgloss.Style
	chipStyle  lipgloss.Style

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

	// Focus treatment: when the outline pane holds focus, its whole column
	// lifts one background shade so the focused pane is visible at a glance.
	rowFocusStyle        lipgloss.Style
	selRowFocusStyle     lipgloss.Style
	tabTitleFocusStyle   lipgloss.Style
	emptyStateFocusStyle lipgloss.Style
)

func init() { buildStyles() }

// SetTheme re-points the active theme and rebuilds every derived style. Call
// it before New: the model captures the theme once.
func SetTheme(th theme.Theme) {
	t = th
	buildStyles()
}

func buildStyles() {
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

	// Focus treatment: the outline column lifts to the element shade.
	rowFocusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted)).
		Background(lipgloss.Color(t.BackgroundElement))

	selRowFocusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.BackgroundElement)).
		Bold(true)

	tabTitleFocusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Background(lipgloss.Color(t.BackgroundElement)).
		Bold(true)

	emptyStateFocusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted)).
		Background(lipgloss.Color(t.BackgroundElement))
}

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

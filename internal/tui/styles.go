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
	tabTitleStyle lipgloss.Style

	emptyStateStyle lipgloss.Style
	sepStyle        lipgloss.Style
	infoStyle       lipgloss.Style
	errStyle        lipgloss.Style
	accentStyle     lipgloss.Style
	successStyle    lipgloss.Style
	warnStyle       lipgloss.Style

	// sideTitleStyle renders the sidebar's title row — the document's
	// first h1: bright bold text on the panel background.
	sideTitleStyle lipgloss.Style

	// The sidebar's resting shades: the same surfaces on the muted panel
	// background, used while the sidebar does not hold focus. Focus lifts
	// the strip to the full panel color.
	rowMutedStyle        lipgloss.Style
	rowHiMutedStyle      lipgloss.Style
	sideTitleMutedStyle  lipgloss.Style
	emptyStateMutedStyle lipgloss.Style

	// rowHiStyle shades top-level heading rows (h1/h2) with bright text;
	// deeper heading levels use rowStyle's muted text.
	rowHiStyle lipgloss.Style

	// gutterBarStyle colors the selection rail at the left edge of the
	// text area.
	gutterBarStyle lipgloss.Style

	// frameStyle draws the border around the entire viewport — the muted
	// chrome color: clearly visible against the canvas without shouting.
	// It demotes to this while the sidebar holds focus.
	frameStyle lipgloss.Style

	// focusFrameStyle is the viewport frame color while the text area
	// holds focus: the frame is the text area's border.
	focusFrameStyle lipgloss.Style

	// sideBorderStyle draws the sidebar's focus border: a single primary
	// line along the strip's left edge.
	sideBorderStyle lipgloss.Style
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

	// The selected row inverts into the primary (state) color: the
	// background carries the selection, and bold keeps it visible in the
	// notty theme where color is unavailable.
	selRowStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Background)).
		Background(lipgloss.Color(t.Primary)).
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

	sideTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.BackgroundPanel)).
		Bold(true)

	rowHiStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.BackgroundPanel))

	// The resting shades: identical to the focused surfaces but on the
	// muted panel background.
	rowMutedStyle = rowStyle.Background(lipgloss.Color(t.BackgroundPanelMuted))
	rowHiMutedStyle = rowHiStyle.Background(lipgloss.Color(t.BackgroundPanelMuted))
	sideTitleMutedStyle = sideTitleStyle.Background(lipgloss.Color(t.BackgroundPanelMuted))
	emptyStateMutedStyle = emptyStateStyle.Background(lipgloss.Color(t.BackgroundPanelMuted))

	gutterBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary))

	frameStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted))

	focusFrameStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary))

	// sideBorderStyle draws the sidebar's focus border: a single primary
	// line along the strip's left edge.
	sideBorderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary))
}

package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/michaelishri/exmd/internal/theme"
)

// t is the active theme; the styles below are derived from it and rebuilt by
// SetTheme.
var t = theme.Dark

// OpenCode-adjacent visual language: no borders — separation comes from
// background shades — muted chrome, and color reserved for state.
var (
	rowStyle    lipgloss.Style
	selRowStyle lipgloss.Style

	emptyStateStyle lipgloss.Style
	infoStyle       lipgloss.Style

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
	// text area; the three state styles color it while a block executes.
	gutterBarStyle lipgloss.Style
	railRunStyle   lipgloss.Style
	railDoneStyle  lipgloss.Style
	railFailStyle  lipgloss.Style

	// Brighter, bold variants mark which executed block holds the cursor.
	railRunActiveStyle  lipgloss.Style
	railDoneActiveStyle lipgloss.Style
	railFailActiveStyle lipgloss.Style

	// execEdgeStyle is the separator under an executable code block: muted
	// foreground on the exec shade.
	execEdgeStyle lipgloss.Style
)

func init() { buildStyles() }

// SetTheme re-points the active theme and rebuilds every derived style. Call
// it before New: the model captures the theme once.
func SetTheme(th theme.Theme) {
	t = th
	buildStyles()
}

func buildStyles() {
	// Focused sidebar rows sit on the raised shade; the resting strip uses
	// the darker panel shade so it is visible for its full height.
	rowStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted)).
		Background(lipgloss.Color(t.BackgroundElement))

	// The selected row inverts into the primary (state) color: the
	// background carries the selection, and bold keeps it visible in the
	// notty theme where color is unavailable.
	selRowStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Background)).
		Background(lipgloss.Color(t.Primary)).
		Bold(true)

	emptyStateStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted)).
		Background(lipgloss.Color(t.BackgroundElement))

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted))

	sideTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.BackgroundElement)).
		Bold(true)

	rowHiStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.BackgroundElement))

	// The sidebar's resting strip is painted on the panel shade for its
	// full height. Focus lifts it to the raised element shade; text colors
	// stay the same in both states.
	rowMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted)).
		Background(lipgloss.Color(t.BackgroundPanel))
	rowHiMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.BackgroundPanel))
	sideTitleMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.BackgroundPanel)).
		Bold(true)
	emptyStateMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted)).
		Background(lipgloss.Color(t.BackgroundPanel))

	gutterBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary))
	railRunStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Running))
	railDoneStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Success))
	railFailStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Error))

	// Same hues, one shade brighter and bold, for the selected block.
	railRunActiveStyle = railRunStyle.
		Foreground(lipgloss.Color(t.RunningBright)).Bold(true)
	railDoneActiveStyle = railDoneStyle.
		Foreground(lipgloss.Color(t.SuccessBright)).Bold(true)
	railFailActiveStyle = railFailStyle.
		Foreground(lipgloss.Color(t.ErrorBright)).Bold(true)

	// The exec block's output separator uses the same chrome tone as the
	// footer hairline.
	execEdgeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Separator)).
		Background(lipgloss.Color(t.ExecPanel))
}

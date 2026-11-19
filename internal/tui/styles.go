package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/michaelishri/md-exec/internal/render"
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
	// text area; the three state styles color it while a block executes.
	gutterBarStyle lipgloss.Style
	railRunStyle   lipgloss.Style
	railDoneStyle  lipgloss.Style
	railFailStyle  lipgloss.Style

	// Brighter, bold variants mark which executed block holds the cursor.
	railRunActiveStyle  lipgloss.Style
	railDoneActiveStyle lipgloss.Style
	railFailActiveStyle lipgloss.Style

	// sideBorderStyle draws the sidebar's left border line in the same
	// muted chrome color as the separator under an executable code block.
	sideBorderStyle lipgloss.Style

	// execEdgeStyle is that separator: muted foreground on the exec shade.
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

	// The sidebar's resting shades: no background at all — the strip
	// blends into the page exactly like the text area — with the same
	// text colors as the focused surfaces. Focus lifts the strip onto the
	// panel background.
	rowMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted))
	rowHiMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text))
	sideTitleMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Bold(true)
	emptyStateMutedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted))

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
		Foreground(lipgloss.Color("#ffc9a3")).Bold(true)
	railDoneActiveStyle = railDoneStyle.
		Foreground(lipgloss.Color("#a5e8a8")).Bold(true)
	railFailActiveStyle = railFailStyle.
		Foreground(lipgloss.Color("#ff9b9b")).Bold(true)

	// sideBorderStyle draws the sidebar's focus border: a single primary
	// line along the strip's left edge.
	sideBorderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted))

	execEdgeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.TextMuted)).
		Background(lipgloss.Color(render.ExecColor()))
}

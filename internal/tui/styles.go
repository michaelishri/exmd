package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/michaelishri/exmd/internal/theme"
)

// t is the theme new viewers start with; SetTheme changes it. A running
// viewer keeps its own theme and styles (Model.theme, Model.styles).
var t = theme.Dark

// styles are the lipgloss styles derived from one theme. OpenCode-adjacent
// visual language: no borders — separation comes from background shades —
// muted chrome, and color reserved for state.
type styles struct {
	row    lipgloss.Style
	selRow lipgloss.Style

	emptyState lipgloss.Style
	info       lipgloss.Style

	// sideTitle renders the sidebar's title row — the document's
	// first h1: bright bold text on the panel background.
	sideTitle lipgloss.Style

	// The sidebar's resting shades: the same surfaces on the muted panel
	// background, used while the sidebar does not hold focus. Focus lifts
	// the strip to the full panel color.
	rowMuted        lipgloss.Style
	rowHiMuted      lipgloss.Style
	sideTitleMuted  lipgloss.Style
	emptyStateMuted lipgloss.Style

	// rowHi shades top-level heading rows (h1/h2) with bright text;
	// deeper heading levels use row's muted text.
	rowHi lipgloss.Style

	// gutterBar colors the selection rail at the left edge of the
	// text area; the three state styles color it while a block executes.
	gutterBar lipgloss.Style
	railRun   lipgloss.Style
	railDone  lipgloss.Style
	railFail  lipgloss.Style

	// Brighter, bold variants mark which executed block holds the cursor.
	railRunActive  lipgloss.Style
	railDoneActive lipgloss.Style
	railFailActive lipgloss.Style

	// execEdge is the separator under an executable code block: muted
	// foreground on the exec shade.
	execEdge lipgloss.Style
}

// SetTheme sets the theme new viewers start with. Call it before New.
func SetTheme(th theme.Theme) {
	t = th
}

// newStyles derives every style from th.
func newStyles(th theme.Theme) styles {
	var s styles
	// Focused sidebar rows sit on the raised shade; the resting strip uses
	// the darker panel shade so it is visible for its full height.
	s.row = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.TextMuted)).
		Background(lipgloss.Color(th.BackgroundElement))

	// The selected row inverts into the primary (state) color: the
	// background carries the selection, and bold keeps it visible in the
	// notty theme where color is unavailable.
	s.selRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Background)).
		Background(lipgloss.Color(th.Primary)).
		Bold(true)

	s.emptyState = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.TextMuted)).
		Background(lipgloss.Color(th.BackgroundElement))

	s.info = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.TextMuted))

	s.sideTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Text)).
		Background(lipgloss.Color(th.BackgroundElement)).
		Bold(true)

	s.rowHi = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Text)).
		Background(lipgloss.Color(th.BackgroundElement))

	// The sidebar's resting strip is painted on the panel shade for its
	// full height. Focus lifts it to the raised element shade; text colors
	// stay the same in both states.
	s.rowMuted = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.TextMuted)).
		Background(lipgloss.Color(th.BackgroundPanel))
	s.rowHiMuted = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Text)).
		Background(lipgloss.Color(th.BackgroundPanel))
	s.sideTitleMuted = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Text)).
		Background(lipgloss.Color(th.BackgroundPanel)).
		Bold(true)
	s.emptyStateMuted = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.TextMuted)).
		Background(lipgloss.Color(th.BackgroundPanel))

	s.gutterBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Primary))
	s.railRun = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Running))
	s.railDone = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Success))
	s.railFail = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Error))

	// Same hues, one shade brighter and bold, for the selected block.
	s.railRunActive = s.railRun.
		Foreground(lipgloss.Color(th.RunningBright)).Bold(true)
	s.railDoneActive = s.railDone.
		Foreground(lipgloss.Color(th.SuccessBright)).Bold(true)
	s.railFailActive = s.railFail.
		Foreground(lipgloss.Color(th.ErrorBright)).Bold(true)

	// The exec block's output separator uses the same chrome tone as the
	// footer hairline.
	s.execEdge = lipgloss.NewStyle().
		Foreground(lipgloss.Color(th.Separator)).
		Background(lipgloss.Color(th.ExecPanel))
	return s
}

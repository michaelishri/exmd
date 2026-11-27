package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/render"
	"github.com/michaelishri/exmd/internal/termsafe"
)

const (
	tabHeadings = 0
	tabBlocks   = 1
)

// outlineEntry is one navigable row of the sidebar.
type outlineEntry struct {
	label      string
	level      int // heading level for indentation (headings tab)
	renderLine int // 0-based rendered line to jump to
}

// outlineModel is the right-hand sidebar: document outline (headings) or
// executable blocks, with a single unified cursor that mirrors the content
// scroll position.
type outlineModel struct {
	tab     int
	title   string // the document's first h1
	sel     int
	visible bool
	width   int
	entries []outlineEntry
}

func newOutline() outlineModel {
	return outlineModel{tab: tabHeadings}
}

// sidebarTitle returns the title row's text and the index of the first h1,
// which stays out of the outline entries. A frontmatter title wins; the h1
// is the fallback, then the document title (file name) when there is no h1.
func sidebarTitle(d *doc.Document) (string, int) {
	firstH1 := -1
	for i, h := range d.Headings {
		if h.Level == 1 {
			firstH1 = i
			break
		}
	}
	if d.Title != "" {
		return d.Title, firstH1
	}
	if firstH1 >= 0 {
		return d.Headings[firstH1].Text, firstH1
	}
	return d.HeadingText(), -1
}

// rebuild recomputes entries from the current render and tab.
func (o *outlineModel) rebuild(d *doc.Document, out *render.Output, width int) {
	o.width = width
	title, skip := sidebarTitle(d)
	o.title = termsafe.Visible(title)
	o.entries = o.entries[:0]
	switch o.tab {
	case tabHeadings:
		// The first h1 is the title row, not a selectable entry.
		for i, h := range d.Headings {
			if i == skip {
				continue
			}
			if i >= len(out.Headings) {
				break
			}
			label := termsafe.Visible(h.Text)
			if n := out.Headings[i].Number; n != "" {
				label = n + " " + label
			}
			if strings.TrimSpace(label) == "" {
				label = "(untitled)"
			}
			o.entries = append(o.entries, outlineEntry{
				label:      label,
				level:      h.Level,
				renderLine: out.Headings[i].Line,
			})
		}
	case tabBlocks:
		for i, bm := range out.Blocks {
			if i >= len(d.Blocks) {
				break
			}
			o.entries = append(o.entries, outlineEntry{
				label:      bm.Label,
				renderLine: bm.Line,
			})
		}
	}
	if o.sel >= len(o.entries) {
		o.sel = len(o.entries) - 1
	}
}

// move shifts the cursor and reports whether anything changed. Index -1 is
// the title row: it sits above the first outlineEntry and takes the reader to the
// very top of the document, so moving up from the first outlineEntry lands there.
func (o *outlineModel) move(delta int) bool {
	if len(o.entries) == 0 {
		return false
	}
	n := o.sel + delta
	if n < -1 {
		n = -1
	}
	if n >= len(o.entries) {
		n = len(o.entries) - 1
	}
	if n == o.sel {
		return false
	}
	o.sel = n
	return true
}

// first selects the title row, the very top of the outline.
func (o *outlineModel) first() bool {
	if o.sel == -1 {
		return false
	}
	o.sel = -1
	return true
}

func (o *outlineModel) last() bool {
	if len(o.entries) == 0 || o.sel == len(o.entries)-1 {
		return false
	}
	o.sel = len(o.entries) - 1
	return true
}

// Sidebar geometry: one base column of left padding at the top level
// (deeper heading levels indent from there), and one blank pad row at the
// top and bottom of the strip.
const panePad = 1

// view renders the sidebar as a full-height shaded column: a blank pad row,
// the document's title (two columns in), a blank pad row, then the packed
// entries, with blank rows padding the bottom. The layout is identical
// whether or not the sidebar holds focus — focus is expressed by shading
// alone. While the sidebar does not hold focus the strip rests on the panel
// shade; focus lifts it to the raised element shade.
func (o *outlineModel) view(height int, focused bool) string {
	if !o.visible {
		return ""
	}
	rows := make([]string, height)

	// The strip's surfaces: the resting (muted) shades while unfocused,
	// the full panel shades while focused. Selected rows always render on
	// the primary background.
	resting, bright := rowStyle, rowHiStyle
	titleBg, emptyBg := sideTitleStyle, emptyStateStyle
	if !focused {
		resting, bright = rowMutedStyle, rowHiMutedStyle
		titleBg, emptyBg = sideTitleMutedStyle, emptyStateMutedStyle
	}

	// The whole column is painted; the bottom row stays blank.
	for r := range rows {
		rows[r] = resting.Width(o.width).Render("")
	}
	setRow := func(r int, s string) {
		if 0 <= r && r < height {
			rows[r] = s
		}
	}

	// Title row: the document's title, two columns in. It doubles as
	// the topmost selectable row (sel == -1): when the cursor is on it,
	// it carries the selection background like any other row, and the
	// content scrolls to the very top.
	const titleRow = 1
	const titlePad = 2
	titleText := strings.Repeat(" ", titlePad) + ansi.Truncate(o.title, o.width-titlePad-1, "…")
	if o.sel < 0 {
		setRow(titleRow, selRowStyle.Width(o.width).Render(titleText))
	} else {
		setRow(titleRow, titleBg.Width(o.width).Render(titleText))
	}

	top := 3 // top pad + title + pad
	bottomPad := 1
	visible := height - top - bottomPad
	if visible < 1 {
		visible = 1
	}
	start := 0
	if len(o.entries) > visible {
		start = o.sel - visible/2
		if start < 0 {
			start = 0
		}
		if start+visible > len(o.entries) {
			start = len(o.entries) - visible
		}
	}

	if len(o.entries) == 0 {
		empty := "no headings"
		if o.tab == tabBlocks {
			empty = "no blocks"
		}
		setRow(top, emptyBg.Width(o.width).Render(strings.Repeat(" ", panePad)+empty))
		return strings.Join(rows, "\n")
	}
	pad := strings.Repeat(" ", panePad)
	for r := 0; r < visible; r++ {
		row := top + r
		if row >= height-bottomPad {
			break
		}
		idx := start + r
		if idx >= len(o.entries) {
			break
		}
		e := o.entries[idx]
		indent := ""
		if o.tab == tabHeadings {
			n := e.level - 1
			if n > 3 {
				n = 3
			}
			if n < 0 {
				n = 0
			}
			indent = strings.Repeat("  ", n)
		}
		label := indent + e.label
		// The selection is carried entirely by the row's background: the
		// text keeps its column, so nothing shifts when the cursor moves.
		text := ansi.Truncate(label, o.width-panePad-1, "…")
		if idx == o.sel {
			setRow(row, selRowStyle.Width(o.width).Render(pad+text))
		} else {
			// h1/h2 rows read bright; deeper heading levels stay muted.
			rowBg := resting
			if o.tab == tabHeadings && e.level <= 2 {
				rowBg = bright
			}
			setRow(row, rowBg.Width(o.width).Render(pad+text))
		}
	}
	return strings.Join(rows, "\n")
}

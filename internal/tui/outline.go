package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/md-exec/internal/doc"
	"github.com/michaelishri/md-exec/internal/render"
)

const (
	tabHeadings = 0
	tabBlocks   = 1
)

// entry is one navigable row of the sidebar.
type entry struct {
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
	entries []entry
}

func newOutline() outlineModel {
	return outlineModel{tab: tabHeadings}
}

// firstH1 returns the text of the document's first level-1 heading. It is
// typically the first body line, but not necessarily — the scan makes no
// position assumption. Documents without an h1 fall back to the title.
func firstH1(d *doc.Document) string {
	for _, h := range d.Headings {
		if h.Level == 1 {
			return h.Text
		}
	}
	return d.HeadingText()
}

// rebuild recomputes entries from the current render and tab.
func (o *outlineModel) rebuild(d *doc.Document, out *render.Output, width int) {
	o.width = width
	o.title = firstH1(d)
	o.entries = o.entries[:0]
	switch o.tab {
	case tabHeadings:
		// The first h1 is the sidebar's title row, not a selectable
		// entry — the outline starts with whatever follows it.
		skip := -1
		for i, h := range d.Headings {
			if h.Level == 1 {
				skip = i
				break
			}
		}
		for i, h := range d.Headings {
			if i == skip {
				continue
			}
			if i >= len(out.Headings) {
				break
			}
			label := h.Text
			if strings.TrimSpace(label) == "" {
				label = "(untitled)"
			}
			o.entries = append(o.entries, entry{
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
			o.entries = append(o.entries, entry{
				label:      bm.Label,
				renderLine: bm.Line,
			})
		}
	}
	if o.sel >= len(o.entries) {
		o.sel = len(o.entries) - 1
	}
}

// move shifts the cursor and reports whether anything changed.
func (o *outlineModel) move(delta int) bool {
	if len(o.entries) == 0 {
		return false
	}
	n := o.sel + delta
	if n < 0 {
		n = 0
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

func (o *outlineModel) first() bool {
	if o.sel == 0 || len(o.entries) == 0 {
		return false
	}
	o.sel = 0
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
// (deeper heading levels indent from there), and one blank row inset at the
// top and bottom of the column.
const panePad = 1

// view renders the sidebar as a full-height shaded column: a blank pad
// row, the document's first h1 as the title row (two columns in), a blank
// pad row, then the entries inset one blank row at the bottom of the
// column. The viewer's outer margins (one row top/bottom, one column
// right) sit outside the strip.
func (o *outlineModel) view(height int) string {
	if !o.visible {
		return ""
	}
	rows := make([]string, height)
	// The whole column is painted; the bottom row stays blank.
	for r := range rows {
		rows[r] = rowStyle.Width(o.width).Render("")
	}
	setRow := func(r int, s string) {
		if 0 <= r && r < height {
			rows[r] = s
		}
	}

	// Blank pad, then the title row, then a blank pad.
	const titlePad = 2
	setRow(1, sideTitleStyle.Width(o.width).Render(
		strings.Repeat(" ", titlePad)+ansi.Truncate(o.title, o.width-titlePad-1, "…")))

	top := 3 // pad + title + pad
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
		setRow(top, emptyStateStyle.Width(o.width).Render(strings.Repeat(" ", panePad)+empty))
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
		if idx == o.sel {
			// The marker occupies two cells after the base pad, so the
			// text budget is two columns tighter to keep the right edge.
			text := ansi.Truncate(label, o.width-panePad-3, "…")
			setRow(row, selRowStyle.Width(o.width).Render(pad+markerStyle.Render("▸")+" "+text))
		} else {
			// h1/h2 rows read bright; deeper heading levels stay muted.
			rowBg := rowStyle
			if o.tab == tabHeadings && e.level <= 2 {
				rowBg = rowHiStyle
			}
			text := ansi.Truncate(label, o.width-panePad-1, "…")
			setRow(row, rowBg.Width(o.width).Render(pad+text))
		}
	}
	return strings.Join(rows, "\n")
}

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
	sel     int
	visible bool
	width   int
	entries []entry
}

func newOutline() outlineModel {
	return outlineModel{tab: tabHeadings}
}

// rebuild recomputes entries from the current render and tab.
func (o *outlineModel) rebuild(d *doc.Document, out *render.Output, width int) {
	o.width = width
	o.entries = o.entries[:0]
	switch o.tab {
	case tabHeadings:
		for i, h := range d.Headings {
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
// (deeper heading levels indent from there).
const panePad = 1

// view renders the sidebar as a shaded strip that hugs the entries: one
// blank pad row above and below, and nothing painted below the strip — so
// there are always exactly two blank rows under the last entry (pad plus
// the viewer's bottom margin). When the list is longer than the window the
// strip fills the height.
func (o *outlineModel) view(height int) string {
	if !o.visible {
		return ""
	}
	rows := make([]string, height)
	blank := rowStyle.Width(o.width).Render("")
	setRow := func(r int, s string) {
		if 0 <= r && r < height {
			rows[r] = s
		}
	}

	n := len(o.entries)
	stripH := n + 2
	if n == 0 {
		stripH = 3
	}
	if stripH > height {
		stripH = height
	}
	visible := stripH - 2
	if visible < 1 {
		visible = 1
	}
	start := 0
	if n > visible {
		start = o.sel - visible/2
		if start < 0 {
			start = 0
		}
		if start+visible > n {
			start = n - visible
		}
	}

	setRow(0, blank) // top pad
	drawn := 0
	if n == 0 {
		empty := "no headings"
		if o.tab == tabBlocks {
			empty = "no blocks"
		}
		setRow(1, emptyStateStyle.Width(o.width).Render(strings.Repeat(" ", panePad)+empty))
		drawn = 1
	} else {
		pad := strings.Repeat(" ", panePad)
		for r := 0; r < visible; r++ {
			idx := start + r
			if idx >= n {
				break
			}
			e := o.entries[idx]
			indent := ""
			if o.tab == tabHeadings {
				lvl := e.level - 1
				if lvl > 3 {
					lvl = 3
				}
				if lvl < 0 {
					lvl = 0
				}
				indent = strings.Repeat("  ", lvl)
			}
			label := indent + e.label
			if idx == o.sel {
				// The marker occupies two cells after the base pad, so the
				// text budget is two columns tighter to keep the right edge.
				text := ansi.Truncate(label, o.width-panePad-3, "…")
				setRow(1+r, selRowStyle.Width(o.width).Render(pad+markerStyle.Render("▸")+" "+text))
			} else {
				text := ansi.Truncate(label, o.width-panePad-1, "…")
				setRow(1+r, rowStyle.Width(o.width).Render(pad+text))
			}
			drawn = r + 1
		}
	}
	setRow(1+drawn, blank) // bottom pad; below it the column is unpainted
	return strings.Join(rows, "\n")
}

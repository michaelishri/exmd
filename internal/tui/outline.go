package tui

import (
	"fmt"
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

// view renders the sidebar as `height` shaded rows.
func (o *outlineModel) view(height int) string {
	if !o.visible {
		return ""
	}
	rows := make([]string, height)
	title := "Outline"
	if o.tab == tabBlocks {
		title = "Blocks"
	}
	rows[0] = tabTitleStyle.Width(o.width).Render(" " + title)

	visible := height - 1
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
		rows[1] = emptyStateStyle.Width(o.width).Render(" " + empty)
	}

	for r := 1; r < height; r++ {
		if rows[r] != "" {
			continue
		}
		idx := start + r - 1
		if idx >= len(o.entries) {
			rows[r] = rowStyle.Width(o.width).Render("")
			continue
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
		text := ansi.Truncate(indent+e.label, o.width-3, "…")
		if idx == o.sel {
			rows[r] = selRowStyle.Width(o.width).Render(markerStyle.Render("▸") + " " + text)
		} else {
			rows[r] = rowStyle.Width(o.width).Render("  " + text)
		}
	}
	return strings.Join(rows, "\n")
}

var _ = fmt.Sprintf

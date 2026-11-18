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

// Sidebar geometry: left padding inside rows and a blank shaded row between
// entries, so the column breathes instead of packing rows edge to edge.
const (
	panePad  = 2
	entryGap = 1
)

// view renders the sidebar as `height` shaded rows. focused selects the
// one-shade-up treatment for the whole column, so tabbing between the
// content and the sidebar is visible at a glance.
func (o *outlineModel) view(height int, focused bool) string {
	if !o.visible {
		return ""
	}
	rowBg, selBg, titleBg, emptyBg := rowStyle, selRowStyle, tabTitleStyle, emptyStateStyle
	if focused {
		rowBg, selBg, titleBg, emptyBg = rowFocusStyle, selRowFocusStyle, tabTitleFocusStyle, emptyStateFocusStyle
	}

	rows := make([]string, height)
	title := "Outline"
	if o.tab == tabBlocks {
		title = "Blocks"
	}
	rows[0] = titleBg.Width(o.width).Render(strings.Repeat(" ", panePad) + title)

	// The body starts below the title and its spacer row; every entry is
	// followed by a blank shaded row.
	bodyTop := 2
	visible := height - bodyTop
	if visible < 1 {
		visible = 1
	}
	slots := (visible + entryGap) / (entryGap + 1)
	if slots < 1 {
		slots = 1
	}
	start := 0
	if len(o.entries) > slots {
		start = o.sel - (slots-1)/2
		if start < 0 {
			start = 0
		}
		if start+slots > len(o.entries) {
			start = len(o.entries) - slots
		}
	}

	// The whole column is painted; entries overwrite their slot lines.
	for r := bodyTop; r < height; r++ {
		rows[r] = rowBg.Width(o.width).Render("")
	}
	if len(o.entries) == 0 {
		empty := "no headings"
		if o.tab == tabBlocks {
			empty = "no blocks"
		}
		rows[bodyTop] = emptyBg.Width(o.width).Render(strings.Repeat(" ", panePad) + empty)
		return strings.Join(rows, "\n")
	}

	pad := strings.Repeat(" ", panePad)
	textBudget := o.width - 2*panePad - 2
	for si := 0; si < slots; si++ {
		r := bodyTop + si*(entryGap+1)
		if r >= height {
			break
		}
		idx := start + si
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
		text := ansi.Truncate(indent+e.label, textBudget, "…")
		if idx == o.sel {
			rows[r] = selBg.Width(o.width).Render(pad + markerStyle.Render("▸") + " " + text)
		} else {
			rows[r] = rowBg.Width(o.width).Render(pad + text)
		}
	}
	return strings.Join(rows, "\n")
}

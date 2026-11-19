package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/theme"
)

// DefaultWidth is the render width when none is supplied.
const DefaultWidth = 80

// newRenderer builds one glamour renderer, reused for every chunk of a style.
func newRenderer(style gansi.StyleConfig, width int) *glamour.TermRenderer {
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
		// True-color chroma so code-block backgrounds are emitted as exact
		// RGB, matching the badge rows and the sidebar surface.
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		return nil
	}
	return r
}

// appendGlamour renders one Markdown chunk and appends its lines.
func (o *Output) appendGlamour(chunk string, r *glamour.TermRenderer) {
	if r == nil {
		o.appendPlain(chunk)
		return
	}
	rendered, err := r.Render(chunk)
	if err != nil {
		o.appendPlain(chunk)
		return
	}
	o.appendTrimmed(rendered)
}

// trimEmptyLines drops empty leading and trailing lines.
func trimEmptyLines(lines []string, empty func(string) bool) []string {
	for len(lines) > 0 && empty(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && empty(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// appendTrimmed splits rendered output into lines, dropping visually empty
// leading and trailing lines.
func (o *Output) appendTrimmed(s string) {
	o.Lines = append(o.Lines, trimEmptyLines(strings.Split(s, "\n"),
		func(l string) bool { return xansi.StringWidth(l) == 0 })...)
}

// appendPlain is the fallback when glamour cannot render a chunk: the raw
// Markdown source, trimmed.
func (o *Output) appendPlain(chunk string) {
	lines := trimEmptyLines(strings.Split(strings.TrimRight(chunk, "\n"), "\n"),
		func(l string) bool { return strings.TrimSpace(l) == "" })
	o.Lines = append(o.Lines, lines...)
}

// appendBadge adds the lightweight executable-block marker that ties the
// rendered view to the runbook semantics (SPEC.md §4.8 observation). It
// sits on the page background like ordinary prose; only the code block
// itself carries the executable surface color.
func (o *Output) appendBadge(b *doc.Block, color bool) {
	label := blockLabel(b)
	var line string
	if !color {
		line = "  ▸ " + label
	} else {
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		line = "  " + accent.Render("▸") + " " + muted.Render(label)
	}
	o.Lines = append(o.Lines, xansi.Truncate(line, max(1, o.Width), "…"))
}

// darkStyle derives the content style from glamour's dark theme: clean
// headings without inverted banners, muted chrome, and code blocks shaded
// with the panel color so they read as raised surfaces.
func darkStyle() gansi.StyleConfig {
	c := styles.DarkStyleConfig

	text := string(theme.Palette.Text)
	bright := string(theme.Palette.Bright)
	soft := string(theme.Palette.Soft)
	muted := string(theme.Palette.Muted)
	accent := string(theme.Palette.Accent)
	panel := string(theme.Palette.Panel)

	c.Document.StylePrimitive.Color = strPtr(text)

	// The h1 block is rebuilt wholesale here (dropping glamour's inherited
	// prefix/margin fields); the h2-h6 keep their inherited settings with
	// a cleared prefix.
	c.H1 = gansi.StyleBlock{
		StylePrimitive: gansi.StylePrimitive{
			Color: strPtr(bright),
			Bold:  boolPtr(true),
		},
	}
	c.Heading.StylePrimitive = gansi.StylePrimitive{
		Color: strPtr(text),
		Bold:  boolPtr(true),
	}
	for _, h := range []*gansi.StyleBlock{&c.H2, &c.H3, &c.H4, &c.H5, &c.H6} {
		h.Prefix = ""
	}
	c.H2.StylePrimitive.Color = strPtr(soft)
	c.H3.StylePrimitive.Color = strPtr(muted)
	c.H4.StylePrimitive.Color = strPtr(muted)
	c.H5.StylePrimitive.Color = strPtr(muted)
	c.H6.StylePrimitive.Color = strPtr(muted)

	c.Link.Color = strPtr(accent)
	c.LinkText.Color = strPtr(accent)
	c.BlockQuote.StylePrimitive.Color = strPtr(muted)
	c.HorizontalRule.Color = strPtr(string(theme.Palette.Rule))

	c.CodeBlock.StylePrimitive.BackgroundColor = strPtr(panel)
	c.Code.StylePrimitive.BackgroundColor = strPtr(panel)

	return c
}

// Executable blocks sit a soft step above the page: a muted blue-gray that
// reads as a raised surface against dark terminal backgrounds (including
// Ghostty's canvas) without competing with the prose.
const (
	execR, execG, execB = 30, 33, 48
)

var (
	execPanel       = fmt.Sprintf("#%02x%02x%02x", execR, execG, execB)
	execPanelEscape = fmt.Sprintf("\x1b[48;2;%d;%d;%dm", execR, execG, execB)
)

// execDarkStyle is darkStyle with the executable shade on the code block's
// primitive (the margin cells); the code text rows are painted by
// paintExecLines, since both chroma terminal formatters clear their
// Background style entry.
func execDarkStyle() gansi.StyleConfig {
	c := darkStyle()
	c.CodeBlock.StylePrimitive.BackgroundColor = strPtr(execPanel)
	return c
}

// paintExecLines applies the executable-block background to the lines
// glamour rendered for a code chunk. Chroma emits a reset after each token,
// so the background must be re-emitted after every reset; lines are padded
// to the chunk width so the shaded surface spans the whole content pane.
func paintExecLines(lines []string, width int) {
	for i, line := range lines {
		// Leave the first column on the page background: the shaded
		// surface starts one column in. Consume one of the code block's
		// indenting spaces so the code text itself does not shift.
		body := xansi.Cut(line, 1, xansi.StringWidth(line))
		body = xansi.Truncate(body, width-1, "")
		body = strings.ReplaceAll(body, "\x1b[0m", "\x1b[0m"+execPanelEscape)
		if visible := xansi.StringWidth(body); visible < width-1 {
			body += execPanelEscape + strings.Repeat(" ", width-1-visible)
		}
		lines[i] = " " + execPanelEscape + body + "\x1b[0m"
	}
}

// ExecColor returns the executable block surface color, for UI elements
// attached to a block (such as its inline output area).
func ExecColor() string { return execPanel }

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

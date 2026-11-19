package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/md-exec/internal/doc"
)

// appendGlamour renders one Markdown chunk and appends its lines.
func (o *Output) appendGlamour(chunk string, width int, style gansi.StyleConfig) {
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
		// True-color chroma so code-block backgrounds are emitted as exact
		// RGB, matching the badge rows and the sidebar surface.
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
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

// appendTrimmed splits rendered output into lines, dropping visually empty
// leading and trailing lines.
func (o *Output) appendTrimmed(s string) {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && ansi.StringWidth(lines[0]) == 0 {
		lines = lines[1:]
	}
	for len(lines) > 0 && ansi.StringWidth(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	o.Lines = append(o.Lines, lines...)
}

// appendPlain is the fallback when glamour cannot render a chunk: the raw
// Markdown source, trimmed.
func (o *Output) appendPlain(chunk string) {
	lines := strings.Split(strings.TrimRight(chunk, "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	o.Lines = append(o.Lines, lines...)
}

// appendBadge adds the lightweight executable-block marker that ties the
// rendered view to the runbook semantics (SPEC.md §4.8 observation). It
// sits on the page background like ordinary prose; only the code block
// itself carries the executable surface color.
func (o *Output) appendBadge(b *doc.Block, color bool) {
	label := fmt.Sprintf("%s — line %d · %s", b.Lang, b.Line, blockSource(b))
	var line string
	if !color {
		line = "  ▸ " + label
	} else {
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		line = "  " + accent.Render("▸") + " " + muted.Render(label)
	}
	o.Lines = append(o.Lines, ansi.Truncate(line, max(1, o.Width), "…"))
}

// darkStyle derives the content style from glamour's dark theme: clean
// headings without inverted banners, muted chrome, and code blocks shaded
// with the panel color so they read as raised surfaces.
func darkStyle() gansi.StyleConfig {
	c := styles.DarkStyleConfig

	text := "#e4e4e9"
	bright := "#f4f4f8"
	soft := "#c8c8d0"
	muted := "#8a8a95"
	accent := "#8f8af4"
	panel := "#1a1a21"

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
	c.HorizontalRule.Color = strPtr("#3a3a44")

	c.CodeBlock.StylePrimitive.BackgroundColor = strPtr(panel)
	c.Code.StylePrimitive.BackgroundColor = strPtr(panel)

	return c
}

// execDarkStyle leaves executable code blocks on the page background: the
// most subtle treatment, so the block is marked by its badge, indentation,
// spacing rows and selection rail rather than by a competing shade.
func execDarkStyle() gansi.StyleConfig {
	c := darkStyle()
	c.CodeBlock.StylePrimitive.BackgroundColor = nil
	return c
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func uintPtr(u uint) *uint    { return &u }

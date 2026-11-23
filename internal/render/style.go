package render

import (
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
		// True-color chroma so code-block colors are emitted as exact RGB,
		// matching the theme tokens used by the chrome.
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		return nil
	}
	return r
}

// appendGlamour renders one Markdown chunk and appends its lines. On
// ordinary chunks, canvas is re-emitted after every reset so a styled span
// such as inline code cannot leave its background on the following text.
// Executable chunks pass an empty canvas: paintExecLines handles them.
func (o *Output) appendGlamour(chunk string, r *glamour.TermRenderer, canvas string) {
	if r == nil {
		o.appendPlain(chunk)
		return
	}
	rendered, err := r.Render(chunk)
	if err != nil {
		o.appendPlain(chunk)
		return
	}
	if canvas != "" {
		rendered = strings.ReplaceAll(rendered, "\x1b[0m", "\x1b[0m"+canvas)
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
func (o *Output) appendBadge(b *doc.Block, th theme.Theme) {
	label := blockLabel(b)
	var line string
	if th.Name == theme.Notty.Name {
		line = "  ▸ " + label
	} else {
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Accent))
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color(th.TextMuted))
		line = "  " + accent.Render("▸") + " " + muted.Render(label)
	}
	o.Lines = append(o.Lines, xansi.Truncate(line, max(1, o.Width), "…"))
}

// styleFor derives the content style from the active theme. Glamour's dark
// config supplies only the structure (margins, prefixes, emphasis flags);
// every color — including the chroma token palette — comes from the theme.
func styleFor(th theme.Theme) gansi.StyleConfig {
	c := styles.DarkStyleConfig

	c.Document.StylePrimitive.Color = strPtr(string(th.Text))
	c.BlockQuote.StylePrimitive.Color = strPtr(string(th.TextMuted))

	// The h1 block is rebuilt wholesale here (dropping glamour's inherited
	// prefix/margin fields); the h2-h6 keep their inherited settings with
	// a cleared prefix.
	c.H1 = gansi.StyleBlock{
		StylePrimitive: gansi.StylePrimitive{
			Color: strPtr(string(th.Bright)),
			Bold:  boolPtr(true),
		},
	}
	c.Heading.StylePrimitive = gansi.StylePrimitive{
		Color: strPtr(string(th.Text)),
		Bold:  boolPtr(true),
	}
	for _, h := range []*gansi.StyleBlock{&c.H2, &c.H3, &c.H4, &c.H5, &c.H6} {
		h.Prefix = ""
		h.StylePrimitive.Color = strPtr(string(th.Text))
	}
	c.H2.StylePrimitive.Color = strPtr(string(th.Soft))
	c.H3.StylePrimitive.Color = strPtr(string(th.TextMuted))
	c.H4.StylePrimitive.Color = strPtr(string(th.TextMuted))
	c.H5.StylePrimitive.Color = strPtr(string(th.TextMuted))
	c.H6.StylePrimitive.Color = strPtr(string(th.TextMuted))

	c.Link.Color = strPtr(string(th.Accent))
	c.LinkText.Color = strPtr(string(th.Accent))
	c.Image.Color = strPtr(string(th.Accent))
	c.ImageText.Color = strPtr(string(th.TextMuted))
	c.HorizontalRule.Color = strPtr(string(th.Rule))

	codeText := strPtr(string(theme.AdjustForSurface(th.Text, th.ExecPanel, 4.5)))
	c.Code.StylePrimitive.Color = codeText
	c.Code.StylePrimitive.BackgroundColor = strPtr(string(th.CodePanel))
	c.CodeBlock.StylePrimitive.Color = codeText
	c.CodeBlock.StylePrimitive.BackgroundColor = strPtr(string(th.CodePanel))
	// Use a per-theme named style: glamour's built-in "charm" registry entry
	// is written once and would otherwise pin every later render to the
	// first theme used in this process.
	c.CodeBlock.Chroma = nil
	c.CodeBlock.Theme = registerChromaStyle(th, chromaFor(th))

	return c
}

// chromaFor builds the syntax-token palette from the theme. A theme with its
// own Syntax keeps its code colors; otherwise the legacy role-derived mapping
// is used so the original dark palette is unchanged. Nothing is left to
// glamour's built-in palettes.
func chromaFor(th theme.Theme) *gansi.Chroma {
	if th.Syntax == (theme.Syntax{}) {
		return legacyChroma(th)
	}
	s := theme.AdjustSyntax(th)
	adj := func(c theme.Color) theme.Color {
		return theme.AdjustForSurface(c, th.ExecPanel, 4.5)
	}
	text := strPtr(string(adj(th.Text)))
	bright := strPtr(string(adj(th.Bright)))
	soft := strPtr(string(adj(th.Soft)))
	muted := strPtr(string(adj(th.TextMuted)))
	success := strPtr(string(adj(th.Success)))
	err := strPtr(string(adj(th.Error)))
	accentC, warningC, successC, softC, mutedC := adj(th.Accent), adj(th.Warning), adj(th.Success), adj(th.Soft), adj(th.TextMuted)

	comment := colorOr(s.Comment, mutedC)
	keyword := colorOr(s.Keyword, accentC)
	str := colorOr(s.String, successC)
	fn := colorOr(s.Function, successC)
	number := colorOr(s.Number, accentC)
	typ := colorOr(s.Type, softC)
	op := colorOr(s.Operator, warningC)
	punct := colorOr(s.Punctuation, softC)
	constant := colorOr(s.Constant, warningC)

	return &gansi.Chroma{
		Text:                gansi.StylePrimitive{Color: text},
		Error:               gansi.StylePrimitive{Color: err},
		Comment:             gansi.StylePrimitive{Color: colorPtr(comment)},
		CommentPreproc:      gansi.StylePrimitive{Color: colorPtr(constant)},
		Keyword:             gansi.StylePrimitive{Color: colorPtr(keyword)},
		KeywordReserved:     gansi.StylePrimitive{Color: colorPtr(keyword)},
		KeywordNamespace:    gansi.StylePrimitive{Color: colorPtr(keyword)},
		KeywordType:         gansi.StylePrimitive{Color: colorPtr(typ)},
		Operator:            gansi.StylePrimitive{Color: colorPtr(op)},
		Punctuation:         gansi.StylePrimitive{Color: colorPtr(punct)},
		Name:                gansi.StylePrimitive{Color: text},
		NameBuiltin:         gansi.StylePrimitive{Color: colorPtr(typ)},
		NameTag:             gansi.StylePrimitive{Color: colorPtr(typ)},
		NameAttribute:       gansi.StylePrimitive{Color: colorPtr(constant)},
		NameClass:           gansi.StylePrimitive{Color: colorPtr(typ)},
		NameConstant:        gansi.StylePrimitive{Color: colorPtr(constant)},
		NameDecorator:       gansi.StylePrimitive{Color: colorPtr(constant)},
		NameException:       gansi.StylePrimitive{Color: err},
		NameFunction:        gansi.StylePrimitive{Color: colorPtr(fn)},
		NameOther:           gansi.StylePrimitive{Color: text},
		Literal:             gansi.StylePrimitive{Color: colorPtr(str)},
		LiteralNumber:       gansi.StylePrimitive{Color: colorPtr(number)},
		LiteralDate:         gansi.StylePrimitive{Color: colorPtr(number)},
		LiteralString:       gansi.StylePrimitive{Color: colorPtr(str)},
		LiteralStringEscape: gansi.StylePrimitive{Color: colorPtr(str)},
		GenericDeleted:      gansi.StylePrimitive{Color: err},
		GenericEmph:         gansi.StylePrimitive{Color: soft},
		GenericInserted:     gansi.StylePrimitive{Color: success},
		GenericStrong:       gansi.StylePrimitive{Color: bright},
		GenericSubheading:   gansi.StylePrimitive{Color: muted},
		// The surface is painted by paintExecLines / PaintCanvas; leaving
		// chroma's background unset keeps it from painting over that work.
		Background: gansi.StylePrimitive{},
	}
}

// legacyChroma is the original role-derived mapping, kept for themes without
// a dedicated syntax palette (dark and notty).
func legacyChroma(th theme.Theme) *gansi.Chroma {
	adj := func(c theme.Color) string {
		return string(theme.AdjustForSurface(c, th.ExecPanel, 4.5))
	}
	text := strPtr(adj(th.Text))
	bright := strPtr(adj(th.Bright))
	soft := strPtr(adj(th.Soft))
	muted := strPtr(adj(th.TextMuted))
	accent := strPtr(adj(th.Accent))
	success := strPtr(adj(th.Success))
	warning := strPtr(adj(th.Warning))
	err := strPtr(adj(th.Error))

	return &gansi.Chroma{
		Text:                gansi.StylePrimitive{Color: text},
		Error:               gansi.StylePrimitive{Color: err},
		Comment:             gansi.StylePrimitive{Color: muted},
		CommentPreproc:      gansi.StylePrimitive{Color: warning},
		Keyword:             gansi.StylePrimitive{Color: accent},
		KeywordReserved:     gansi.StylePrimitive{Color: accent},
		KeywordNamespace:    gansi.StylePrimitive{Color: bright},
		KeywordType:         gansi.StylePrimitive{Color: soft},
		Operator:            gansi.StylePrimitive{Color: warning},
		Punctuation:         gansi.StylePrimitive{Color: soft},
		Name:                gansi.StylePrimitive{Color: text},
		NameBuiltin:         gansi.StylePrimitive{Color: success},
		NameTag:             gansi.StylePrimitive{Color: accent},
		NameAttribute:       gansi.StylePrimitive{Color: soft},
		NameClass:           gansi.StylePrimitive{Color: bright},
		NameConstant:        gansi.StylePrimitive{Color: warning},
		NameDecorator:       gansi.StylePrimitive{Color: warning},
		NameException:       gansi.StylePrimitive{Color: err},
		NameFunction:        gansi.StylePrimitive{Color: success},
		NameOther:           gansi.StylePrimitive{Color: text},
		Literal:             gansi.StylePrimitive{Color: success},
		LiteralNumber:       gansi.StylePrimitive{Color: accent},
		LiteralDate:         gansi.StylePrimitive{Color: warning},
		LiteralString:       gansi.StylePrimitive{Color: success},
		LiteralStringEscape: gansi.StylePrimitive{Color: bright},
		GenericDeleted:      gansi.StylePrimitive{Color: err},
		GenericEmph:         gansi.StylePrimitive{Color: soft},
		GenericInserted:     gansi.StylePrimitive{Color: success},
		GenericStrong:       gansi.StylePrimitive{Color: bright},
		GenericSubheading:   gansi.StylePrimitive{Color: muted},
		Background:          gansi.StylePrimitive{},
	}
}

// colorOr picks the first non-empty color.
func colorOr(preferred, fallback theme.Color) theme.Color {
	if preferred != "" {
		return preferred
	}
	return fallback
}

func colorPtr(c theme.Color) *string { return strPtr(string(c)) }

// execStyleFor is styleFor with the executable shade on the code block's
// primitive (the margin cells); the code text rows are painted by
// paintExecLines, since the chroma formatter clears their background.
func execStyleFor(th theme.Theme) gansi.StyleConfig {
	c := styleFor(th)
	c.CodeBlock.StylePrimitive.BackgroundColor = strPtr(string(th.ExecPanel))
	return c
}

// paintExecLines applies the executable-block surface to the lines glamour
// rendered for a code chunk. Chroma emits a reset after each token, so the
// surface must be re-emitted after every reset.
func paintExecLines(lines []string, width int, canvas, exec string) {
	for i, line := range lines {
		// Leave the first column on the page background: the surface
		// starts one column in. Consume one of the block's indenting
		// spaces so the code text itself does not shift.
		body := xansi.Cut(line, 1, xansi.StringWidth(line))
		body = xansi.Truncate(body, width-1, "")
		body = strings.ReplaceAll(body, "\x1b[0m", "\x1b[0m"+exec)
		if visible := xansi.StringWidth(body); visible < width-1 {
			body += exec + strings.Repeat(" ", width-1-visible)
		}
		lines[i] = canvas + " " + exec + body + "\x1b[0m"
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

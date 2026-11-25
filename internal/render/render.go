// Package render turns a parsed document into styled terminal lines.
//
// It is a pure function of (document, width, theme): the TUI re-renders on
// resize and the `exmd print` verb shares the same pipeline. Content is
// rendered with glamour, chunk by chunk, so the output carries the rendered
// line index of every heading (for outline navigation and scroll-spy) and
// every executable block (for badge placement and jumps).
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/doc"
	"github.com/michaelishri/exmd/internal/theme"
)

// HeadingMark is the rendered line at which a heading starts. Index i
// corresponds to Document.Headings[i].
type HeadingMark struct {
	Line   int    // 0-based rendered line index
	Number string // generated heading number ("1.1."), "" when none
}

// BlockMark is the rendered line at which an executable block's badge sits.
// Index i corresponds to Document.Blocks[i].
type BlockMark struct {
	Line     int // 0-based rendered line index
	End      int // exclusive rendered line after the block's last line
	Label    string
	BlockRef *doc.Block
}

// Output is the rendered document.
type Output struct {
	Lines    []string // ANSI-styled, wrapped to width
	Headings []HeadingMark
	Blocks   []BlockMark
	Width    int
}

type boundary struct {
	line    int // 1-based source line where the chunk starts
	heading int // index into d.Headings, -1 otherwise
	block   int // index into d.Blocks, -1 otherwise
}

// Render renders d at the given width with the active theme. width <= 0
// falls back to DefaultWidth. The notty theme yields unstyled plain text.
func Render(d *doc.Document, width int, th theme.Theme) *Output {
	if width <= 0 {
		width = DefaultWidth
	}
	color := th.Name != theme.Notty.Name
	out := &Output{Width: width}
	numbers := headingNumbers(d)

	bounds := boundaries(d)
	srcLines := strings.Split(string(d.Source), "\n")

	var style, execStyle ansi.StyleConfig
	if color {
		style = styleFor(th)
		execStyle = execStyleFor(th)
	} else {
		style = styles.ASCIIStyleConfig
	}
	canvas := backgroundEscape(th.Background)
	exec := backgroundEscape(th.ExecPanel)

	// Build the renderers once: glamour's setup is expensive and the same
	// two styles are used for every chunk in the document.
	renderer := newRenderer(style, width)
	execRenderer := renderer
	if color {
		execRenderer = newRenderer(execStyle, width)
	}

	for i, b := range bounds {
		end := d.BodyLine + len(srcLines) // default: to EOF
		if i+1 < len(bounds) {
			end = bounds[i+1].line
		}
		if b.line > len(srcLines) {
			continue
		}
		chunk := joinLines(srcLines, b.line, end)
		if chunk == "" {
			continue
		}
		// One blank line between chunks, unless the previous chunk already
		// ends with a visually blank line (glamour pads heading blocks and
		// the executable surface now ends with a shaded blank row).
		if len(out.Lines) > 0 {
			if last := xansi.Strip(out.Lines[len(out.Lines)-1]); strings.TrimSpace(last) != "" {
				out.Lines = append(out.Lines, "")
			}
		}

		switch {
		case b.heading >= 0:
			number := numbers[b.heading]
			out.Headings = append(out.Headings, HeadingMark{
				Line:   len(out.Lines),
				Number: number,
			})
			out.appendGlamour(prefixHeadingNumber(chunk, number), renderer, canvas)
		case b.block >= 0:
			blk := d.Blocks[b.block]
			out.Blocks = append(out.Blocks, BlockMark{
				Line:     len(out.Lines),
				Label:    blockLabel(blk),
				BlockRef: blk,
			})
			mark := &out.Blocks[len(out.Blocks)-1]
			out.appendBadge(blk, th)
			// Strip the :exec suffix so chroma picks the right lexer; this
			// only affects rendering, never execution (SPEC.md §9).
			chunk = stripExecSuffix(chunk)
			start := len(out.Lines)
			out.appendGlamour(chunk, execRenderer, "")
			if color {
				paintExecLines(out.Lines[start:], width, canvas, exec)
				// Glamour's code block has a blank row above its first code
				// line; add a matching shaded blank row below the last one.
				out.Lines = append(out.Lines,
					canvas+" "+exec+strings.Repeat(" ", width-1)+"\x1b[0m")
			}
			// The surface ends at the shaded blank row; the plain spacing
			// row below it belongs to the page, not to the block.
			mark.End = len(out.Lines)
			if color {
				out.Lines = append(out.Lines, "")
			}
		default:
			out.appendGlamour(chunk, renderer, canvas)
		}
	}

	return out
}

// blockLabel is the single source of truth for an executable block's badge
// text and its render mark label.
func blockLabel(b *doc.Block) string {
	return fmt.Sprintf("%s — line %d · %s", b.Lang, b.Line, blockSource(b))
}

// boundaries returns the chunk start lines: the first body line, then every
// heading and every executable-block fence, in document order. Each heading
// chunk ends right after the heading line — a synthetic boundary starts the
// section's content as its own chunk, which the separator blanks before, so
// every heading is followed by an empty line in the rendered output. A
// boundary after each block's closing fence keeps following prose out of the
// executable chunk.
func boundaries(d *doc.Document) []boundary {
	var bs []boundary
	used := map[int]bool{}
	add := func(b boundary) {
		bs = append(bs, b)
		used[b.line] = true
	}
	add(boundary{line: d.BodyLine, heading: -1, block: -1})
	for i, h := range d.Headings {
		if h.Line > 0 {
			add(boundary{line: h.Line, heading: i, block: -1})
			add(boundary{line: h.Line + 1, heading: -1, block: -1})
		}
	}
	for i, b := range d.Blocks {
		add(boundary{line: b.Line, heading: -1, block: i})
	}
	// Without a boundary after the closing fence, prose following an
	// executable block joins its chunk and is painted with the executable
	// surface. Skip the boundary when a heading or another block already
	// starts on that line: it cuts the chunk at the same place and must
	// keep its own (higher priority) role.
	for _, b := range d.Blocks {
		line := b.EndLine + 1
		if !used[line] {
			add(boundary{line: line, heading: -1, block: -1})
		}
	}
	sort.SliceStable(bs, func(i, j int) bool { return bs[i].line < bs[j].line })
	return bs
}

// joinLines joins 1-based source lines [start, end): `end` is the next
// boundary's line number, so the last included line is end-1.
func joinLines(srcLines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	last := end - 1
	if last > len(srcLines) {
		last = len(srcLines)
	}
	if start > last {
		return ""
	}
	var sb strings.Builder
	for i := start - 1; i < last; i++ {
		sb.WriteString(srcLines[i])
		sb.WriteByte('\n')
	}
	return dedent(sb.String())
}

// dedent removes a common leading-indentation prefix so that code blocks
// nested in list items render as top-level fences.
func dedent(chunk string) string {
	lines := strings.Split(chunk, "\n")
	minIndent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := 0
		for n < len(l) && l[n] == ' ' {
			n++
		}
		if minIndent < 0 || n < minIndent {
			minIndent = n
		}
	}
	if minIndent <= 0 {
		return chunk
	}
	for i, l := range lines {
		if len(l) >= minIndent {
			lines[i] = l[minIndent:]
		}
	}
	return strings.Join(lines, "\n")
}

// stripExecSuffix rewrites the first fence's `lang:exec` info string to
// `lang` so glamour's chroma lexer resolves.
func stripExecSuffix(chunk string) string {
	lines := strings.SplitN(chunk, "\n", 2)
	lines[0] = strings.Replace(lines[0], ":exec", "", 1)
	return strings.Join(lines, "\n")
}

func blockSource(b *doc.Block) string {
	if b.Kind == doc.KindScript {
		return b.ScriptPath
	}
	return "embedded"
}

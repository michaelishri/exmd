// Package render turns a parsed document into styled terminal lines.
//
// It is a pure function of (document, width, color): the TUI re-renders on
// resize and the `md-exec print` verb shares the same pipeline. Content is
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

	"github.com/michaelishri/md-exec/internal/doc"
)

// HeadingMark is the rendered line at which a heading starts. Index i
// corresponds to Document.Headings[i].
type HeadingMark struct {
	Line  int // 0-based rendered line index
	Level int
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
	Color    bool
}

type boundary struct {
	line    int // 1-based source line where the chunk starts
	heading int // index into d.Headings, -1 otherwise
	block   int // index into d.Blocks, -1 otherwise
}

// Render renders d at the given width. width <= 0 falls back to 80.
func Render(d *doc.Document, width int, color bool) (*Output, error) {
	if width <= 0 {
		width = 80
	}
	out := &Output{Width: width, Color: color}

	bounds := boundaries(d)
	srcLines := strings.Split(string(d.Source), "\n")

	var style, execStyle ansi.StyleConfig
	if color {
		style = darkStyle()
		execStyle = execDarkStyle()
	} else {
		style = styles.ASCIIStyleConfig
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
			out.Headings = append(out.Headings, HeadingMark{Line: len(out.Lines), Level: d.Headings[b.heading].Level})
			out.appendGlamour(chunk, width, style)
		case b.block >= 0:
			blk := d.Blocks[b.block]
			out.Blocks = append(out.Blocks, BlockMark{
				Line:     len(out.Lines),
				Label:    fmt.Sprintf("%s — line %d · %s", blk.Lang, blk.Line, blockSource(blk)),
				BlockRef: blk,
			})
			out.appendBadge(blk, color)
			// Strip the :exec suffix so chroma picks the right lexer; this
			// only affects rendering, never execution (SPEC.md §9).
			chunk = stripExecSuffix(chunk)
			start := len(out.Lines)
			out.appendGlamour(chunk, width, execStyle)
			if color {
				paintExecLines(out.Lines[start:], width)
				// Glamour's code block has a blank row above its first code
				// line; add a matching shaded blank row below the last one.
				out.Lines = append(out.Lines,
					" "+execPanelEscape+strings.Repeat(" ", width-1)+"\x1b[0m")
				// And a plain spacing row below the whole box.
				out.Lines = append(out.Lines, "")
			}
		default:
			out.appendGlamour(chunk, width, style)
		}
	}

	// Each block's rendered extent ends where the next chunk starts: the
	// next heading or block mark, minus the separator blank that precedes
	// it. The final block also excludes the plain spacing row appended
	// after its shaded surface, so the extent never covers that row.
	marks := make([]int, 0, len(out.Headings)+len(out.Blocks))
	for _, hm := range out.Headings {
		marks = append(marks, hm.Line)
	}
	for i := range out.Blocks {
		marks = append(marks, out.Blocks[i].Line)
	}
	sort.Ints(marks)
	for i := range out.Blocks {
		end := len(out.Lines)
		if color {
			end-- // trailing spacing row of the final exec block
		}
		for _, m := range marks {
			if m > out.Blocks[i].Line {
				end = m - 1
				break
			}
		}
		out.Blocks[i].End = end
	}
	return out, nil
}

// boundaries returns the chunk start lines: the first body line, then every
// heading and every executable-block fence, in document order. Each heading
// chunk ends right after the heading line — a synthetic boundary starts the
// section's content as its own chunk, which the separator blanks before, so
// every heading is followed by an empty line in the rendered output.
func boundaries(d *doc.Document) []boundary {
	var bs []boundary
	bs = append(bs, boundary{line: d.BodyLine, heading: -1, block: -1})
	for i, h := range d.Headings {
		if h.Line > 0 {
			bs = append(bs, boundary{line: h.Line, heading: i, block: -1})
			bs = append(bs, boundary{line: h.Line + 1, heading: -1, block: -1})
		}
	}
	for i, b := range d.Blocks {
		bs = append(bs, boundary{line: b.Line, heading: -1, block: i})
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

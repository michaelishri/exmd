package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/michaelishri/exmd/internal/termsafe"
)

const (
	// maxBlockLines caps how many output lines one block's inline box
	// keeps. The most recent lines win: errors usually come last.
	maxBlockLines = 500

	// maxLineBytes caps one stored output line. Output without newlines
	// (minified JSON, a progress bar on \r) would otherwise grow without
	// limit; the rest of such a line is dropped and marked with "…".
	maxLineBytes = 16 << 10

	// tabWidth is the tab stop used to lay out output lines.
	tabWidth = 8
)

// write accumulates an output chunk into complete lines. Memory stays
// bounded however much a block prints: at most maxBlockLines lines of at
// most maxLineBytes each, plus one partial line of the same size.
func (st *inlineState) write(chunk []byte) {
	data := string(chunk)
	for data != "" {
		part, rest, complete := strings.Cut(data, "\n")
		if !st.overflow {
			if room := maxLineBytes - len(st.pending); len(part) > room {
				st.pending += part[:room]
				st.overflow = true
			} else {
				st.pending += part
			}
		}
		if !complete {
			break
		}
		st.addLine()
		data = rest
	}
	st.urlsValid = false
}

// flush stores the trailing partial line, if any, once a run ends.
func (st *inlineState) flush() {
	if st.pending != "" || st.overflow {
		st.addLine()
	}
}

// addLine moves the pending line into the stored lines, discarding the
// oldest line (and its rendered row) beyond maxBlockLines.
func (st *inlineState) addLine() {
	line := cleanOutputLine(st.pending)
	if st.overflow {
		line += "…"
	}
	st.pending, st.overflow = "", false
	st.lines = append(st.lines, line)
	if over := len(st.lines) - maxBlockLines; over > 0 {
		st.lines = st.lines[over:]
		st.dropped += over
		if len(st.rendered) >= over {
			st.rendered = st.rendered[over:]
		} else {
			st.rendered = nil
		}
	}
	st.urlsValid = false
}

// cleanOutputLine turns one raw output line into text that is safe and
// sensible to draw. A carriage return rewinds to the start of the line, so
// only the text after the last one is kept (progress bars), and escape
// sequences and other control characters are dropped.
func cleanOutputLine(s string) string {
	if strings.IndexByte(s, '\r') >= 0 {
		segs := strings.Split(s, "\r")
		s = ""
		for i := len(segs) - 1; i >= 0; i-- {
			if segs[i] != "" {
				s = segs[i]
				break
			}
		}
	}
	return termsafe.Strip(s)
}

// copiedText returns the block's output as plain text: every stored line,
// plus the trailing partial line still accumulating during a run. Copying
// uses these stored lines, not the rendered box, so the text is neither
// truncated to the pane width nor decorated with the output box's chrome.
func (st *inlineState) copiedText() string {
	text := strings.Join(st.lines, "\n")
	if st.pending != "" {
		if text != "" {
			text += "\n"
		}
		text += cleanOutputLine(st.pending)
	}
	return text
}

// outputURLs returns the URLs in the block's output, cached until the
// output changes: the footer asks on every frame.
func (st *inlineState) outputURLs() []string {
	if !st.urlsValid {
		st.urls = outputURLs(st.copiedText())
		st.urlsValid = true
	}
	return st.urls
}

// fitCells lays out a line in exactly width terminal cells: tabs expand to
// the next tab stop, the text is cut by display width (never inside a
// character, and wide characters count as two cells), and the rest is
// padded with spaces.
func fitCells(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if strings.IndexByte(line, '\t') >= 0 {
		line = expandTabs(line, width)
	}
	line = ansi.Truncate(line, width, "")
	if pad := width - ansi.StringWidth(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return line
}

// expandTabs replaces tabs with spaces up to the next tab stop. It stops
// once the text is wider than limit: the rest would be cut anyway.
func expandTabs(line string, limit int) string {
	var sb strings.Builder
	col := 0
	for {
		seg, rest, found := strings.Cut(line, "\t")
		sb.WriteString(seg)
		col += ansi.StringWidth(seg)
		if !found || col > limit {
			return sb.String()
		}
		n := tabWidth - col%tabWidth
		sb.WriteString(strings.Repeat(" ", n))
		col += n
		line = rest
	}
}

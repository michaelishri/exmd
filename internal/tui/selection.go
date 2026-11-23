package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// selPoint is a position in the rendered document: an absolute line index in
// contentLines() and a display column within that line. Storing absolute
// lines means scrolling does not disturb a selection.
type selPoint struct {
	line int
	col  int
}

// textSelection is a mouse selection in the content pane. anchor is where the
// drag began and head follows the pointer.
type textSelection struct {
	anchor, head selPoint
	set          bool
	dragging     bool
}

func (s *textSelection) clear() { *s = textSelection{} }

// ordered returns the endpoints in document order and whether at least one
// cell is selected.
func (s textSelection) ordered() (selPoint, selPoint, bool) {
	if !s.set {
		return selPoint{}, selPoint{}, false
	}
	a, b := s.anchor, s.head
	if a.line > b.line || (a.line == b.line && a.col > b.col) {
		a, b = b, a
	}
	return a, b, a != b
}

// contentOrigin returns the terminal cell where the content viewport starts.
// Bubble Tea reports mouse coordinates zero-based; the wide layout adds a
// rail column and a pane padding row, while the narrow layout renders the
// bare viewport.
func (m Model) contentOrigin() (int, int) {
	switch {
	case m.width < minViewWidth || m.height < minViewHeight:
		return 0, 0
	case !m.outline.visible:
		return 0, 1
	default:
		return railWidth, 1
	}
}

// mousePoint maps a mouse event to a point in the rendered document. Events
// outside the content pane (rail, gutter, sidebar, footer) are rejected.
func (m Model) mousePoint(msg tea.MouseMsg) (selPoint, bool) {
	if m.out == nil {
		return selPoint{}, false
	}
	ox, oy := m.contentOrigin()
	col, row := msg.X-ox, msg.Y-oy
	if col < 0 || col >= m.content.Width || row < 0 || row >= m.content.Height {
		return selPoint{}, false
	}
	lines := m.rawContentLines()
	line := clamp(m.content.YOffset+row, 0, max(0, len(lines)-1))
	col = clamp(col, 0, ansi.StringWidth(ansi.Strip(lines[line])))
	return selPoint{line: line, col: col}, true
}

// selectionText extracts the selected span as plain text. Display padding is
// trimmed from each line, but the text is otherwise what the reader sees.
func selectionText(lines []string, from, to selPoint) string {
	if from.line < 0 || to.line >= len(lines) || from.line > to.line {
		return ""
	}
	parts := make([]string, 0, to.line-from.line+1)
	for line := from.line; line <= to.line; line++ {
		plain := ansi.Strip(lines[line])
		lo, hi := 0, ansi.StringWidth(plain)
		if line == from.line {
			lo = from.col
		}
		if line == to.line {
			hi = to.col
		}
		parts = append(parts, strings.TrimRight(ansi.Cut(plain, lo, hi), " \t"))
	}
	return strings.Join(parts, "\n")
}

// Inverse video marks the selection without resetting the syntax colors that
// precede or follow it: SGR 27 turns only the inverse attribute back off.
const (
	selectionOn  = "\x1b[7m"
	selectionOff = "\x1b[27m"
)

// highlightSelection shades [from, to) display columns of one rendered line.
func highlightSelection(line string, from, to int) string {
	if to <= from {
		return line
	}
	width := ansi.StringWidth(line)
	return ansi.Cut(line, 0, from) + selectionOn +
		ansi.Strip(ansi.Cut(line, from, to)) + selectionOff +
		ansi.Cut(line, to, width)
}

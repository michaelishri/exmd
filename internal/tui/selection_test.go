package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestContentOrigin(t *testing.T) {
	for _, tt := range []struct {
		name         string
		w, h         int
		wantX, wantY int
	}{
		{"wide with sidebar", 120, 40, railWidth, 2},
		{"sidebar hidden", 80, 30, 0, 1},
		{"narrow", 15, 10, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newViewer(t, tt.w, tt.h)
			x, y := m.contentOrigin()
			if x != tt.wantX || y != tt.wantY {
				t.Fatalf("origin = (%d,%d), want (%d,%d)", x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestMousePointMapsViewportCoordinates(t *testing.T) {
	m := newViewer(t, 120, 40)
	m.content.SetYOffset(3)
	ox, oy := m.contentOrigin()

	p, ok := m.mousePoint(tea.MouseMsg{
		X: ox + 2, Y: oy + 4,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if !ok {
		t.Fatal("click inside the viewport was rejected")
	}
	if p.line != 7 || p.col != 2 {
		t.Fatalf("point = %+v, want line 7 col 2 (viewport row + YOffset)", p)
	}
	for _, msg := range []tea.MouseMsg{
		{X: ox - 1, Y: oy},
		{X: ox + m.content.Width, Y: oy},
		{X: ox, Y: oy - 1},
		{X: ox, Y: oy + m.content.Height},
	} {
		if _, ok := m.mousePoint(msg); ok {
			t.Fatalf("click outside content accepted: %+v", msg)
		}
	}
}

func TestSelectionOrdered(t *testing.T) {
	s := textSelection{anchor: selPoint{2, 3}, head: selPoint{1, 5}, set: true}
	from, to, ok := s.ordered()
	if !ok || from != (selPoint{1, 5}) || to != (selPoint{2, 3}) {
		t.Fatalf("ordered() = %+v, %+v, %v", from, to, ok)
	}
	empty := textSelection{anchor: selPoint{1, 1}, head: selPoint{1, 1}, set: true}
	if _, _, ok := empty.ordered(); ok {
		t.Fatal("zero-width selection reported as a range")
	}
	if _, _, ok := (textSelection{}).ordered(); ok {
		t.Fatal("unset selection reported as a range")
	}
}

func TestSelectionText(t *testing.T) {
	lines := []string{
		"\x1b[31mhello world\x1b[0m",
		"second line   ",
		"third",
	}
	for _, tt := range []struct {
		name     string
		from, to selPoint
		want     string
	}{
		{"single line", selPoint{0, 0}, selPoint{0, 5}, "hello"},
		{"across lines", selPoint{0, 6}, selPoint{1, 6}, "world\nsecond"},
		{"whole lines", selPoint{0, 0}, selPoint{2, 5}, "hello world\nsecond line\nthird"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectionText(lines, tt.from, tt.to); got != tt.want {
				t.Fatalf("selectionText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// pointForText returns the position of the first occurrence of text in the
// rendered document (before any selection styling).
func pointForText(t *testing.T, m Model, text string) selPoint {
	t.Helper()
	for i, l := range m.rawContentLines() {
		plain := ansi.Strip(l)
		if col := strings.Index(plain, text); col >= 0 {
			return selPoint{line: i, col: col}
		}
	}
	t.Fatalf("text %q not found in the rendered document", text)
	return selPoint{}
}

// updateModel feeds one message through the model and returns the result.
func updateModel(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

// mouseAt builds a mouse event for a document point, assuming the point is
// visible in the viewport.
func mouseAt(m Model, p selPoint, action tea.MouseAction, button tea.MouseButton) tea.MouseMsg {
	ox, oy := m.contentOrigin()
	return tea.MouseMsg{
		X: ox + p.col, Y: oy + p.line - m.content.YOffset,
		Action: action, Button: button,
	}
}

// beginDrag presses the left button at from and drags to to.
func beginDrag(m Model, from, to selPoint) Model {
	m = updateModel(m, mouseAt(m, from, tea.MouseActionPress, tea.MouseButtonLeft))
	m = updateModel(m, mouseAt(m, to, tea.MouseActionMotion, tea.MouseButtonLeft))
	return m
}

// TestDragCopiesOnRelease checks the whole gesture: the highlight appears
// during the drag, the selected span is copied on release, and the highlight
// is cleared afterwards.
func TestDragCopiesOnRelease(t *testing.T) {
	m := newViewer(t, 120, 40)
	start := pointForText(t, m, "exmd feature tour")
	end := selPoint{line: start.line, col: start.col + len("exmd")}
	m = beginDrag(m, start, end)

	if !strings.Contains(m.View(), selectionOn) {
		t.Fatal("selection is not highlighted during the drag")
	}
	seq := captureClipboard(t, func() {
		m = updateModel(m, mouseAt(m, end, tea.MouseActionRelease, tea.MouseButtonNone))
	})

	if text := clipPayload(t, seq); text != "exmd" {
		t.Fatalf("copied %q, want the selected text %q", text, "exmd")
	}
	if strings.Contains(m.View(), selectionOn) {
		t.Fatal("selection highlight remained after the copy")
	}
	if !strings.Contains(m.flash, "copied 1 line from selection") {
		t.Fatalf("footer reaction = %q, want a one-line copy confirmation", m.flash)
	}
}

// TestMultiLineReverseDragCopies checks a drag that ends above where it
// started: the text is copied in document order across several lines.
func TestMultiLineReverseDragCopies(t *testing.T) {
	m := newViewer(t, 120, 40)
	title := pointForText(t, m, "exmd feature tour")
	body := pointForText(t, m, "This runbook")
	start := selPoint{line: title.line, col: title.col + len("exmd")}
	end := selPoint{line: body.line, col: body.col + len("This run")}

	seq := captureClipboard(t, func() {
		m = beginDrag(m, end, start)
		m = updateModel(m, mouseAt(m, start, tea.MouseActionRelease, tea.MouseButtonNone))
	})
	text := clipPayload(t, seq)
	if !strings.Contains(text, "feature tour") || !strings.Contains(text, "\n") ||
		!strings.Contains(text, "This run") {
		t.Fatalf("copied text = %q, want the reverse-selected span in document order", text)
	}
	if !strings.Contains(m.flash, "lines from selection") {
		t.Fatalf("footer reaction = %q, want a multiline copy confirmation", m.flash)
	}
}

func TestPlainClickDoesNotCopy(t *testing.T) {
	m := newViewer(t, 120, 40)
	p := pointForText(t, m, "exmd feature tour")

	seq := captureClipboard(t, func() {
		m = updateModel(m, mouseAt(m, p, tea.MouseActionPress, tea.MouseButtonLeft))
		m = updateModel(m, mouseAt(m, p, tea.MouseActionRelease, tea.MouseButtonNone))
	})
	if seq != "" {
		t.Fatalf("plain click copied text: %q", seq)
	}
	if strings.Contains(m.View(), selectionOn) {
		t.Fatal("plain click left a highlight")
	}
}

func TestEscapeCancelsDrag(t *testing.T) {
	m := newViewer(t, 120, 40)
	start := pointForText(t, m, "exmd feature tour")
	end := selPoint{line: start.line, col: start.col + len("exmd")}
	m = beginDrag(m, start, end)

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.sel.set {
		t.Fatal("escape did not clear the selection")
	}
	if strings.Contains(m.View(), selectionOn) {
		t.Fatal("escape left a highlight")
	}
	seq := captureClipboard(t, func() {
		m = updateModel(m, mouseAt(m, end, tea.MouseActionRelease, tea.MouseButtonNone))
	})
	if seq != "" {
		t.Fatalf("release after escape copied text: %q", seq)
	}
}

func TestWheelKeepsSelection(t *testing.T) {
	m := newViewer(t, 120, 40)
	start := pointForText(t, m, "exmd feature tour")
	end := selPoint{line: start.line, col: start.col + len("exmd")}
	m = beginDrag(m, start, end)
	beforeFrom, beforeTo, _ := m.sel.ordered()

	before := m.content.YOffset
	m = updateModel(m, tea.MouseMsg{
		X: 5, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown,
	})
	if m.content.YOffset == before {
		t.Fatal("wheel event did not scroll")
	}
	afterFrom, afterTo, ok := m.sel.ordered()
	if !ok || afterFrom != beforeFrom || afterTo != beforeTo {
		t.Fatalf("selection moved on scroll: %+v..%+v, want %+v..%+v",
			afterFrom, afterTo, beforeFrom, beforeTo)
	}
}

func TestResizeClearsSelection(t *testing.T) {
	m := newViewer(t, 120, 40)
	start := pointForText(t, m, "exmd feature tour")
	end := selPoint{line: start.line, col: start.col + len("exmd")}
	m = beginDrag(m, start, end)

	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.sel.set {
		t.Fatal("selection survived a resize")
	}
	if strings.Contains(m.View(), selectionOn) {
		t.Fatal("highlight survived a resize")
	}
}

func TestClickOutsideContentClearsSelection(t *testing.T) {
	m := newViewer(t, 120, 40)
	start := pointForText(t, m, "exmd feature tour")
	end := selPoint{line: start.line, col: start.col + len("exmd")}
	m = beginDrag(m, start, end)

	// Column 0 is the rail, outside the content pane.
	m = updateModel(m, tea.MouseMsg{
		X: 0, Y: 3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if m.sel.set || strings.Contains(m.View(), selectionOn) {
		t.Fatal("click outside the content did not clear the selection")
	}
}

func TestCopyFailureOnReleaseIsReported(t *testing.T) {
	m := newViewer(t, 120, 40)
	start := pointForText(t, m, "exmd feature tour")
	end := selPoint{line: start.line, col: start.col + len("exmd")}
	m = beginDrag(m, start, end)

	prev := clipboardOut
	clipboardOut = errWriter{err: errors.New("clipboard unavailable")}
	defer func() { clipboardOut = prev }()
	t.Setenv("TERM", "xterm-256color")

	m = updateModel(m, mouseAt(m, end, tea.MouseActionRelease, tea.MouseButtonNone))
	if !strings.Contains(m.flash, "copy failed: clipboard unavailable") {
		t.Fatalf("footer reaction = %q, want the write error", m.flash)
	}
	if m.sel.set {
		t.Fatal("selection was not cleared after a failed copy")
	}
}

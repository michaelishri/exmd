package tui

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// captureClipboard swaps the OSC 52 destination for the duration of fn and
// returns the raw bytes that would have gone to the terminal.
func captureClipboard(t *testing.T, fn func()) string {
	t.Helper()
	prev := clipboardOut
	var buf bytes.Buffer
	clipboardOut = &buf
	t.Setenv("TERM", "xterm-256color")
	defer func() { clipboardOut = prev }()
	fn()
	return buf.String()
}

// clipPayload decodes the text carried by the first OSC 52 copy sequence.
func clipPayload(t *testing.T, seq string) string {
	t.Helper()
	const prefix = "\x1b]52;c;"
	i := strings.Index(seq, prefix)
	if i < 0 {
		t.Fatalf("no OSC 52 copy sequence in %q", seq)
	}
	data := seq[i+len(prefix):]
	end := strings.IndexByte(data, '\a')
	if end < 0 {
		t.Fatalf("unterminated OSC 52 sequence: %q", data)
	}
	payload, err := base64.StdEncoding.DecodeString(data[:end])
	if err != nil {
		t.Fatalf("decode clipboard payload: %v", err)
	}
	return string(payload)
}

// runSelectedBlock presses r and pumps the inline execution to completion.
func runSelectedBlock(t *testing.T, m Model) Model {
	t.Helper()
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	return pump(t, mm.(Model), cmd)
}

func pressCopy(m Model) Model {
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	return mm.(Model)
}

// TestCopyFocusedBlockOutput checks that c copies exactly the focused block's
// plain output: no code, no output-box chrome, and the same text the block
// streamed into the document.
func TestCopyFocusedBlockOutput(t *testing.T) {
	got := runSelectedBlock(t, enterBlockModeFromFirstHeading(t, newViewer(t, 120, 40)))
	if !got.inline[0].done {
		t.Fatal("inline run did not finish")
	}
	seq := captureClipboard(t, func() { got = pressCopy(got) })

	want := strings.Join(got.inline[0].lines, "\n")
	text := clipPayload(t, seq)
	if text != want {
		t.Fatalf("copied %q, want %q", text, want)
	}
	if !strings.Contains(text, "author:  Michael Ishri") {
		t.Fatalf("copied text missing block output: %q", text)
	}
	if strings.Contains(text, `echo "author:  $AUTHOR"`) {
		t.Fatalf("copied text contains the block's source: %q", text)
	}
	if strings.Contains(text, "─") {
		t.Fatalf("copied text contains the output box separator: %q", text)
	}
	if !strings.Contains(got.flash, "copied") || !strings.Contains(got.flash, "block 1") {
		t.Fatalf("footer reaction = %q, want a copy confirmation for block 1", got.flash)
	}
}

// TestCopyFollowsBlockSelection checks that only the block under the cursor
// is copied, even when other blocks also carry output.
func TestCopyFollowsBlockSelection(t *testing.T) {
	got := enterBlockModeFromFirstHeading(t, newViewer(t, 120, 40))
	got = runSelectedBlock(t, got) // block 0
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	got = runSelectedBlock(t, got) // block 1

	seq := captureClipboard(t, func() { got = pressCopy(got) })
	want := strings.Join(got.inline[1].lines, "\n")
	if text := clipPayload(t, seq); text != want {
		t.Fatalf("copied %q, want block 2's output %q", text, want)
	}
	if text := clipPayload(t, seq); strings.Contains(text, "author:") {
		t.Fatalf("copied text includes another block's output: %q", text)
	}
	if !strings.Contains(got.flash, "block 2") {
		t.Fatalf("footer reaction = %q, want block 2", got.flash)
	}
}

func TestCopyWithoutOutput(t *testing.T) {
	got := enterBlockModeFromFirstHeading(t, newViewer(t, 120, 40))
	seq := captureClipboard(t, func() { got = pressCopy(got) })
	if seq != "" {
		t.Fatalf("clipboard written with no output: %q", seq)
	}
	if !strings.Contains(got.flash, "no output to copy") || !strings.Contains(got.flash, "r") {
		t.Fatalf("footer reaction = %q, want a hint to run the block", got.flash)
	}
}

func TestCopyWithoutSelection(t *testing.T) {
	got := newViewer(t, 120, 40)
	seq := captureClipboard(t, func() { got = pressCopy(got) })
	if seq != "" {
		t.Fatalf("clipboard written with no selection: %q", seq)
	}
	if !strings.Contains(got.flash, "select a block") {
		t.Fatalf("footer reaction = %q, want a hint to select a block", got.flash)
	}
}

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCopyFailureIsReported(t *testing.T) {
	got := runSelectedBlock(t, enterBlockModeFromFirstHeading(t, newViewer(t, 120, 40)))
	prev := clipboardOut
	clipboardOut = errWriter{err: errors.New("terminal unavailable")}
	t.Setenv("TERM", "xterm-256color")
	defer func() { clipboardOut = prev }()

	got = pressCopy(got)
	if !strings.Contains(got.flash, "copy failed: terminal unavailable") {
		t.Fatalf("footer reaction = %q, want the write error", got.flash)
	}
}

// TestCopiedText checks the plain-text assembly directly, including a partial
// line still accumulating while a block runs.
func TestCopiedText(t *testing.T) {
	tests := []struct {
		name string
		st   inlineState
		want string
	}{
		{"empty", inlineState{}, ""},
		{"only pending", inlineState{pending: "partial"}, "partial"},
		{"complete lines", inlineState{lines: []string{"a", "b"}}, "a\nb"},
		{"lines and pending", inlineState{lines: []string{"a"}, pending: "b"}, "a\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.st.copiedText(); got != tt.want {
				t.Errorf("copiedText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCopyKeepsFullWidthLines checks that copy uses the stored output lines
// rather than the rendered box, which truncates lines to the pane width.
func TestCopyKeepsFullWidthLines(t *testing.T) {
	got := runSelectedBlock(t, enterBlockModeFromFirstHeading(t,
		newViewerFromDoc(t, 120, 40, "testdata/long_output.md", nil)))
	if got.content.Width >= 200 {
		t.Fatalf("test needs output wider than the pane (width = %d)", got.content.Width)
	}
	seq := captureClipboard(t, func() { got = pressCopy(got) })
	text := clipPayload(t, seq)
	if len(text) != 200 || strings.Trim(text, "0") != "" {
		t.Fatalf("copied %d chars, want 200 zeros: %q", len(text), text)
	}
	if want := strings.Join(got.inline[0].lines, "\n"); text != want {
		t.Fatalf("copied %q, want the full stored line %q", text, want)
	}
}

func TestCopyFlashClearsOnNextKey(t *testing.T) {
	got := runSelectedBlock(t, enterBlockModeFromFirstHeading(t, newViewer(t, 120, 40)))
	_ = captureClipboard(t, func() { got = pressCopy(got) })
	if got.flash == "" {
		t.Fatal("copy did not leave a footer reaction")
	}
	mm, _ := got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = mm.(Model)
	if got.flash != "" {
		t.Fatalf("footer reaction persisted after the next key: %q", got.flash)
	}
	lines := strings.Split(got.View(), "\n")
	footer := ansi.Strip(lines[len(lines)-2])
	if !strings.Contains(footer, "j/k blocks") || !strings.Contains(footer, "c copy") {
		t.Fatalf("key hints did not return: %q", footer)
	}
}

func TestCopySequenceWrapsForTmuxAndScreen(t *testing.T) {
	for _, tt := range []struct {
		term string
		want string
	}{
		{"xterm-256color", ""},
		{"tmux-256color", "\x1bPtmux;\x1b"},
		{"screen-256color", "\x1bP"},
	} {
		t.Run(tt.term, func(t *testing.T) {
			t.Setenv("TERM", tt.term)
			prev := clipboardOut
			var buf bytes.Buffer
			clipboardOut = &buf
			defer func() { clipboardOut = prev }()

			if err := copyToClipboard("clip"); err != nil {
				t.Fatalf("copyToClipboard: %v", err)
			}
			if !strings.HasPrefix(buf.String(), tt.want) {
				t.Fatalf("sequence = %q, want prefix %q", buf.String(), tt.want)
			}
			if text := clipPayload(t, buf.String()); text != "clip" {
				t.Fatalf("decoded text = %q, want clip", text)
			}
		})
	}
}

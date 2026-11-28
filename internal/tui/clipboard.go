package tui

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/aymanbagabas/go-osc52/v2"
)

// clipboardSequence builds the OSC 52 sequence that puts text on the
// terminal's clipboard. The sequence travels through the terminal, so it
// works over SSH and needs no external clipboard helpers. tmux and screen
// need their escape wrappers unless their clipboard passthrough is enabled.
func clipboardSequence(text string) string {
	seq := osc52.New(text)
	term := os.Getenv("TERM")
	switch {
	case strings.Contains(term, "tmux"):
		seq = seq.Tmux()
	case strings.Contains(term, "screen"):
		seq = seq.Screen()
	}
	return seq.String()
}

// copyToClipboard returns a command that copies text to the clipboard. Bubble
// Tea writes the sequence itself, in order with the frames it draws, so the
// two can never interleave.
func copyToClipboard(text string) tea.Cmd {
	return tea.Raw(clipboardSequence(text))
}

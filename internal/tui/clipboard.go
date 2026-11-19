package tui

import (
	"io"
	"os"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
)

// clipboardOut receives OSC 52 sequences. It is a variable so tests can
// capture what would be written to the terminal without touching a real
// clipboard.
var clipboardOut io.Writer = os.Stdout

// copyToClipboard writes text to the terminal's clipboard with OSC 52. The
// sequence travels through the terminal, so it works over SSH and needs no
// external clipboard helpers. tmux and screen need their escape wrappers
// unless their clipboard passthrough is enabled.
func copyToClipboard(text string) error {
	seq := osc52.New(text)
	term := os.Getenv("TERM")
	switch {
	case strings.Contains(term, "tmux"):
		seq = seq.Tmux()
	case strings.Contains(term, "screen"):
		seq = seq.Screen()
	}
	_, err := seq.WriteTo(clipboardOut)
	return err
}

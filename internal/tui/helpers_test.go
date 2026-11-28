package tui

import (
	"fmt"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/michaelishri/exmd/internal/theme"
)

// TestMain keeps every test's automatic session saves inside a temporary
// directory instead of the user's real config.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "exmd-tui-test-")
	if err == nil {
		_ = os.Setenv("XDG_CONFIG_HOME", dir)
	}
	code := m.Run()
	if err == nil {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}

// pump executes a tea.Cmd chain synchronously, feeding every produced
// message back through Update until the queue drains. It lets the inline
// execution tests drive the event pipeline without a live terminal.
func pump(t *testing.T, m Model, initial tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{initial}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		nm, cmd := m.Update(msg)
		m = nm.(Model)
		if cmd != nil {
			queue = append(queue, cmd)
		}
	}
	return m
}

// useProfile makes new viewers assume the given color profile and returns a
// function that restores the previous one.
func useProfile(p colorprofile.Profile) func() {
	prev := startProfile
	SetColorProfile(p)
	return func() { SetColorProfile(prev) }
}

// trueColorSeq is the SGR parameter string lipgloss emits for a theme color:
// 38;2;R;G;B for a foreground, 48;2;R;G;B for a background.
func trueColorSeq(c theme.Color, bg bool) string {
	r, g, b, _ := lipgloss.Color(string(c)).RGBA()
	kind := 38
	if bg {
		kind = 48
	}
	return fmt.Sprintf("%d;2;%d;%d;%d", kind, r>>8, g>>8, b>>8)
}

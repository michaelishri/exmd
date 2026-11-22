package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

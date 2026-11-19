package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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

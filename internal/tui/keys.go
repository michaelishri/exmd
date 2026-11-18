package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the viewer's key surface; it doubles as the help model's source.
type keyMap struct {
	Scroll key.Binding
	Jump   key.Binding
	Focus  key.Binding
	Tab    key.Binding
	Help   key.Binding
	Quit   key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Scroll: key.NewBinding(
			key.WithKeys("j", "k", "up", "down"),
			key.WithHelp("j/k", "scroll"),
		),
		Jump: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "jump"),
		),
		Focus: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab", "focus"),
		),
		Tab: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "outline/blocks"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Scroll, k.Focus, k.Tab, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	topBottom := key.NewBinding(
		key.WithKeys("g", "G"),
		key.WithHelp("g/G", "top/bottom"),
	)
	pages := key.NewBinding(
		key.WithKeys("pgup", "pgdown", "ctrl+u", "ctrl+d"),
		key.WithHelp("pgup/pgdn", "page"),
	)
	return [][]key.Binding{
		{k.Scroll, pages, topBottom},
		{k.Focus, k.Tab, k.Jump},
		{k.Help, k.Quit},
	}
}

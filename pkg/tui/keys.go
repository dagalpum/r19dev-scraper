package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines keybindings for the TUI application.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	Top       key.Binding
	Bottom    key.Binding
	Edit      key.Binding
	Scrape    key.Binding
	ScrapeAll key.Binding
	Rescan    key.Binding
	Help      key.Binding
	Quit      key.Binding
	Enter     key.Binding
	Esc       key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("pgdn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "bottom"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit ID"),
		),
		Scrape: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("enter", "scrape detail"),
		),
		ScrapeAll: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "scrape all"),
		),
		Rescan: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rescan"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

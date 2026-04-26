package ui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys are handled by the root App model. Quit is split from
// ForceQuit because q is context-sensitive (closes the viewer, clears
// an active search, then quits) while Ctrl+C always quits.
type GlobalKeys struct {
	Help      key.Binding
	Quit      key.Binding
	ForceQuit key.Binding
	CloseHelp key.Binding
}

// NewGlobalKeys returns the default global key bindings.
func NewGlobalKeys() GlobalKeys {
	return GlobalKeys{
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
		CloseHelp: key.NewBinding(key.WithKeys("?", "esc"), key.WithHelp("?/esc", "close help")),
	}
}

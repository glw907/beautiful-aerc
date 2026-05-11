package messagelist

import "charm.land/bubbles/v2/key"

// KeyMap collects the bindings messagelist.Update dispatches on.
// Only navigation keys live here — fold, visual-mode, and triage
// bindings need account-level guards and stay in account.keys.
type KeyMap struct {
	Down   key.Binding
	Up     key.Binding
	Top    key.Binding
	Bottom key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		Top:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	}
}

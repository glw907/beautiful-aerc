package contacts

import "github.com/charmbracelet/bubbles/key"

// keys are the bindings active inside contacts surfaces.
var keys = struct {
	Esc    key.Binding
	I      key.Binding
	N      key.Binding
	JUpper key.Binding
	KUpper key.Binding
	D      key.Binding
}{
	Esc:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "dismiss")),
	I:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "close")),
	N:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new contact")),
	JUpper: key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "next group")),
	KUpper: key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "prev group")),
	D:      key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete contact")),
}

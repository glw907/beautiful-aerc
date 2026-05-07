// SPDX-License-Identifier: MIT

package contacts

import "github.com/charmbracelet/bubbles/key"

// keys holds the bindings active inside contacts surfaces (popover, sidebar,
// list, form). Package-level so all surfaces share the same declarations
// without a per-instance allocator.
var keys = struct {
	Esc    key.Binding
	I      key.Binding
	N      key.Binding
	JUpper key.Binding
	KUpper key.Binding
}{
	Esc:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "dismiss")),
	I:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "close")),
	N:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new contact")),
	JUpper: key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "next group")),
	KUpper: key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "prev group")),
}

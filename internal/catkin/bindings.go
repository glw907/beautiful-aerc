package catkin

import (
	"charm.land/bubbles/v2/key"

	"github.com/glw907/poplar/internal/ui/uicore"
)

// ChordSet returns Catkin's command vocabulary. Entries tagged
// RequiresKittyKbd carry semantics (bold, italic, link, list, quote,
// task) that depend on Ctrl+letter chord disambiguation; on terminals
// without the Kitty keyboard protocol, helppopover hides them and the
// catkin status footer collapses to the plain markdown hint.
func ChordSet() []uicore.GatedBinding {
	return []uicore.GatedBinding{
		{Binding: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("^B", "bold")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+i"), key.WithHelp("^I", "italic")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("^K", "link")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("^L", "list")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+q"), key.WithHelp("^Q", "quote")), RequiresKittyKbd: true},
		{Binding: key.NewBinding(key.WithKeys("ctrl+@", "ctrl+ "), key.WithHelp("^@", "task")), RequiresKittyKbd: true},
	}
}

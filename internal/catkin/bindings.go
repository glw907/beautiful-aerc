package catkin

import (
	"charm.land/bubbles/v2/key"

	"github.com/glw907/poplar/internal/ui/uicore"
)

var chordSet = []uicore.GatedBinding{
	{Binding: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("^B", "bold")), RequiresKittyKbd: true},
	{Binding: key.NewBinding(key.WithKeys("ctrl+i"), key.WithHelp("^I", "italic")), RequiresKittyKbd: true},
	{Binding: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("^K", "link")), RequiresKittyKbd: true},
	{Binding: key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("^L", "list")), RequiresKittyKbd: true},
	{Binding: key.NewBinding(key.WithKeys("ctrl+q"), key.WithHelp("^Q", "quote")), RequiresKittyKbd: true},
	{Binding: key.NewBinding(key.WithKeys("ctrl+@", "ctrl+ "), key.WithHelp("^@", "task")), RequiresKittyKbd: true},
}

// ChordSet returns Catkin's command vocabulary. Entries tagged
// RequiresKittyKbd require Ctrl+letter chord disambiguation; without the
// Kitty keyboard protocol they fold into the plain markdown hint.
func ChordSet() []uicore.GatedBinding { return chordSet }

// ActiveChords returns the chords that should render given the negotiated
// keyboard capabilities. Entries tagged RequiresKittyKbd drop when the
// protocol isn't active.
func ActiveChords(disambiguates bool) []uicore.GatedBinding {
	if disambiguates {
		return chordSet
	}
	out := make([]uicore.GatedBinding, 0, len(chordSet))
	for _, gb := range chordSet {
		if !gb.RequiresKittyKbd {
			out = append(out, gb)
		}
	}
	return out
}

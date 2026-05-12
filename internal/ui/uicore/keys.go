package uicore

import "charm.land/bubbles/v2/key"

// GatedBinding pairs a key.Binding with a capability flag.
// RequiresKittyKbd marks chords that rely on Kitty keyboard protocol
// disambiguation; without it, Ctrl+letter chords collide with
// Tab/Enter/Backspace in the legacy ASCII mapping.
type GatedBinding struct {
	Binding          key.Binding
	RequiresKittyKbd bool
}

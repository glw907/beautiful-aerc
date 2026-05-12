package uicore

import "charm.land/bubbles/v2/key"

// GatedBinding pairs a key.Binding with a capability tag. Bindings whose
// semantics require the Kitty keyboard protocol (Ctrl+letter chords that
// the legacy ASCII mapping confuses with Tab/Enter/Backspace) carry
// RequiresKittyKbd = true. helppopover filters them out when the protocol
// isn't negotiated.
type GatedBinding struct {
	Binding          key.Binding
	RequiresKittyKbd bool
}

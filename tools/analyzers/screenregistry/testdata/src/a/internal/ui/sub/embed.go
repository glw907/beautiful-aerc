package sub

import "a/internal/ui"

// EmbedScreen implements ui.Screen only through ui.GoodScreen's
// promoted methods (it declares none of Init, Update, View, or Entry
// itself) and is never registered: the promoted-method embedding
// evasion, the case a syntax-only scan of declared methods can never
// see, since go/types' Implements resolves the promoted method set
// the same way the compiler does.
type EmbedScreen struct { // want "type EmbedScreen implements ui.Screen but is never registered via Register"
	ui.GoodScreen
}

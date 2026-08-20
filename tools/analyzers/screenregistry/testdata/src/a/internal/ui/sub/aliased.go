package sub

import u "a/internal/ui"

// AliasScreen implements u.Screen through an aliased import of ui and
// is never registered: the aliased-import evasion, proving Screen
// lookup and Register resolution both work through the alias rather
// than the exact identifier "ui".
type AliasScreen struct{} // want "type AliasScreen implements ui.Screen but is never registered via Register"

func (AliasScreen) Init() int             { return 0 }
func (AliasScreen) Update(int) (int, int) { return 0, 0 }
func (AliasScreen) View() string          { return "" }
func (AliasScreen) Entry() u.ScreenEntry  { return u.ScreenEntry{} }

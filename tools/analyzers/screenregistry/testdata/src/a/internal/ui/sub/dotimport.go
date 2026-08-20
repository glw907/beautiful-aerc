package sub

import . "a/internal/ui"

// DotScreen implements the dot-imported Screen and is never
// registered: the dot-import evasion.
type DotScreen struct{} // want "type DotScreen implements ui.Screen but is never registered via Register"

func (DotScreen) Init() int             { return 0 }
func (DotScreen) Update(int) (int, int) { return 0, 0 }
func (DotScreen) View() string          { return "" }
func (DotScreen) Entry() ScreenEntry    { return ScreenEntry{} }

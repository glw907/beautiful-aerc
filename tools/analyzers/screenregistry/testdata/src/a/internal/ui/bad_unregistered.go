package ui

// UnregisteredScreen implements Screen but no init ever registers
// it: the baseline the whole analyzer exists to catch.
type UnregisteredScreen struct{} // want "type UnregisteredScreen implements ui.Screen but is never registered via Register"

func (UnregisteredScreen) Init() int             { return 0 }
func (UnregisteredScreen) Update(int) (int, int) { return 0, 0 }
func (UnregisteredScreen) View() string          { return "" }
func (UnregisteredScreen) Entry() ScreenEntry    { return ScreenEntry{} }

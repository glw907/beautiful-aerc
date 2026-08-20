package ui

// GoodScreen implements Screen and is registered: the happy path.
type GoodScreen struct{}

func (GoodScreen) Init() int             { return 0 }
func (GoodScreen) Update(int) (int, int) { return 0, 0 }
func (GoodScreen) View() string          { return "" }
func (GoodScreen) Entry() ScreenEntry    { return ScreenEntry{} }

func init() { Register[GoodScreen](ScreenEntry{}) }

// Package ui is a minimal stand-in for the real internal/ui, just
// enough of Screen and Register's shape for screenregistry's
// analysistest fixtures. types.Implements matches structurally, so
// the stub method signatures below need not match the real
// tea.Model shape at all.
package ui

type Screen interface {
	Init() int
	Update(int) (int, int)
	View() string
	Entry() ScreenEntry
}

type ScreenEntry struct {
	Name string
}

func Register[S Screen](entry ScreenEntry) {}

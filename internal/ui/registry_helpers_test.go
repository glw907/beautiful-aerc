package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/key"
)

// fakeKeyMap is a minimal help.KeyMap for constructing synthetic
// ScreenEntry fixtures across this package's registry tests.
type fakeKeyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (k fakeKeyMap) ShortHelp() []key.Binding  { return k.short }
func (k fakeKeyMap) FullHelp() [][]key.Binding { return k.full }

// flatKeyMap builds a fakeKeyMap whose full help is one column
// holding bindings, and whose short help is the same list.
func flatKeyMap(bindings ...key.Binding) fakeKeyMap {
	return fakeKeyMap{short: bindings, full: [][]key.Binding{bindings}}
}

// bind is shorthand for a key.Binding carrying one physical key and
// its help text.
func bind(k, helpKey, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(k), key.WithHelp(helpKey, desc))
}

// resetRegistry clears the package-level registry for the duration
// of the calling test, restoring the prior contents on cleanup, so a
// test exercising Register directly does not pollute the entries
// other tests (or a later screen's init) contribute.
func resetRegistry(t *testing.T) {
	t.Helper()
	saved := registered
	registered = nil
	t.Cleanup(func() { registered = saved })
}

// valueScreen is a minimal Screen with value receivers, for
// exercising Register[valueScreen] directly.
type valueScreen struct{}

func (valueScreen) Init() tea.Cmd                       { return nil }
func (valueScreen) Update(tea.Msg) (tea.Model, tea.Cmd) { return valueScreen{}, nil }
func (valueScreen) View() tea.View                      { return tea.NewView("") }
func (valueScreen) Entry() ScreenEntry                  { return ScreenEntry{} }

// fakeScreen is a minimal Screen whose methods carry pointer
// receivers, so Register[*fakeScreen] exercises the
// pointer-normalization path (M13).
type fakeScreen struct{}

func (*fakeScreen) Init() tea.Cmd                       { return nil }
func (*fakeScreen) Update(tea.Msg) (tea.Model, tea.Cmd) { return &fakeScreen{}, nil }
func (*fakeScreen) View() tea.View                      { return tea.NewView("") }
func (*fakeScreen) Entry() ScreenEntry                  { return ScreenEntry{} }

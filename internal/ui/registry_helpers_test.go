package ui

import (
	"testing"

	"charm.land/bubbles/v2/key"
)

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

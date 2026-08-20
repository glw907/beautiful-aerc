package ui

// AccountScoped holds a per-account copy of a UI-state value (C4):
// a screen's cursor, scroll position, or search scope lives in one
// of these instead of a bare field, so every account-keyed value in
// the UI layer is findable by type rather than by audit once a
// second account arrives. The zero value is ready to use.
type AccountScoped[T any] struct {
	byAccount map[string]T
}

// Get returns account's stored value, or T's zero value when none
// has been set yet.
func (a AccountScoped[T]) Get(account string) T {
	return a.byAccount[account]
}

// Set stores value for account.
func (a *AccountScoped[T]) Set(account string, value T) {
	if a.byAccount == nil {
		a.byAccount = make(map[string]T)
	}
	a.byAccount[account] = value
}

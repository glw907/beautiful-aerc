package ui

import "testing"

func TestAccountScopedRoundTrip(t *testing.T) {
	var cursors AccountScoped[int]

	cursors.Set("geoff@907.life", 42)
	cursors.Set("other@example.com", 7)

	if got := cursors.Get("geoff@907.life"); got != 42 {
		t.Errorf("Get(geoff) = %d, want 42", got)
	}
	if got := cursors.Get("other@example.com"); got != 7 {
		t.Errorf("Get(other) = %d, want 7", got)
	}
}

func TestAccountScopedZeroValueForUnsetAccount(t *testing.T) {
	var cursors AccountScoped[int]

	if got := cursors.Get("nobody@example.com"); got != 0 {
		t.Errorf("Get on unset account = %d, want 0", got)
	}
}

func TestAccountScopedZeroValueUsableBeforeSet(t *testing.T) {
	var scroll AccountScoped[string]

	if got := scroll.Get("geoff@907.life"); got != "" {
		t.Errorf("Get on zero-value AccountScoped = %q, want empty", got)
	}
}

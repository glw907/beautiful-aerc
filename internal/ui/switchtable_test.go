package ui

import (
	"reflect"
	"testing"
)

// digitsSwitchStates transcribes design language section 2's list of
// states where the digit keys switch surfaces: the eleven-entry
// authority M10's switch-table test checks declared states against.
var digitsSwitchStates = []string{
	"mail list", "thread view", "reader",
	"calendar agenda", "calendar grid views", "event detail",
	"contact list", "contact card",
	"config sections", "help overlay",
	"compose's message-level command state",
}

// printableEntryStates transcribes design language section 2's list
// of states where digits are input, reached and left through Esc
// first: the six-entry authority whose membership the switch-table
// test asserts equals the set of screens accepting printable input.
var printableEntryStates = []string{
	"compose headers", "Catkin's entry state", "Catkin's command state",
	"the search bar", "form fields", "picker filter fields",
}

func TestSwitchTable_AuthorityListCounts(t *testing.T) {
	if len(digitsSwitchStates) != 11 {
		t.Errorf("digitsSwitchStates has %d entries, want 11 (design language section 2)", len(digitsSwitchStates))
	}
	if len(printableEntryStates) != 6 {
		t.Errorf("printableEntryStates has %d entries, want 6 (design language section 2)", len(printableEntryStates))
	}
}

func TestValidSwitchStates(t *testing.T) {
	entries := []ScreenEntry{
		{Type: reflect.TypeOf(struct{ a int }{}), SwitchState: StateDigitsSwitch},
		{Type: reflect.TypeOf(struct{ b int }{}), SwitchState: StatePrintableEntry},
		{Type: reflect.TypeOf(struct{ c int }{}), SwitchState: StateModal},
	}

	if got := validSwitchStates(entries); len(got) != 0 {
		t.Errorf("validSwitchStates() on the three UX-4 classes = %v, want none", got)
	}
}

func TestValidSwitchStates_RejectsAStrayValue(t *testing.T) {
	entries := []ScreenEntry{
		{Type: reflect.TypeOf(struct{}{}), SwitchState: StateClass(99)},
	}

	if got := validSwitchStates(entries); len(got) != 1 {
		t.Fatalf("validSwitchStates() with a stray SwitchState = %v, want one violation", got)
	}
}

// TestValidSwitchStates_LiveRegistry is CR1's live check: every
// currently registered screen resolves to one of the UX-4 closed
// states.
func TestValidSwitchStates_LiveRegistry(t *testing.T) {
	if got := validSwitchStates(Registered()); len(got) != 0 {
		t.Errorf("validSwitchStates(Registered()) = %v, want none", got)
	}
}

// TestPrintableEntryScreens_IsEmptyThisPass asserts UX-4's
// printable-entry acceptance criterion against the live registry: no
// screen accepts printable input yet (pass 2 has no forms), so the
// printable-entry set must be exactly empty. The assertion goes live
// with pass 2b's forms, per the design-decisions record.
func TestPrintableEntryScreens_IsEmptyThisPass(t *testing.T) {
	if got := printableEntryScreens(Registered()); len(got) != 0 {
		t.Errorf("printableEntryScreens(Registered()) = %v, want none this pass", got)
	}
}

func TestPrintableEntryScreens_SelectsOnlyThatState(t *testing.T) {
	entries := []ScreenEntry{
		{Type: reflect.TypeOf(struct{ a int }{}), SwitchState: StateDigitsSwitch},
		{Type: reflect.TypeOf(struct{ b int }{}), SwitchState: StatePrintableEntry},
		{Type: reflect.TypeOf(struct{ c int }{}), SwitchState: StateModal},
	}

	got := printableEntryScreens(entries)
	if len(got) != 1 || got[0] != entries[1].Type {
		t.Errorf("printableEntryScreens() = %v, want only %v", got, entries[1].Type)
	}
}

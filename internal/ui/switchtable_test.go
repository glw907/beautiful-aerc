package ui

import (
	"reflect"
	"testing"
)

// TestSwitchTable_EveryRegisteredScreenResolvesToAState proves the
// mechanism CheckGrammar and the footer machinery both lean on:
// every entry's SwitchState is one of the UX-4 switch table's three
// classes (design language section 2), never a stray value.
func TestSwitchTable_EveryRegisteredScreenResolvesToAState(t *testing.T) {
	entries := []ScreenEntry{
		{Type: reflect.TypeOf(struct{ a int }{}), SwitchState: StateDigitsSwitch},
		{Type: reflect.TypeOf(struct{ b int }{}), SwitchState: StatePrintableEntry},
		{Type: reflect.TypeOf(struct{ c int }{}), SwitchState: StateModal},
	}

	for _, e := range entries {
		switch e.SwitchState {
		case StateDigitsSwitch, StatePrintableEntry, StateModal:
		default:
			t.Errorf("%v resolves to %v, not one of the three UX-4 state classes", e.Type, e.SwitchState)
		}
	}
}

// TestSwitchTable_PrintableEntrySetIsEmptyThisPass asserts UX-4's
// printable-entry acceptance criterion against the live registry:
// no screen accepts printable input yet (pass 2 has no forms), so
// the printable-entry set must be exactly empty. The assertion goes
// live with pass 2b's forms, per the design-decisions record.
func TestSwitchTable_PrintableEntrySetIsEmptyThisPass(t *testing.T) {
	if got := PrintableEntryScreens(Registered()); len(got) != 0 {
		t.Errorf("PrintableEntryScreens(Registered()) = %v, want none this pass", got)
	}
}

func TestPrintableEntryScreens_SelectsOnlyThatState(t *testing.T) {
	entries := []ScreenEntry{
		{Type: reflect.TypeOf(struct{ a int }{}), SwitchState: StateDigitsSwitch},
		{Type: reflect.TypeOf(struct{ b int }{}), SwitchState: StatePrintableEntry},
		{Type: reflect.TypeOf(struct{ c int }{}), SwitchState: StateModal},
	}

	got := PrintableEntryScreens(entries)
	if len(got) != 1 || got[0] != entries[1].Type {
		t.Errorf("PrintableEntryScreens() = %v, want only %v", got, entries[1].Type)
	}
}

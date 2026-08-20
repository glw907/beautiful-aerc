package ui

import (
	"reflect"
	"testing"
)

func TestCheckPointerGrammar_LegalBindingPasses(t *testing.T) {
	openMsg := bind("enter", "enter", "open")
	entry := ScreenEntry{
		Type:        reflect.TypeOf(struct{}{}),
		Keys:        flatKeyMap(openMsg),
		SwitchState: StateDigitsSwitch,
		Pointer: []PointerBinding{
			{Target: PointerRowOpen, Key: openMsg},
		},
	}

	if got := CheckPointerGrammar([]ScreenEntry{entry}); len(got) != 0 {
		t.Errorf("CheckPointerGrammar on a legal binding = %v, want none", got)
	}
}

func TestCheckPointerGrammar_AbsentVerbFails(t *testing.T) {
	// The screen's keymap never binds "enter"; the pointer target
	// names a verb the screen does not itself offer.
	entry := ScreenEntry{
		Type:        reflect.TypeOf(struct{}{}),
		Keys:        flatKeyMap(bind("q", "q", "quit")),
		SwitchState: StateDigitsSwitch,
		Pointer: []PointerBinding{
			{Target: PointerRowOpen, Key: bind("enter", "enter", "open")},
		},
	}

	got := CheckPointerGrammar([]ScreenEntry{entry})
	if len(got) != 1 {
		t.Fatalf("CheckPointerGrammar with an absent verb = %v, want exactly one violation", got)
	}
}

func TestCheckPointerGrammar_IllegalStateFails(t *testing.T) {
	// A surface-digit click only switches surfaces in
	// StateDigitsSwitch (ADR-0017); a modal claiming it is illegal.
	digitSwitch := bind("1", "1-4", "surface switch")
	entry := ScreenEntry{
		Type:        reflect.TypeOf(struct{}{}),
		Keys:        flatKeyMap(digitSwitch),
		SwitchState: StateModal,
		Pointer: []PointerBinding{
			{Target: PointerSurfaceDigit, Key: digitSwitch},
		},
	}

	got := CheckPointerGrammar([]ScreenEntry{entry})
	if len(got) != 1 {
		t.Fatalf("CheckPointerGrammar with an illegal-state target = %v, want exactly one violation", got)
	}
}

func TestCheckPointerGrammar_ModalAnswerLegalOnlyInModal(t *testing.T) {
	yes := bind("y", "y", "yes")
	entry := ScreenEntry{
		Type:        reflect.TypeOf(struct{}{}),
		Keys:        flatKeyMap(yes),
		SwitchState: StateModal,
		Pointer: []PointerBinding{
			{Target: PointerModalAnswer, Key: yes},
		},
	}

	if got := CheckPointerGrammar([]ScreenEntry{entry}); len(got) != 0 {
		t.Errorf("CheckPointerGrammar for a modal answer inside a modal = %v, want none", got)
	}
}

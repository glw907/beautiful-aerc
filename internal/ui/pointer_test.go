package ui

import (
	"reflect"
	"testing"
)

// TestCheckPointerGrammar exercises checkPointerGrammar against
// ADR-0017's pointer vocabulary.
func TestCheckPointerGrammar(t *testing.T) {
	openMsg := bind("enter", "enter", "open")
	digitSwitch := bind("1", "1-4", "surface switch")
	yes := bind("y", "y", "yes")

	tests := []struct {
		name    string
		entry   ScreenEntry
		wantLen int
	}{
		{
			name: "legal binding passes",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(openMsg),
				SwitchState: StateDigitsSwitch,
				Pointer:     []PointerBinding{{Target: PointerRowOpen, Key: openMsg}},
			},
			wantLen: 0,
		},
		{
			name: "verb absent from the screen's own keymap fails",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("q", "q", "quit")), // never binds "enter"/"open"
				SwitchState: StateDigitsSwitch,
				Pointer:     []PointerBinding{{Target: PointerRowOpen, Key: openMsg}},
			},
			wantLen: 1,
		},
		{
			name: "surface-digit click illegal outside StateDigitsSwitch (ADR-0017)",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(digitSwitch),
				SwitchState: StateModal,
				Pointer:     []PointerBinding{{Target: PointerSurfaceDigit, Key: digitSwitch}},
			},
			wantLen: 1,
		},
		{
			name: "modal answer legal only in StateModal",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(yes),
				SwitchState: StateModal,
				Pointer:     []PointerBinding{{Target: PointerModalAnswer, Key: yes}},
			},
			wantLen: 0,
		},
		{
			name: "pane click illegal in a modal (M14)",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("tab", "tab", "focus pane")),
				SwitchState: StateModal,
				Pointer:     []PointerBinding{{Target: PointerPane, Key: bind("tab", "tab", "focus pane")}},
			},
			wantLen: 1,
		},
		{
			name: "wheel legal in a picker's filtered list, not only digits-switch (M15)",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("j", "j/k", "navigate")),
				SwitchState: StatePrintableEntry,
				Pointer:     []PointerBinding{{Target: PointerWheel, Key: bind("j", "j/k", "navigate")}},
			},
			wantLen: 0,
		},
		{
			name: "drag-in-reader select target legal in StateDigitsSwitch (M16)",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("y", "y", "yank")),
				SwitchState: StateDigitsSwitch,
				Pointer:     []PointerBinding{{Target: PointerDragSelect, Key: bind("y", "y", "yank")}},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkPointerGrammar([]ScreenEntry{tt.entry}); len(got) != tt.wantLen {
				t.Errorf("checkPointerGrammar() = %v, want %d violation(s)", got, tt.wantLen)
			}
		})
	}
}

// TestCheckPointerGrammar_LiveRegistry is CR1's live check.
func TestCheckPointerGrammar_LiveRegistry(t *testing.T) {
	if got := checkPointerGrammar(Registered()); len(got) != 0 {
		t.Errorf("checkPointerGrammar(Registered()) = %v, want none", got)
	}
}

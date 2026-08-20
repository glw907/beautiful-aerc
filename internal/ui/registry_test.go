package ui

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
)

// grammarTable is the interaction grammar's canonical key-to-verb
// mapping (design language, 2026-07-27, section 2), transcribed as
// test data: the global verb table, the triage verb set (identical
// from list, thread view, and reader per LT-2), the folder-jump
// capitals, calendar's `t`, and thread fold/unfold. CheckGrammar
// checks every registered, non-exempt screen's bindings against it.
var grammarTable = map[string]key.Binding{
	// The global verb table.
	"j":     bind("j", "j/k", "navigate"),
	"k":     bind("k", "j/k", "navigate"),
	"down":  bind("down", "j/k", "navigate"),
	"up":    bind("up", "j/k", "navigate"),
	"space": bind("space", "space/b", "page"),
	"b":     bind("b", "space/b", "page"),
	"home":  bind("home", "home/end", "extremes"),
	"end":   bind("end", "home/end", "extremes"),
	"G":     bind("G", "home/end", "extremes"),
	"enter": bind("enter", "enter", "open"),
	"esc":   bind("esc", "esc", "back"),
	"n":     bind("n", "n/p", "message step"),
	"p":     bind("p", "n/p", "message step"),
	"/":     bind("/", "/", "search"),
	"g":     bind("g", "g", "goto"),
	"tab":   bind("tab", "tab", "next unread"),
	"x":     bind("x", "x", "select"),
	";":     bind(";", ";", "select by"),
	"u":     bind("u", "u", "undo"),
	"?":     bind("?", "?", "help"),
	"q":     bind("q", "q", "quit"),
	"1":     bind("1", "1-4", "surface switch"),
	"2":     bind("2", "1-4", "surface switch"),
	"3":     bind("3", "1-4", "surface switch"),
	"4":     bind("4", "1-4", "surface switch"),

	// The triage verb set (LT-2).
	"a": bind("a", "a", "archive"),
	"d": bind("d", "d", "delete"),
	"*": bind("*", "*", "flag toggle"),
	"e": bind("e", "e", "read toggle"),
	"s": bind("s", "s", "move"),
	"!": bind("!", "!", "junk toggle"),
	"r": bind("r", "r/R", "reply"),
	"R": bind("R", "r/R", "reply all"),
	"f": bind("f", "f", "forward"),
	"m": bind("m", "m", "compose"),
	"y": bind("y", "y", "yank"),
	"v": bind("v", "v", "attachments"),

	// Mail-surface folder jumps (FO-2).
	"I": bind("I", "I", "inbox"),
	"D": bind("D", "D", "drafts"),
	"S": bind("S", "S", "sent"),
	"A": bind("A", "A", "archive folder"),
	"J": bind("J", "J", "junk folder"),
	"T": bind("T", "T", "trash"),

	// Calendar-surface today jump.
	"t": bind("t", "t", "today"),

	// Thread fold/unfold in thread views.
	"h": bind("h", "h/l", "thread fold"),
	"l": bind("l", "h/l", "thread unfold"),
}

func TestCheckGrammar_ConformingScreenPasses(t *testing.T) {
	entry := ScreenEntry{
		Type: reflect.TypeOf(struct{}{}),
		Keys: flatKeyMap(
			bind("j", "j/k", "navigate"),
			bind("q", "q", "quit"),
		),
		SwitchState: StateDigitsSwitch,
	}

	if got := CheckGrammar([]ScreenEntry{entry}, grammarTable); len(got) != 0 {
		t.Errorf("CheckGrammar on a conforming screen = %v, want none", got)
	}
}

func TestCheckGrammar_AbsenceIsNotContradiction(t *testing.T) {
	// A calendar screen leaves the mail folder capitals unbound; the
	// grammar's own preamble states absence is not contradiction.
	entry := ScreenEntry{
		Type:        reflect.TypeOf(struct{}{}),
		Keys:        flatKeyMap(bind("t", "t", "today")),
		SwitchState: StateDigitsSwitch,
	}

	if got := CheckGrammar([]ScreenEntry{entry}, grammarTable); len(got) != 0 {
		t.Errorf("CheckGrammar on a screen that only binds a subset of the grammar = %v, want none", got)
	}
}

func TestCheckGrammar_ContradictingBindingFails(t *testing.T) {
	entry := ScreenEntry{
		Type:        reflect.TypeOf(struct{}{}),
		Keys:        flatKeyMap(bind("j", "j", "yank")), // j means navigate, not yank
		SwitchState: StateDigitsSwitch,
	}

	got := CheckGrammar([]ScreenEntry{entry}, grammarTable)
	if len(got) != 1 {
		t.Fatalf("CheckGrammar on a contradicting binding = %v, want exactly one violation", got)
	}
}

func TestCheckGrammar_ModalConfirmExempt(t *testing.T) {
	entry := ScreenEntry{
		Type:          reflect.TypeOf(struct{}{}),
		Keys:          flatKeyMap(bind("y", "y", "yes")), // y means yank elsewhere; legal here
		SwitchState:   StateModal,
		GrammarExempt: GrammarExemptModalConfirm,
	}

	if got := CheckGrammar([]ScreenEntry{entry}, grammarTable); len(got) != 0 {
		t.Errorf("CheckGrammar on the modal-confirm exemption = %v, want none", got)
	}
}

func TestCheckGrammar_CatkinCommandExempt(t *testing.T) {
	entry := ScreenEntry{
		Type:          reflect.TypeOf(struct{}{}),
		Keys:          flatKeyMap(bind("j", "j", "char right")), // Catkin's h/j/k/l are motion, not navigate
		SwitchState:   StateDigitsSwitch,
		GrammarExempt: GrammarExemptCatkinCommand,
	}

	if got := CheckGrammar([]ScreenEntry{entry}, grammarTable); len(got) != 0 {
		t.Errorf("CheckGrammar on the Catkin command-state exemption = %v, want none", got)
	}
}

// TestCheckGrammar_UnrecognizedExemptionStillChecked proves the
// exemption list is exactly two entries: a GrammarExemption value
// outside the two named constants is not recognized, so a
// contradicting binding under it is still reported.
func TestCheckGrammar_UnrecognizedExemptionStillChecked(t *testing.T) {
	entry := ScreenEntry{
		Type:          reflect.TypeOf(struct{}{}),
		Keys:          flatKeyMap(bind("j", "j", "yank")),
		SwitchState:   StateDigitsSwitch,
		GrammarExempt: GrammarExemption(99),
	}

	got := CheckGrammar([]ScreenEntry{entry}, grammarTable)
	if len(got) != 1 {
		t.Fatalf("CheckGrammar under an unrecognized exemption value = %v, want exactly one violation", got)
	}
}

func TestCheckGrammar_PrintableEntryOutOfScope(t *testing.T) {
	// The scope rule: text-entry states are governed by section 3,
	// not the browse/command grammar, so a printable-entry screen is
	// never checked even without claiming an exemption.
	entry := ScreenEntry{
		Type:        reflect.TypeOf(struct{}{}),
		Keys:        flatKeyMap(bind("j", "j", "type the letter j")),
		SwitchState: StatePrintableEntry,
	}

	if got := CheckGrammar([]ScreenEntry{entry}, grammarTable); len(got) != 0 {
		t.Errorf("CheckGrammar on a printable-entry screen = %v, want none (out of scope)", got)
	}
}

func TestRegisterAndRegistered(t *testing.T) {
	resetRegistry(t)

	entry := ScreenEntry{
		Type: reflect.TypeOf(struct{ x int }{}),
		Keys: flatKeyMap(bind("q", "q", "quit")),
	}
	Register(entry)

	got := Registered()
	if len(got) != 1 || got[0].Type != entry.Type {
		t.Fatalf("Registered() = %v, want one entry for %v", got, entry.Type)
	}

	// Registered returns a copy: mutating it must not reach back into
	// the package's own registered slice.
	got[0] = ScreenEntry{}
	if Registered()[0].Type != entry.Type {
		t.Error("Registered() returned a slice aliasing the internal registry")
	}
}

func TestRegisterPanicsOnNilType(t *testing.T) {
	resetRegistry(t)

	defer func() {
		if recover() == nil {
			t.Error("Register with a nil Type did not panic")
		}
	}()
	Register(ScreenEntry{})
}

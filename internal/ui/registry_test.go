package ui

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
)

// TestCheckGrammar exercises checkGrammar against the interaction
// grammar's non-contradiction rule (design language section 2).
func TestCheckGrammar(t *testing.T) {
	tests := []struct {
		name    string
		entry   ScreenEntry
		wantLen int
	}{
		{
			name: "conforming screen passes",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("j", "j/k", "navigate"), bind("q", "q", "quit")),
				SwitchState: StateDigitsSwitch,
			},
			wantLen: 0,
		},
		{
			name: "multi-key binding for one verb passes (CR3: verb identity, not whole-binding equality)",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("j", "j/k/up/down", "navigate")),
				SwitchState: StateDigitsSwitch,
			},
			wantLen: 0,
		},
		{
			name: "absence is not contradiction (a calendar screen leaves the mail capitals unbound)",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("t", "t", "today")),
				SwitchState: StateDigitsSwitch,
			},
			wantLen: 0,
		},
		{
			name: "contradicting binding fails",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("j", "j", "yank")), // j means navigate, not yank
				SwitchState: StateDigitsSwitch,
			},
			wantLen: 1,
		},
		{
			name: "disabled binding is not checked (CR4)",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "yank"), key.WithDisabled())),
				SwitchState: StateDigitsSwitch,
			},
			wantLen: 0,
		},
		{
			name: "modal confirm exemption",
			entry: ScreenEntry{
				Type:          reflect.TypeOf(struct{}{}),
				Keys:          flatKeyMap(bind("y", "y", "yes")), // y means yank elsewhere; legal here
				SwitchState:   StateModal,
				GrammarExempt: GrammarExemptModalConfirm,
			},
			wantLen: 0,
		},
		{
			name: "Catkin command-state exemption",
			entry: ScreenEntry{
				Type:          reflect.TypeOf(struct{}{}),
				Keys:          flatKeyMap(bind("j", "j", "char right")), // Catkin's h/j/k/l are motion, not navigate
				SwitchState:   StateDigitsSwitch,
				GrammarExempt: GrammarExemptCatkinCommand,
			},
			wantLen: 0,
		},
		{
			name: "an unrecognized exemption value is still checked (the list is exactly two)",
			entry: ScreenEntry{
				Type:          reflect.TypeOf(struct{}{}),
				Keys:          flatKeyMap(bind("j", "j", "yank")),
				SwitchState:   StateDigitsSwitch,
				GrammarExempt: GrammarExemption(99),
			},
			wantLen: 1,
		},
		{
			name: "printable-entry screens are out of scope",
			entry: ScreenEntry{
				Type:        reflect.TypeOf(struct{}{}),
				Keys:        flatKeyMap(bind("j", "j", "type the letter j")),
				SwitchState: StatePrintableEntry,
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkGrammar([]ScreenEntry{tt.entry}); len(got) != tt.wantLen {
				t.Errorf("checkGrammar() = %v, want %d violation(s)", got, tt.wantLen)
			}
		})
	}
}

// TestCheckGrammar_ComposedFromGrammarKeys proves M9's claim: a
// screen that composes its own bindings directly from GrammarKeys,
// bundling several verbs under one screen keymap, never contradicts
// itself.
func TestCheckGrammar_ComposedFromGrammarKeys(t *testing.T) {
	entry := ScreenEntry{
		Type: reflect.TypeOf(struct{}{}),
		Keys: flatKeyMap(
			GrammarKeys.Navigate, GrammarKeys.Open, GrammarKeys.Back,
			GrammarKeys.Archive, GrammarKeys.Delete, GrammarKeys.Quit,
		),
		SwitchState: StateDigitsSwitch,
	}

	if got := checkGrammar([]ScreenEntry{entry}); len(got) != 0 {
		t.Errorf("checkGrammar() on bindings composed from GrammarKeys = %v, want none", got)
	}
}

// TestCheckGrammar_LiveRegistry is CR1's live check: every currently
// registered screen's bindings conform to the grammar. It is
// vacuous until task 5 registers real screens, and live from then on.
func TestCheckGrammar_LiveRegistry(t *testing.T) {
	if got := checkGrammar(Registered()); len(got) != 0 {
		t.Errorf("checkGrammar(Registered()) = %v, want none", got)
	}
}

// grammarSectionTwoRows transcribes the design language's section 2
// tables (2026-07-27) as test data: the global verb table, the
// triage verb set (LT-2), the folder-jump capitals, calendar's `t`,
// and thread fold/unfold. It is M9's authority: GrammarKeys must
// carry exactly this key set and verb text.
var grammarSectionTwoRows = []struct {
	field key.Binding
	keys  []string
	desc  string
}{
	{GrammarKeys.Navigate, []string{"j", "k", "up", "down"}, "navigate"},
	{GrammarKeys.Page, []string{"space", "b", "pgup", "pgdown"}, "page"},
	{GrammarKeys.Extremes, []string{"home", "end", "G"}, "extremes"},
	{GrammarKeys.Open, []string{"enter"}, "open"},
	{GrammarKeys.Back, []string{"esc"}, "back"},
	{GrammarKeys.MessageStep, []string{"n", "p"}, "message step"},
	{GrammarKeys.Search, []string{"/"}, "search"},
	{GrammarKeys.Goto, []string{"g"}, "goto"},
	{GrammarKeys.NextUnread, []string{"tab"}, "next unread"},
	{GrammarKeys.Select, []string{"x"}, "select"},
	{GrammarKeys.SelectBy, []string{";"}, "select by"},
	{GrammarKeys.Undo, []string{"u"}, "undo"},
	{GrammarKeys.Help, []string{"?"}, "help"},
	{GrammarKeys.Quit, []string{"q"}, "quit"},
	{GrammarKeys.SurfaceSwitch, []string{"1", "2", "3", "4"}, "surface switch"},

	{GrammarKeys.Archive, []string{"a"}, "archive"},
	{GrammarKeys.Delete, []string{"d"}, "delete"},
	{GrammarKeys.FlagToggle, []string{"*"}, "flag toggle"},
	{GrammarKeys.ReadToggle, []string{"e"}, "read toggle"},
	{GrammarKeys.Move, []string{"s"}, "move"},
	{GrammarKeys.JunkToggle, []string{"!"}, "junk toggle"},
	{GrammarKeys.Reply, []string{"r"}, "reply"},
	{GrammarKeys.ReplyAll, []string{"R"}, "reply all"},
	{GrammarKeys.Forward, []string{"f"}, "forward"},
	{GrammarKeys.Compose, []string{"m"}, "compose"},
	{GrammarKeys.Yank, []string{"y"}, "yank"},
	{GrammarKeys.Attachments, []string{"v"}, "attachments"},

	{GrammarKeys.GotoInbox, []string{"I"}, "inbox"},
	{GrammarKeys.GotoDrafts, []string{"D"}, "drafts"},
	{GrammarKeys.GotoSent, []string{"S"}, "sent"},
	{GrammarKeys.GotoArchive, []string{"A"}, "archive folder"},
	{GrammarKeys.GotoJunk, []string{"J"}, "junk folder"},
	{GrammarKeys.GotoTrash, []string{"T"}, "trash"},

	{GrammarKeys.Today, []string{"t"}, "today"},

	{GrammarKeys.ThreadFold, []string{"h"}, "thread fold"},
	{GrammarKeys.ThreadUnfold, []string{"l"}, "thread unfold"},
}

func TestGrammarKeys_MatchesDesignLanguageSectionTwo(t *testing.T) {
	for _, row := range grammarSectionTwoRows {
		t.Run(row.desc, func(t *testing.T) {
			if got := row.field.Help().Desc; got != row.desc {
				t.Errorf("Desc = %q, want %q", got, row.desc)
			}
			if got := row.field.Keys(); !keySetEqual(got, row.keys) {
				t.Errorf("Keys() = %v, want %v", got, row.keys)
			}
		})
	}
	if len(grammarSectionTwoRows) != len(GrammarKeys.fields()) {
		t.Errorf("grammarSectionTwoRows has %d rows, GrammarKeys has %d fields; every field must be transcribed",
			len(grammarSectionTwoRows), len(GrammarKeys.fields()))
	}
}

func keySetEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]bool, len(got))
	for _, k := range got {
		seen[k] = true
	}
	for _, k := range want {
		if !seen[k] {
			return false
		}
	}
	return true
}

// TestGrammarExemptions_IsExactlyTwo is M8: the exemption list is
// closed at exactly the two named in design language section 2, the
// modal confirm and Catkin's command state.
func TestGrammarExemptions_IsExactlyTwo(t *testing.T) {
	if len(grammarExemptions) != 2 {
		t.Fatalf("grammarExemptions has %d entries, want exactly 2 (design language section 2)", len(grammarExemptions))
	}
	if !grammarExemptions[GrammarExemptModalConfirm] {
		t.Error("grammarExemptions is missing GrammarExemptModalConfirm")
	}
	if !grammarExemptions[GrammarExemptCatkinCommand] {
		t.Error("grammarExemptions is missing GrammarExemptCatkinCommand")
	}
}

// TestShortHelpWithinFullHelp exercises M11's registry-driven
// assertion: ShortHelp is always a subset of FullHelp.
func TestShortHelpWithinFullHelp(t *testing.T) {
	navigate := bind("j", "j/k", "navigate")
	openMsg := bind("enter", "enter", "open")

	tests := []struct {
		name    string
		entry   ScreenEntry
		wantLen int
	}{
		{
			name: "ShortHelp subset of FullHelp passes",
			entry: ScreenEntry{
				Type: reflect.TypeOf(struct{}{}),
				Keys: fakeKeyMap{
					short: []key.Binding{navigate},
					full:  [][]key.Binding{{navigate, openMsg}},
				},
			},
			wantLen: 0,
		},
		{
			name: "ShortHelp binding absent from FullHelp fails",
			entry: ScreenEntry{
				Type: reflect.TypeOf(struct{}{}),
				Keys: fakeKeyMap{
					short: []key.Binding{navigate},
					full:  [][]key.Binding{{openMsg}},
				},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortHelpWithinFullHelp([]ScreenEntry{tt.entry}); len(got) != tt.wantLen {
				t.Errorf("shortHelpWithinFullHelp() = %v, want %d violation(s)", got, tt.wantLen)
			}
		})
	}
}

// TestShortHelpWithinFullHelp_LiveRegistry is CR1's live check.
func TestShortHelpWithinFullHelp_LiveRegistry(t *testing.T) {
	if got := shortHelpWithinFullHelp(Registered()); len(got) != 0 {
		t.Errorf("shortHelpWithinFullHelp(Registered()) = %v, want none", got)
	}
}

func TestRegisterAndRegistered(t *testing.T) {
	resetRegistry(t)

	Register[valueScreen](ScreenEntry{Keys: flatKeyMap(bind("q", "q", "quit"))})

	got := Registered()
	want := reflect.TypeFor[valueScreen]()
	if len(got) != 1 || got[0].Type != want {
		t.Fatalf("Registered() = %v, want one entry for %v", got, want)
	}

	// Registered returns a copy: mutating it must not reach back into
	// the package's own registered slice.
	got[0] = ScreenEntry{}
	if Registered()[0].Type != want {
		t.Error("Registered() returned a slice aliasing the internal registry")
	}
}

// TestRegisterNormalizesPointerType is M13: a pointer-receiver
// screen registers under its named type, not the pointer type.
func TestRegisterNormalizesPointerType(t *testing.T) {
	resetRegistry(t)

	Register[*fakeScreen](ScreenEntry{})

	got := Registered()
	want := reflect.TypeFor[fakeScreen]()
	if len(got) != 1 || got[0].Type != want {
		t.Fatalf("Register[*fakeScreen] registered under %v, want %v", got, want)
	}
}

package contacts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

func newTestStyles() Styles {
	return NewStyles(theme.OneDark)
}

func newPersonForm(initial Contact) Form {
	return NewForm(newTestStyles(), initial, false, []string{"Local file", "test@example.com"})
}

func TestForm_KindToggleFlipsLayout(t *testing.T) {
	f := newPersonForm(Contact{})
	if f.kind != KindPerson {
		t.Fatalf("default kind = %v, want KindPerson", f.kind)
	}
	// Focus the kind toggle (index 0) and press Right.
	f.focusIdx = 0
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRight})
	if f.kind != KindOrg {
		t.Fatalf("after Right kind = %v, want KindOrg", f.kind)
	}
	// Press Left to flip back.
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if f.kind != KindPerson {
		t.Fatalf("after Left kind = %v, want KindPerson", f.kind)
	}
	// Space also toggles.
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if f.kind != KindOrg {
		t.Fatalf("after Space kind = %v, want KindOrg", f.kind)
	}
}

func TestForm_AddEmailAppendsRow(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: "a@b.c"}}})
	before := len(f.emails)
	f.focusIdx = f.addEmailIdx()
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(f.emails) != before+1 {
		t.Fatalf("emails after add = %d, want %d", len(f.emails), before+1)
	}
}

func TestForm_RemoveEmailRow(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{
		{Address: "a@b.c"}, {Address: "d@e.f"},
	}})
	// Focus the − button on row 1.
	f.focusIdx = f.emailRowMinusIdx(1)
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(f.emails) != 1 {
		t.Fatalf("emails after remove = %d, want 1", len(f.emails))
	}
	if f.emails[0].input.Value() != "a@b.c" {
		t.Fatalf("survivor = %q, want a@b.c", f.emails[0].input.Value())
	}
}

func TestForm_RemoveDisabledOnSoleEmail(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: "a@b.c"}}})
	f.focusIdx = f.emailRowMinusIdx(0)
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(f.emails) != 1 {
		t.Fatalf("sole email row was removed; got %d rows", len(f.emails))
	}
}

func TestForm_PromotePrimary(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{
		{Address: "primary@x.com"},
		{Address: "second@x.com"},
	}})
	// Press the ★ on row 1 to promote.
	f.focusIdx = f.emailRowStarIdx(1)
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := f.emails[0].input.Value(); got != "second@x.com" {
		t.Fatalf("after promote primary = %q, want second@x.com", got)
	}
	if got := f.emails[1].input.Value(); got != "primary@x.com" {
		t.Fatalf("after promote secondary = %q, want primary@x.com", got)
	}
}

func TestForm_StarOnRowZeroNoOp(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{
		{Address: "a@b.c"},
		{Address: "d@e.f"},
	}})
	f.focusIdx = f.emailRowStarIdx(0)
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := f.emails[0].input.Value(); got != "a@b.c" {
		t.Fatalf("row 0 star reorder mutated; row 0 = %q", got)
	}
}

func TestForm_ValidatePersonRequiresName(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: "a@b.c"}}})
	if err := f.Validate(); err == nil {
		t.Fatal("validate accepted person with empty First and Last")
	}
	f.first.SetValue("Alice")
	if err := f.Validate(); err != nil {
		t.Fatalf("validate rejected person with First set: %v", err)
	}
}

func TestForm_ValidateBusinessRequiresName(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: "a@b.c"}}})
	f.kind = KindOrg
	if err := f.Validate(); err == nil {
		t.Fatal("validate accepted business with empty Name")
	}
	f.bizName.SetValue("ACME")
	if err := f.Validate(); err != nil {
		t.Fatalf("validate rejected business with Name set: %v", err)
	}
}

func TestForm_ValidateRequiresAtLeastOneEmail(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: ""}}})
	f.first.SetValue("Alice")
	if err := f.Validate(); err == nil {
		t.Fatal("validate accepted form with empty email")
	}
}

func TestForm_ValidateRejectsMalformedEmail(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: "not-an-email"}}})
	f.first.SetValue("Alice")
	if err := f.Validate(); err == nil {
		t.Fatal("validate accepted malformed email")
	}
}

func TestForm_CtrlSEmitsSaveOnSuccess(t *testing.T) {
	f := newPersonForm(Contact{})
	f.first.SetValue("Alice")
	f.emails[0].input.SetValue("alice@example.com")
	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("Ctrl+S returned nil cmd; want ContactSaveMsg")
	}
	msg := cmd()
	save, ok := msg.(ContactSaveMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want ContactSaveMsg", msg)
	}
	if save.Contact.Given != "Alice" {
		t.Fatalf("save.Contact.Given = %q, want Alice", save.Contact.Given)
	}
	if save.SaveTo == "" {
		t.Fatal("save.SaveTo empty")
	}
}

func TestForm_CtrlSStaysOpenOnValidationError(t *testing.T) {
	f := newPersonForm(Contact{})
	f.emails[0].input.SetValue("alice@example.com")
	next, cmd := f.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		// Some Cmds may fire (e.g., refocus). Make sure no save msg.
		if msg := cmd(); msg != nil {
			if _, ok := msg.(ContactSaveMsg); ok {
				t.Fatal("validation failure produced ContactSaveMsg")
			}
		}
	}
	if next.err == "" {
		t.Fatal("validation failure did not set f.err")
	}
}

func TestForm_EscEmitsCancelDirtyTracking(t *testing.T) {
	initial := Contact{Emails: []Email{{Address: "a@b.c"}}, Given: "Alice"}
	f := newPersonForm(initial)
	// Pristine: cancel reports Dirty=false.
	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc produced nil cmd")
	}
	c, ok := cmd().(ContactCancelMsg)
	if !ok {
		t.Fatalf("Esc produced %T, want ContactCancelMsg", cmd())
	}
	if c.Dirty {
		t.Fatal("pristine form reported Dirty=true")
	}

	// Modify a field so dirty becomes true.
	f.first.SetValue("Bob")
	_, cmd = f.Update(tea.KeyMsg{Type: tea.KeyEsc})
	c2 := cmd().(ContactCancelMsg)
	if !c2.Dirty {
		t.Fatal("modified form reported Dirty=false")
	}
}

func TestForm_TabCyclesFocus(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: "a@b.c"}}})
	start := f.focusIdx
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	if f.focusIdx == start {
		t.Fatal("Tab did not advance focus")
	}
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if f.focusIdx != start {
		t.Fatalf("ShiftTab landed at %d, want %d", f.focusIdx, start)
	}
}

func TestForm_ViewRendersHeaderAndFields(t *testing.T) {
	f := newPersonForm(Contact{Emails: []Email{{Address: "a@b.c"}}})
	f = f.SetSize(80, 24)
	v := f.View()
	if !strings.Contains(v, "Person") {
		t.Errorf("view missing Person label\n%s", v)
	}
	if !strings.Contains(v, "Business") {
		t.Errorf("view missing Business label\n%s", v)
	}
	if !strings.Contains(v, "Save to") {
		t.Errorf("view missing Save to row\n%s", v)
	}
}

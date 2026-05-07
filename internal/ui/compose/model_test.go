package compose

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gomail "github.com/emersion/go-message/mail"
	mailcompose "github.com/glw907/poplar/internal/compose"
)

type fakeCache struct {
	createCalls int
	updateCalls int
	lastPayload []byte
}

func (f *fakeCache) CreateDraft(_ context.Context, _ string, payload []byte) error {
	f.createCalls++
	f.lastPayload = payload
	return nil
}

func (f *fakeCache) UpdateDraft(_ context.Context, _ string, payload []byte) error {
	f.updateCalls++
	f.lastPayload = payload
	return nil
}

func (f *fakeCache) LoadDraft(_ context.Context, _ string) ([]byte, error) {
	return f.lastPayload, nil
}

func newTestModel(t *testing.T) *Model {
	t.Helper()
	c := New(Styles{ErrorBanner: lipgloss.NewStyle()}, "geoff@907.life")
	c.SetSize(80, 24)
	return c
}

func keyMsgFromString(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func sendKey(c *Model, k string) *Model {
	next, _ := c.Update(keyMsgFromString(k))
	return next
}

func TestModel_View_HonorsAssignedWidth(t *testing.T) {
	c := newTestModel(t)
	c.SetSize(60, 20)
	for i, line := range strings.Split(c.View(), "\n") {
		if w := lipgloss.Width(line); w != 60 {
			t.Fatalf("line %d width = %d, want 60: %q", i, w, line)
		}
	}
}

func TestModel_View_HasHeaderRows(t *testing.T) {
	c := newTestModel(t)
	v := c.View()
	for _, want := range []string{"From:", "To:", "Cc:", "Bcc:", "Subject:"} {
		if !strings.Contains(v, want) {
			t.Fatalf("View missing %q\n%s", want, v)
		}
	}
}

func TestModel_TabCyclesFields(t *testing.T) {
	c := newTestModel(t)
	want := []int{focusCc, focusBcc, focusSubject, focusBody, focusTo}
	for i, w := range want {
		c = sendKey(c, "tab")
		if c.focus != w {
			t.Fatalf("step %d: focus = %d, want %d", i, c.focus, w)
		}
	}
}

func TestModel_ShiftTabCyclesBackward(t *testing.T) {
	c := newTestModel(t)
	c = sendKey(c, "shift+tab")
	if c.focus != focusBody {
		t.Fatalf("Shift+Tab from To should wrap to Body, got %d", c.focus)
	}
}

func TestModel_EscFromBodyReturnsToSubject(t *testing.T) {
	c := newTestModel(t)
	c.focus = focusBody
	c.editor.Focus()
	c.to.Blur()
	c = sendKey(c, "esc")
	if c.focus != focusSubject {
		t.Fatalf("Esc from Body should focus Subject, got %d", c.focus)
	}
}

func TestModel_EscFromHeaderReturnsToBody(t *testing.T) {
	c := newTestModel(t)
	c.focus = focusTo
	c = sendKey(c, "esc")
	if c.focus != focusBody {
		t.Fatalf("Esc from header should focus Body, got %d", c.focus)
	}
}

func gomailAddress(addr string) gomail.Address {
	return gomail.Address{Address: addr}
}

func TestModel_DraftReflectsInputs(t *testing.T) {
	c := newTestModel(t)
	c.to.SetValue("alice@example.com, bob@example.com")
	c.cc.SetValue("c@example.com")
	c.subject.SetValue("hi")
	c.editor.SetValue("hello world")

	d, err := c.Draft()
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if len(d.To) != 2 || d.To[0].Address != "alice@example.com" {
		t.Fatalf("To not parsed: %+v", d.To)
	}
	if len(d.Cc) != 1 || d.Cc[0].Address != "c@example.com" {
		t.Fatalf("Cc not parsed: %+v", d.Cc)
	}
	if d.Subject != "hi" || d.Body != "hello world" {
		t.Fatalf("subject/body wrong: %q %q", d.Subject, d.Body)
	}
	if d.From.Address != "geoff@907.life" {
		t.Fatalf("From wrong: %+v", d.From)
	}
}

func TestModel_DraftBadAddressFails(t *testing.T) {
	c := newTestModel(t)
	c.to.SetValue("not an address")
	if _, err := c.Draft(); err == nil {
		t.Fatalf("want parse error on bad address, got nil")
	}
}

func TestModel_IsDirty(t *testing.T) {
	c := newTestModel(t)
	if c.IsDirty() {
		t.Fatalf("fresh compose should not be dirty")
	}
	c.editor.SetValue("hi")
	if !c.IsDirty() {
		t.Fatalf("body content should mark dirty")
	}
}

func TestModel_Seed(t *testing.T) {
	c := newTestModel(t)
	d := mailcompose.Draft{
		Subject: "Re: hi",
		Body:    "> original\n\n",
	}
	d.To = append(d.To, gomailAddress("alice@example.com"))
	c.Seed(d)
	if c.subject.Value() != "Re: hi" {
		t.Fatalf("subject not seeded: %q", c.subject.Value())
	}
	if c.editor.Value() != "> original\n\n" {
		t.Fatalf("body not seeded: %q", c.editor.Value())
	}
	if c.to.Value() != "alice@example.com" {
		t.Fatalf("To not seeded: %q", c.to.Value())
	}
}

func TestModel_CtrlXEmitsSendMsg(t *testing.T) {
	c := newTestModel(t)
	c.to.SetValue("alice@example.com")
	c.subject.SetValue("hi")
	c.editor.SetValue("body")

	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd == nil {
		t.Fatal("Ctrl+X should return a Cmd that emits SendMsg")
	}
	msg := cmd()
	send, ok := msg.(SendMsg)
	if !ok {
		t.Fatalf("want SendMsg, got %T", msg)
	}
	if send.Draft.Subject != "hi" {
		t.Fatalf("send carries wrong draft: %+v", send.Draft)
	}
}

func TestModel_CtrlCEmitsCancelMsg(t *testing.T) {
	c := newTestModel(t)
	c.editor.SetValue("dirty")

	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C should return a Cmd")
	}
	msg := cmd()
	cancel, ok := msg.(CancelMsg)
	if !ok {
		t.Fatalf("want CancelMsg, got %T", msg)
	}
	if !cancel.Dirty {
		t.Fatalf("dirty draft should set Dirty=true")
	}
}

func TestModel_CtrlXBadAddressInlinesError(t *testing.T) {
	c := newTestModel(t)
	c.to.SetValue("not an address")
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd != nil {
		t.Fatalf("Ctrl+X with bad address should not emit send")
	}
	if c.err == "" {
		t.Fatalf("inline err row should be set")
	}
}

func TestModel_AutosaveDebounce(t *testing.T) {
	cache := &fakeCache{}
	c := newTestModel(t)
	c.SetCache(cache)

	// Simulate a key edit, then wind lastEditAt back so the debounce
	// condition is satisfied without sleeping.
	c.localDirty = true
	c.lastEditAt = time.Now().Add(-2 * autosaveDelay)

	// Autosave tick fires. Debounce condition is met.
	next, _ := c.Update(autosaveTickMsg{})
	if next.localDirty {
		t.Fatalf("localDirty should be cleared after autosave tick")
	}

	// Drive the update Cmd directly. Same logic as the Cmd closure.
	msg := next.updateDraftCmd()()
	persisted, ok := msg.(DraftPersistedMsg)
	if !ok {
		t.Fatalf("updateDraftCmd returned %T, want DraftPersistedMsg", msg)
	}
	if persisted.DraftID != next.draftID {
		t.Fatalf("DraftPersistedMsg.DraftID = %q, want %q", persisted.DraftID, next.draftID)
	}
	if cache.updateCalls != 1 {
		t.Fatalf("cache.updateCalls = %d, want 1", cache.updateCalls)
	}
	if len(cache.lastPayload) == 0 {
		t.Fatalf("cache.lastPayload is empty; no bytes written")
	}
}

func TestModel_AutosaveNoopBeforeDebounce(t *testing.T) {
	cache := &fakeCache{}
	c := newTestModel(t)
	c.SetCache(cache)

	// Dirty but lastEditAt is recent. Debounce not satisfied.
	c.localDirty = true
	c.lastEditAt = time.Now()

	next, _ := c.Update(autosaveTickMsg{})
	if !next.localDirty {
		t.Fatalf("localDirty should not be cleared when debounce window hasn't elapsed")
	}
	if cache.updateCalls != 0 {
		t.Fatalf("cache.updateCalls = %d, want 0 before debounce window", cache.updateCalls)
	}
}

func TestModel_DraftIDIsSet(t *testing.T) {
	c := newTestModel(t)
	if c.DraftID() == "" {
		t.Fatalf("New() should assign a non-empty draftID")
	}
}

func TestModel_OpenPreservesID(t *testing.T) {
	styles := Styles{ErrorBanner: lipgloss.NewStyle()}
	d := mailcompose.Draft{Subject: "saved", Body: "body text"}
	c := Open(styles, "geoff@907.life", "fixed-id", d)
	if c.DraftID() != "fixed-id" {
		t.Fatalf("Open draftID = %q, want fixed-id", c.DraftID())
	}
	if c.subject.Value() != "saved" {
		t.Fatalf("Open did not seed draft: subject = %q", c.subject.Value())
	}
	if c.localDirty || c.pushDirty {
		t.Fatalf("Open should not mark dirty: localDirty=%v pushDirty=%v", c.localDirty, c.pushDirty)
	}
}

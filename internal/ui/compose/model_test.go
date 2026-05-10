package compose

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	gomail "github.com/emersion/go-message/mail"
	mailcompose "github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/contacts"
)

func stripANSI(s string) string { return ansi.Strip(s) }

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
	c := New(theme.OneDark, Styles{ErrorBanner: lipgloss.NewStyle()}, "geoff@907.life", nil)
	c.SetSize(80, 24)
	return c
}

func keyMsgFromString(s string) tea.KeyPressMsg {
	switch s {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Mod: tea.ModShift, Code: tea.KeyTab}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	rs := []rune(s)
	if len(rs) == 1 {
		return tea.KeyPressMsg{Code: rs[0], Text: s}
	}
	// multi-rune: callers should use a loop; this handles the common single-char case
	return tea.KeyPressMsg{Code: rs[0], Text: s}
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
	want := []int{focusCc, focusBcc, focusSubject, focusBody, focusFrom, focusTo}
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
	if c.focus != focusFrom {
		t.Fatalf("Shift+Tab from To should wrap to From, got %d", c.focus)
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

	_, cmd := c.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 'x'})
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

	_, cmd := c.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 'c'})
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
	_, cmd := c.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 'x'})
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

func TestModel_AcceptsSuggestionWithTab(t *testing.T) {
	suggest := staticSuggest([]contacts.Suggestion{
		{Name: "Alice Adams", Email: "alice@example.com"},
	})
	c := New(theme.OneDark, Styles{}, "geoff@907.life", suggest)
	c.SetSize(80, 24)
	c, _ = c.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	c, _ = c.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if c.suggest.Empty() {
		t.Fatalf("dropdown should populate after typing 'al'")
	}
	c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !c.suggest.Empty() {
		t.Fatalf("Tab should clear the dropdown after accept")
	}
	if got := c.to.Value(); got != "Alice Adams <alice@example.com>, " {
		t.Fatalf("To value after accept = %q, want %q", got, "Alice Adams <alice@example.com>, ")
	}
	if c.focus != focusTo {
		t.Fatalf("focus should stay on To after accept; got %d", c.focus)
	}
}

func TestModel_TabAdvancesFocusWhenDropdownEmpty(t *testing.T) {
	c := newTestModel(t)
	c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if c.focus != focusCc {
		t.Fatalf("Tab should advance focus when dropdown is empty; focus=%d", c.focus)
	}
}

func TestModel_DropdownPrefixUsesTrailingFragment(t *testing.T) {
	suggest := staticSuggest([]contacts.Suggestion{
		{Name: "Bob", Email: "bob@example.com"},
	})
	c := New(theme.OneDark, Styles{}, "geoff@907.life", suggest)
	c.SetSize(80, 24)
	c.to.SetValue("Alice <alice@example.com>, ")
	c, _ = c.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	c, _ = c.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if c.suggest.Empty() {
		t.Fatalf("dropdown should populate from trailing 'bo' fragment")
	}
	c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	want := "Alice <alice@example.com>, Bob <bob@example.com>, "
	if got := c.to.Value(); got != want {
		t.Fatalf("To value after accept = %q, want %q", got, want)
	}
}

func TestModel_View_WidthHonoredWhileDropdownOpen(t *testing.T) {
	suggest := staticSuggest([]contacts.Suggestion{
		{Name: "Alice Adams", Email: "alice@example.com", Org: "Acme"},
		{Name: "Alex Brown", Email: "alex@example.com"},
	})
	c := New(theme.OneDark, Styles{}, "geoff@907.life", suggest)
	c.SetSize(80, 24)
	c, _ = c.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	c, _ = c.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if c.suggest.Empty() {
		t.Fatalf("precondition: dropdown should populate")
	}
	v := c.View()
	for i, line := range strings.Split(v, "\n") {
		if w := lipgloss.Width(line); w != 80 {
			t.Fatalf("line %d width = %d, want 80: %q", i, w, line)
		}
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
	c := Open(theme.OneDark, styles, "geoff@907.life", "fixed-id", d, nil)
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

func TestComposeFocusOrderIncludesFrom(t *testing.T) {
	c := newTestModel(t)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "G", Email: "g@x", Signatures: []mailcompose.Signature{{Name: "s", Text: "-- \nG"}}},
	})
	c.SetSize(80, 24)

	// Tab from initial focus (To) walks To→Cc→Bcc→Subject→Body→From→To.
	got := []int{c.Focus()}
	for i := 0; i < 6; i++ {
		c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		got = append(got, c.Focus())
	}
	want := []int{focusTo, focusCc, focusBcc, focusSubject, focusBody, focusFrom, focusTo}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("focus order = %v, want %v", got, want)
	}
}

func TestComposeIdentityCycle(t *testing.T) {
	c := newTestModel(t)
	c.SetSize(80, 24)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "A", Email: "a@x", Signatures: []mailcompose.Signature{{Name: "s1", Text: "-- \n1"}}},
		{Name: "B", Email: "b@x"}, // no sigs
		{Name: "C", Email: "c@x", Signatures: []mailcompose.Signature{{Name: "s3", Text: "-- \n3"}}},
	})
	// Tab to focusFrom (last in cycle).
	for c.Focus() != focusFrom {
		c.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}

	// Space → next identity.
	c.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if c.Identity() != 1 {
		t.Errorf("after Space: identity = %d, want 1", c.Identity())
	}
	// B has no sigs → Signature must reset to -1.
	if c.Signature() != -1 {
		t.Errorf("after Space onto B: signature = %d, want -1", c.Signature())
	}

	// Space again → C; Signature resets to 0 (C has sigs).
	c.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if c.Identity() != 2 || c.Signature() != 0 {
		t.Errorf("after second Space: identity = %d, signature = %d, want 2, 0", c.Identity(), c.Signature())
	}

	// Wrap forward.
	c.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if c.Identity() != 0 {
		t.Errorf("after wrap: identity = %d, want 0", c.Identity())
	}

	// Left arrow → previous (wrap to 2).
	c.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if c.Identity() != 2 {
		t.Errorf("after Left wrap: identity = %d, want 2", c.Identity())
	}
}

func TestComposeSignatureCycle(t *testing.T) {
	c := newTestModel(t)
	c.SetSize(80, 24)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "A", Email: "a@x", Signatures: []mailcompose.Signature{
			{Name: "s1", Text: "-- \n1"},
			{Name: "s2", Text: "-- \n2"},
		}},
	})

	// Navigate to Body (initial focus is To).
	for c.Focus() != focusBody {
		c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}

	// Ctrl+G in Body: 0 → 1 → -1 → 0.
	steps := []struct{ want int }{{1}, {-1}, {0}}
	for _, st := range steps {
		c, _ = c.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 'g'})
		if c.Signature() != st.want {
			t.Errorf("after Ctrl+G: signature = %d, want %d", c.Signature(), st.want)
		}
	}

	// Tab to focusFrom; bare 'g' cycles.
	for c.Focus() != focusFrom {
		c, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	c, _ = c.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if c.Signature() != 1 {
		t.Errorf("after 'g' in focusFrom: signature = %d, want 1", c.Signature())
	}

	// Inert when identity has zero sigs.
	c.SetIdentities([]mailcompose.Identity{{Name: "B", Email: "b@x"}})
	c, _ = c.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 'g'})
	if c.Signature() != -1 {
		t.Errorf("expected signature stays -1, got %d", c.Signature())
	}
}

func newComposeForTest(t *testing.T) *Model {
	t.Helper()
	return newTestModel(t)
}

func TestCompose_TabSkipsAttachWhenEmpty(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	c.setFocus(focusSubject)
	c.advanceFocus(+1)
	if c.focus != focusBody {
		t.Errorf("Tab from Subject with no attachments: focus = %d, want focusBody=%d", c.focus, focusBody)
	}
}

func TestCompose_AttachAcceptedAppends(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf", "/tmp/b.pdf"}})
	d := c.CurrentDraft()
	if len(d.Attachments) != 2 {
		t.Fatalf("attachments len = %d, want 2", len(d.Attachments))
	}
	if c.attachLastDir != "/tmp" {
		t.Errorf("attachLastDir = %q, want /tmp", c.attachLastDir)
	}
	if !c.localDirty {
		t.Error("expected localDirty after AttachAcceptedMsg")
	}
}

func TestCompose_AttachAcceptedDedupes(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf"}})
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf", "/tmp/b.pdf"}})
	d := c.CurrentDraft()
	if len(d.Attachments) != 2 {
		t.Errorf("dedupe failed: got %v", d.Attachments)
	}
}

func TestCompose_FocusAttachRemoveCollapsesEmpty(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a.pdf"}})
	c.setFocus(focusAttach)
	_, _ = c.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if d := c.CurrentDraft(); len(d.Attachments) != 0 {
		t.Fatalf("attachments not removed: %v", d.Attachments)
	}
	if c.focus != focusSubject {
		t.Errorf("focus did not collapse to Subject: %d", c.focus)
	}
}

func TestCompose_FocusAttachArrowsAndDeleteMid(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/a", "/tmp/b", "/tmp/c"}})
	c.setFocus(focusAttach)
	_, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	_, _ = c.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	d := c.CurrentDraft()
	if len(d.Attachments) != 2 || d.Attachments[1] != "/tmp/c" {
		t.Errorf("after middle delete: %v", d.Attachments)
	}
}

func TestCompose_AttachRowHiddenWhenEmpty(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	out := c.View()
	if strings.Contains(out, "Attach:") {
		t.Errorf("attach row should not render when no attachments")
	}
}

func TestCompose_AttachRowVisibleWithAttachments(t *testing.T) {
	c := newComposeForTest(t)
	c.SetSize(80, 24)
	_, _ = c.Update(AttachAcceptedMsg{Paths: []string{"/tmp/notes.pdf"}})
	out := stripANSI(c.View())
	if !strings.Contains(out, "Attach:") {
		t.Errorf("attach row should render:\n%s", out)
	}
	if !strings.Contains(out, "notes.pdf") {
		t.Errorf("chip should show basename:\n%s", out)
	}
}

func TestComposeFromRowRendersChip(t *testing.T) {
	c := newTestModel(t)
	c.SetSize(80, 24)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "Geoff Wright", Email: "geoff@907.life",
			Signatures: []mailcompose.Signature{
				{Name: "default", Text: "-- \nG"},
				{Name: "casual", Text: "-- \ng"},
			}},
	})

	view := stripANSI(c.View())

	if !strings.Contains(view, "Geoff Wright <geoff@907.life>") {
		t.Errorf("From row missing identity:\n%s", view)
	}
	if !strings.Contains(view, "· sig: default") {
		t.Errorf("From row missing chip:\n%s", view)
	}

	// Zero-signature identity → no chip.
	c.SetIdentities([]mailcompose.Identity{
		{Name: "G", Email: "g@x"},
	})
	view = stripANSI(c.View())
	if strings.Contains(view, "· sig:") {
		t.Errorf("expected no chip for zero-sig identity, got:\n%s", view)
	}

	// Signature == -1 → "no sig" chip when sigs exist.
	c.SetIdentities([]mailcompose.Identity{
		{Name: "G", Email: "g@x", Signatures: []mailcompose.Signature{{Name: "x", Text: "-- \nx"}}},
	})
	c.SetSignature(-1)
	view = stripANSI(c.View())
	if !strings.Contains(view, "· no sig") {
		t.Errorf("expected '· no sig' chip, got:\n%s", view)
	}
}

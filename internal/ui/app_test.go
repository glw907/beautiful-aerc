package ui

import (
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/theme"
)

func testDeps(t *testing.T) Deps {
	t.Helper()
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	return Deps{
		Store:   reads,
		Theme:   theme.New(true, theme.ProfileTrueColor),
		Profile: theme.ProfileTrueColor,
		Account: "geoff@907.life",
	}
}

func digitKey(d string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(d[0]), Text: d}
}

func mustApp(t *testing.T, model tea.Model) App {
	t.Helper()
	app, ok := model.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", model)
	}
	return app
}

// TestMatchDigit proves matchDigit's key.Matches dispatch: each digit
// names its own Surface at StateDigitsSwitch, and the whole set
// disables (SetEnabled false, so key.Matches itself reports no match)
// at any other state.
func TestMatchDigit(t *testing.T) {
	tests := []struct {
		digit string
		want  Surface
	}{
		{"1", SurfaceMail},
		{"2", SurfaceCalendar},
		{"3", SurfaceContacts},
		{"4", SurfaceConfig},
	}
	for _, tt := range tests {
		t.Run(tt.digit, func(t *testing.T) {
			got, ok := matchDigit(digitKey(tt.digit), StateDigitsSwitch)
			if !ok || got != tt.want {
				t.Errorf("matchDigit(%q, StateDigitsSwitch) = (%v, %v), want (%v, true)", tt.digit, got, ok, tt.want)
			}
		})
	}

	for _, state := range []StateClass{StateModal, StatePrintableEntry} {
		if _, ok := matchDigit(digitKey("1"), state); ok {
			t.Errorf("matchDigit(%q, %v) matched, want disabled", "1", state)
		}
	}

	if _, ok := matchDigit(tea.KeyPressMsg{Code: 'q', Text: "q"}, StateDigitsSwitch); ok {
		t.Error("matchDigit on a non-digit key matched, want no match")
	}
}

// TestApp_DigitSurfaceSwitchRoundTrip is UX-4's round-trip
// acceptance criterion: 1->3->1 lands back on mail with its state
// (here, its already-loaded store counts) intact, byte-for-byte in
// the rendered view.
func TestApp_DigitSurfaceSwitchRoundTrip(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(mailStatsMsg{stats: store.MailStats{Messages: 42, Mailboxes: 3}})))

	if got := app.activeSurface(); got != SurfaceMail {
		t.Fatalf("initial active surface = %v, want SurfaceMail", got)
	}
	before := app.View().Content

	app = mustApp(t, first(app.Update(digitKey("3"))))
	if got := app.activeSurface(); got != SurfaceContacts {
		t.Fatalf("after '3' active surface = %v, want SurfaceContacts", got)
	}

	app = mustApp(t, first(app.Update(digitKey("1"))))
	if got := app.activeSurface(); got != SurfaceMail {
		t.Fatalf("after '1' active surface = %v, want SurfaceMail", got)
	}

	after := app.View().Content
	if before != after {
		t.Errorf("mail placeholder view changed across a 1->3->1 round trip:\nbefore: %q\nafter:  %q", before, after)
	}
}

// TestApp_ModalEatsDigits is UX-4's other acceptance criterion: a
// screen on the stack whose SwitchState is StateModal absorbs a digit
// as its own answer/no-op instead of switching surfaces.
func TestApp_ModalEatsDigits(t *testing.T) {
	// fakeModal implements Screen (it is pushed onto App's own stack
	// below), so the screenregistry analyzer requires a Register call
	// naming it somewhere in this package; resetRegistry keeps that
	// registration from leaking into another test's Registered() view.
	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})

	app := NewApp(testDeps(t))
	app.stack = append(app.stack, &fakeModal{})

	if got := app.activeSurface(); got != SurfaceMail {
		t.Fatalf("initial active surface = %v, want SurfaceMail", got)
	}

	app = mustApp(t, first(app.Update(digitKey("3"))))

	if got := app.activeSurface(); got != SurfaceMail {
		t.Errorf("active surface after a digit with a modal on the stack = %v, want unchanged SurfaceMail", got)
	}
	if len(app.stack) != 1 {
		t.Fatalf("stack length = %d, want 1 (the modal stays)", len(app.stack))
	}
	modal, ok := app.stack[0].(*fakeModal)
	if !ok {
		t.Fatalf("stack[0] is %T, want *fakeModal", app.stack[0])
	}
	if modal.seenDigit != "3" {
		t.Errorf("fakeModal.seenDigit = %q, want %q (the digit reached the modal's own Update)", modal.seenDigit, "3")
	}
}

// fakeModal is a StateModal Screen fixture: it records whichever key
// reaches its Update, standing in for a modal that answers or no-ops
// on a digit rather than treating it as a surface switch.
type fakeModal struct {
	seenDigit string
}

func (m *fakeModal) Init() tea.Cmd { return nil }

func (m *fakeModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		m.seenDigit = k.String()
	}
	return m, nil
}

func (m *fakeModal) View() tea.View { return tea.NewView("modal") }

func (m *fakeModal) Entry() ScreenEntry {
	return ScreenEntry{SwitchState: StateModal}
}

// TestApp_EscPopsStack proves Esc pops the topmost stack entry and
// no-ops at a surface root (an empty stack), this pass.
func TestApp_EscPopsStack(t *testing.T) {
	app := NewApp(testDeps(t))

	app = mustApp(t, first(app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})))
	if len(app.stack) != 0 {
		t.Errorf("Esc at a surface root changed the stack: %v", app.stack)
	}

	app.stack = append(app.stack, &fakeModal{})
	app = mustApp(t, first(app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})))
	if len(app.stack) != 0 {
		t.Errorf("Esc with one entry on the stack left %d, want 0", len(app.stack))
	}
}

// TestApp_QuitAtSurfaceRoot proves q quits only when the stack is
// empty.
func TestApp_QuitAtSurfaceRoot(t *testing.T) {
	app := NewApp(testDeps(t))

	_, cmd := app.Update(digitKey("q"))
	if cmd == nil {
		t.Fatal("q at a surface root returned a nil Cmd, want tea.Quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("q at a surface root's Cmd yielded %#v, want tea.QuitMsg", msg)
	}

	app.stack = append(app.stack, &fakeModal{})
	_, cmd = app.Update(digitKey("q"))
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Error("q with a modal on the stack quit; want it forwarded to the modal instead")
		}
	}
}

// TestApp_ResizeRoundTripPreservesState proves a 100x30 -> 80x24 ->
// 100x30 resize round trip leaves the mail placeholder's loaded state
// intact (composition rule 4's mechanism).
func TestApp_ResizeRoundTripPreservesState(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(mailStatsMsg{stats: store.MailStats{Messages: 7, Mailboxes: 2}})))

	if !app.mail.loaded {
		t.Fatal("mail placeholder did not load before the resize round trip")
	}

	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	if !app.mail.loaded || app.mail.stats.Messages != 7 || app.mail.stats.Mailboxes != 2 {
		t.Errorf("mail placeholder state after the resize round trip = %+v, want loaded with 7 messages, 2 mailboxes", app.mail)
	}
}

// TestApp_BackgroundColorTimeoutLogsOneDebugLine is the carried
// ruling from task 2: the root model handles BackgroundColorTimeoutMsg
// by emitting exactly one debug-level slog line and otherwise
// changing nothing (DefaultDark already governs every frame rendered
// so far).
func TestApp_BackgroundColorTimeoutLogsOneDebugLine(t *testing.T) {
	buf := captureDebugLog(t)
	app := NewApp(testDeps(t))

	updated, cmd := app.Update(BackgroundColorTimeoutMsg{})
	if cmd != nil {
		t.Errorf("BackgroundColorTimeoutMsg returned a non-nil Cmd, want nil")
	}
	if got := mustApp(t, updated).theme; got != app.theme {
		t.Errorf("BackgroundColorTimeoutMsg changed the theme; want it untouched")
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want exactly 1:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "background-color query unanswered; staying on the default dark theme") {
		t.Errorf("log line = %q, want it to state the unanswered-query outcome", lines[0])
	}
}

// TestApp_BackgroundColorAnsweredThenTimeoutLogsNothing is F3: once
// tea.BackgroundColorMsg has answered, a BackgroundColorTimeoutMsg
// that arrives after it (the bounded wait firing after the answer
// already landed) must not log the unanswered-query line, which
// would misstate what happened.
func TestApp_BackgroundColorAnsweredThenTimeoutLogsNothing(t *testing.T) {
	buf := captureDebugLog(t)
	app := NewApp(testDeps(t))

	app = mustApp(t, first(app.Update(tea.BackgroundColorMsg{Color: whiteColor{}})))
	_, cmd := app.Update(BackgroundColorTimeoutMsg{})
	if cmd != nil {
		t.Errorf("BackgroundColorTimeoutMsg after an answer returned a non-nil Cmd, want nil")
	}

	if got := buf.String(); got != "" {
		t.Errorf("log after an answered query then its timeout = %q, want no lines", got)
	}
}

// TestApp_OrdinaryKeyReachesActiveScreen is F4: a key that is neither
// Esc, a legal surface digit, nor q at a surface root now reaches the
// active screen's own Update (through updateActive) instead of being
// silently dropped, which is what handleKey's fallthrough did before
// this fix. Verified as a delegation contract, since no placeholder
// yet reacts to a key differently from any other unhandled message:
// handleKey's fallthrough for an ordinary key must produce exactly
// what calling updateActive directly produces.
func TestApp_OrdinaryKeyReachesActiveScreen(t *testing.T) {
	app := NewApp(testDeps(t))
	ordinary := tea.KeyPressMsg{Code: 'j', Text: "j"}

	viaKey, cmdKey := app.handleKey(ordinary)
	viaActive, cmdActive := app.updateActive(ordinary)

	gotKey, ok := viaKey.(App)
	if !ok {
		t.Fatalf("handleKey returned %T, want App", viaKey)
	}
	gotActive, ok := viaActive.(App)
	if !ok {
		t.Fatalf("updateActive returned %T, want App", viaActive)
	}
	if !reflect.DeepEqual(gotKey, gotActive) {
		t.Errorf("handleKey's fallthrough did not delegate to updateActive:\nhandleKey:     %+v\nupdateActive:  %+v", gotKey, gotActive)
	}
	if (cmdKey == nil) != (cmdActive == nil) {
		t.Errorf("handleKey's Cmd nilness = %v, updateActive's = %v, want equal", cmdKey == nil, cmdActive == nil)
	}
}

// TestApp_ViewRendersStackTopWhenPresent is F5: View renders the top
// of the screen stack when one is pushed, not the active surface
// underneath it.
func TestApp_ViewRendersStackTopWhenPresent(t *testing.T) {
	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})

	app := NewApp(testDeps(t))
	app.stack = append(app.stack, &fakeModal{})

	if got := app.View().Content; got != "modal" {
		t.Errorf("View() with a stacked screen = %q, want the stack top's own view %q", got, "modal")
	}
}

// TestApp_ViewRendersActiveSurfaceWhenStackEmpty is F5's other half:
// an empty stack falls back to the active surface.
func TestApp_ViewRendersActiveSurfaceWhenStackEmpty(t *testing.T) {
	app := NewApp(testDeps(t))

	got := app.View().Content
	want := app.mail.View().Content
	if got != want {
		t.Errorf("View() with an empty stack = %q, want the active surface's own view %q", got, want)
	}
}

// TestNewProgram proves NewProgram returns a non-nil *tea.Program
// wrapping app, with the caller's own options layered on top of it.
func TestNewProgram(t *testing.T) {
	app := NewApp(testDeps(t))
	p := NewProgram(app, tea.WithInput(strings.NewReader("")))
	if p == nil {
		t.Fatal("NewProgram returned nil")
	}
}

// TestApp_BackgroundColorAnswerRebuildsTheme proves the answered path:
// a tea.BackgroundColorMsg rebuilds the theme from msg.IsDark() and
// the resolved profile, and broadcasts it to every surface screen.
func TestApp_BackgroundColorAnswerRebuildsTheme(t *testing.T) {
	app := NewApp(testDeps(t))
	before := app.theme

	updated := mustApp(t, first(app.Update(tea.BackgroundColorMsg{Color: whiteColor{}})))

	if updated.theme == before {
		t.Error("tea.BackgroundColorMsg did not rebuild the theme")
	}
	if updated.mail.theme != updated.theme {
		t.Error("the mail placeholder did not receive the rebuilt theme")
	}
}

// whiteColor is a minimal color.Color fixture resolving to a light
// background, so tea.BackgroundColorMsg.IsDark() reports false.
type whiteColor struct{}

func (whiteColor) RGBA() (r, g, b, a uint32) { return 0xffff, 0xffff, 0xffff, 0xffff }

// first drops the second return value, for chaining Update calls
// inline in a table-free test without a throwaway variable at each
// step.
func first(m tea.Model, _ tea.Cmd) tea.Model { return m }

// captureDebugLog points slog's default logger at a buffer for t's
// duration, at debug level, mirroring
// internal/backend/jmapsource/push_test.go's own helper.
func captureDebugLog(t *testing.T) *logBuffer {
	t.Helper()

	buf := &logBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

type logBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

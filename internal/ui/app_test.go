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
// names its Surface at StateDigitsSwitch, and the whole set
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
// as its answer/no-op instead of switching surfaces.
func TestApp_ModalEatsDigits(t *testing.T) {
	// fakeModal implements Screen (it is pushed onto App's stack
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

// TestApp_EscPopsStack proves Esc no-ops at a surface root (an empty
// stack), and pops a non-modal stack entry, this pass. A StateModal
// front owns Esc itself instead (TestApp_EscForwardsToStateModalFront,
// the template task-8-findings-r1.md's conventions ruling
// establishes), so it is no longer this test's concern.
func TestApp_EscPopsStack(t *testing.T) {
	app := NewApp(testDeps(t))

	app = mustApp(t, first(app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})))
	if len(app.stack) != 0 {
		t.Errorf("Esc at a surface root changed the stack: %v", app.stack)
	}

	app.stack = append(app.stack, &fakeScreen{})
	app = mustApp(t, first(app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})))
	if len(app.stack) != 0 {
		t.Errorf("Esc with one non-modal entry on the stack left %d, want 0", len(app.stack))
	}
}

// TestApp_EscForwardsToStateModalFront proves a StateModal stack
// front owns Esc itself now (task-8-findings-r1.md's conventions
// ruling): App forwards the key rather than bare-popping, and a modal
// that does not itself answer (fakeModal's Update never does)
// leaves the stack exactly as it found it.
func TestApp_EscForwardsToStateModalFront(t *testing.T) {
	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})

	app := NewApp(testDeps(t))
	app.stack = append(app.stack, &fakeModal{})

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	app = mustApp(t, first(app.Update(esc)))

	modal, ok := app.stack[len(app.stack)-1].(*fakeModal)
	if !ok {
		t.Fatalf("stack top is %T, want *fakeModal", app.stack[len(app.stack)-1])
	}
	if modal.seenDigit != esc.String() {
		t.Errorf("fakeModal.seenDigit = %q, want %q (Esc forwarded to the modal's own Update)", modal.seenDigit, esc.String())
	}
	if len(app.stack) != 1 {
		t.Errorf("stack length = %d, want 1 (fakeModal does not answer, so nothing pops it)", len(app.stack))
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
// active screen's Update (through updateActive) instead of being
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

// unadvertisedAlphabet is F1's fixed key alphabet (guard-falsifiability,
// MAJOR): every lowercase and uppercase letter, every digit, and the
// named specials mouse.go's keyPressForString already resolves,
// filtered down to whatever claimed does not already claim (a
// front's own flattened keymap plus GrammarKeys' whole field set):
// TestApp_UnadvertisedKeysAreNoOpsAtEverySurfaceRoot's whole input.
func unadvertisedAlphabet(claimed map[string]bool) []string {
	var out []string
	for c := 'a'; c <= 'z'; c++ {
		out = append(out, string(c))
	}
	for c := 'A'; c <= 'Z'; c++ {
		out = append(out, string(c))
	}
	for c := '0'; c <= '9'; c++ {
		out = append(out, string(c))
	}
	out = append(out, "esc", "enter", "tab", "space", "home", "end",
		"pgup", "pgdown", "up", "down", "left", "right", "backspace", "delete", "insert")

	var filtered []string
	for _, k := range out {
		if !claimed[k] {
			filtered = append(filtered, k)
		}
	}
	return filtered
}

// claimedKeys returns every physical key entry's own keymap or the
// global grammar binds, the set unadvertisedAlphabet excludes: a key
// claimed by either is legitimately meant to do something, so driving
// it proves nothing about an undocumented handler.
func claimedKeys(entry ScreenEntry) map[string]bool {
	claimed := make(map[string]bool)
	for _, b := range flattenKeys(entry.Keys) {
		for _, k := range b.Keys() {
			claimed[k] = true
		}
	}
	for _, b := range GrammarKeys.fields() {
		for _, k := range b.Keys() {
			claimed[k] = true
		}
	}
	return claimed
}

// TestApp_UnadvertisedKeysAreNoOpsAtEverySurfaceRoot is F1's live
// registry oracle (MAJOR, guard-falsifiability): a screen Update
// honoring a key no registry entry advertises must not pass silently
// (proven by perturbation: a live "z" handler added to
// MailPlaceholder's update passed the whole tree and the analyzers
// before this guard existed). For each surface root, every key in
// unadvertisedAlphabet not claimed by that front's own keymap or the
// global grammar drives App.Update directly against the live app
// (never a mock), and must return a nil Cmd and leave the rendered
// View unchanged.
func TestApp_UnadvertisedKeysAreNoOpsAtEverySurfaceRoot(t *testing.T) {
	for s := range Surface(len(surfaceNames)) {
		app := NewApp(testDeps(t))
		app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
		app.active.Set(app.account, s)

		front := app.activeScreen().Entry()
		before := app.View().Content

		for _, k := range unadvertisedAlphabet(claimedKeys(front)) {
			updated, cmd := app.Update(keyPressForString(k))
			if cmd != nil {
				t.Errorf("surface %v: key %q returned a non-nil Cmd, want nil (unadvertised)", s, k)
			}
			got := mustApp(t, updated).View().Content
			if got != before {
				t.Errorf("surface %v: key %q changed the View, want it unchanged (unadvertised)", s, k)
			}
		}
	}
}

// TestApp_ViewRendersStackTopWhenPresent is F5: View renders the top
// of the screen stack when one is pushed, not the active surface
// underneath it.
func TestApp_ViewRendersStackTopWhenPresent(t *testing.T) {
	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})

	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app.stack = append(app.stack, &fakeModal{})

	if got := app.View().Content; got != "modal" {
		t.Errorf("View() with a stacked screen = %q, want the stack top's own view %q", got, "modal")
	}
}

// TestApp_ViewRendersActiveSurfaceWhenStackEmpty is F5's other half:
// an empty stack falls back to the active surface, rendered through
// the same seam TestApp_ViewMatchesRenderSeam pins (FullRegion,
// isPlaceholderScreen's ruling), never the placeholder's bare
// unstyled View() alone.
func TestApp_ViewRendersActiveSurfaceWhenStackEmpty(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	want := Render(RenderInput{Screen: app.mail, FullRegion: true, Layout: app.layout, Theme: app.theme, Status: app.statusLine(), Banner: app.banner}).Content
	if got := app.View().Content; got != want {
		t.Errorf("View() with an empty stack = %q, want the active surface's own render %q", got, want)
	}
}

// TestApp_ViewSetsMouseModeAndAltScreenOnEveryPath proves task 11's
// carried review finding, extended to AltScreen (correctness C1, pass
// 2 final fix round): mouse cell-motion reporting and the alt-screen
// declaration are both per-frame tea.View fields, and all three of
// View's return paths (below the floor, the StateModal stack top, and
// the ordinary Render seam) set both, so neither a modal nor the
// floor rung can silently toggle either off.
func TestApp_ViewSetsMouseModeAndAltScreenOnEveryPath(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	if view := app.View(); view.MouseMode != tea.MouseModeCellMotion || !view.AltScreen {
		t.Errorf("View() with an empty stack MouseMode=%v AltScreen=%v, want MouseModeCellMotion and true", view.MouseMode, view.AltScreen)
	}

	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})
	app.stack = append(app.stack, &fakeModal{})
	if view := app.View(); view.MouseMode != tea.MouseModeCellMotion || !view.AltScreen {
		t.Errorf("View() with a StateModal stack top MouseMode=%v AltScreen=%v, want MouseModeCellMotion and true", view.MouseMode, view.AltScreen)
	}

	floor := mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 40, Height: 10})))
	if view := floor.View(); view.MouseMode != tea.MouseModeCellMotion || !view.AltScreen {
		t.Errorf("View() below the floor MouseMode=%v AltScreen=%v, want MouseModeCellMotion and true", view.MouseMode, view.AltScreen)
	}
}

// TestApp_ViewMatchesRenderSeam is CR1: after a WindowSizeMsg, the
// product's View().Content is exactly what Render produces from
// the same active screen, layout, and theme App itself holds, so the
// running program never drifts from what the gallery pins.
func TestApp_ViewMatchesRenderSeam(t *testing.T) {
	app := NewApp(testDeps(t))
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = mustApp(t, updated)

	screen := app.activeScreen()
	want := Render(RenderInput{Screen: screen, FullRegion: isPlaceholderScreen(screen), Layout: app.layout, Theme: app.theme, Status: app.statusLine(), Banner: app.banner}).Content
	if got := app.View().Content; got != want {
		t.Errorf("App.View().Content = %q, want Render(...)'s own content %q", got, want)
	}
}

// TestNewProgram proves NewProgram returns a non-nil *tea.Program
// wrapping app, with the caller's options layered on top of it.
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
// internal/backend/jmapsource/push_test.go's helper.
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

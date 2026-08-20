package ui

import (
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
)

// TestHitSpanAt proves hitSpanAt returns the first span whose Rect
// contains pt, and reports false once pt lands outside every span.
func TestHitSpanAt(t *testing.T) {
	spans := []HitSpan{
		{Target: PointerFooterHint, Rect: image.Rect(0, 0, 3, 1)},
		{Target: PointerBannerDismiss, Rect: image.Rect(3, 0, 6, 1)},
	}

	if got, ok := hitSpanAt(spans, image.Pt(1, 0)); !ok || got.Target != PointerFooterHint {
		t.Errorf("hitSpanAt((1,0)) = (%v, %v), want the first span", got, ok)
	}
	if got, ok := hitSpanAt(spans, image.Pt(4, 0)); !ok || got.Target != PointerBannerDismiss {
		t.Errorf("hitSpanAt((4,0)) = (%v, %v), want the second span", got, ok)
	}
	if _, ok := hitSpanAt(spans, image.Pt(6, 0)); ok {
		t.Error("hitSpanAt past every span's right edge matched, want none")
	}
}

// TestPaneAt proves paneAt resolves a point against LayoutMode's pane
// rectangles (ADR-0017's pane grain), and reports false for a point
// outside every pane (a chrome band, most notably).
func TestPaneAt(t *testing.T) {
	lm := ComputeLayout(140, 30, false)

	pt := lm.Content().Rect.Min
	if id, ok := paneAt(lm, pt); !ok || id != PaneContent {
		t.Errorf("paneAt(content origin) = (%v, %v), want (PaneContent, true)", id, ok)
	}

	if _, ok := paneAt(lm, image.Pt(0, 0)); ok {
		t.Error("paneAt(status row) matched a pane, want none (chrome bands are not panes)")
	}
}

// TestStatusDigitKeyAt proves statusDigitKeyAt resolves a click at a
// given digit span back to that digit's physical key, index for index
// against GrammarKeys.SurfaceSwitch.Keys(), not the shared
// SurfaceSwitch Verb every StatusLineHitSpans entry carries.
func TestStatusDigitKeyAt(t *testing.T) {
	statusRow := image.Rect(0, 0, 100, 1)
	keys := GrammarKeys.SurfaceSwitch.Keys()

	spans := StatusLineHitSpans(SurfaceMail, StateDigitsSwitch, statusRow)
	for i, span := range spans {
		got, ok := statusDigitKeyAt(SurfaceMail, StateDigitsSwitch, statusRow, span.Rect.Min)
		if !ok || got != keys[i] {
			t.Errorf("statusDigitKeyAt(digit %d) = (%q, %v), want (%q, true)", i, got, ok, keys[i])
		}
	}

	if _, ok := statusDigitKeyAt(SurfaceMail, StateModal, statusRow, spans[0].Rect.Min); ok {
		t.Error("statusDigitKeyAt matched at StateModal, want none (PointerSurfaceDigit is illegal there)")
	}
}

// TestKeyPressForString proves keyPressForString round-trips through
// tea.KeyPressMsg.String() back to s, for both a special key
// (specialKeyPresses) and a printable literal (the single-rune
// fallback), for every key GrammarKeys.fields() actually binds as a
// first key.
func TestKeyPressForString(t *testing.T) {
	for _, b := range GrammarKeys.fields() {
		s := b.Keys()[0]
		if got := keyPressForString(s).String(); got != s {
			t.Errorf("keyPressForString(%q).String() = %q, want %q", s, got, s)
		}
	}
}

// TestKeyPressForString_ArrowKeysResolveThroughTheMap proves the
// arrow/edit entries F3 added to specialKeyPresses resolve through
// the map lookup: "up" synthesizes tea.KeyUp, never the string's own
// first rune (the pre-fix bug the finding names) and never the
// multi-rune fallback's tea.KeyExtended.
func TestKeyPressForString_ArrowKeysResolveThroughTheMap(t *testing.T) {
	got := keyPressForString("up")
	if got.Code != tea.KeyUp {
		t.Errorf("keyPressForString(\"up\").Code = %v, want tea.KeyUp", got.Code)
	}
	if got.Code == 'u' {
		t.Error("keyPressForString(\"up\").Code synthesized the string's own first rune ('u'), want the up-arrow code")
	}
}

// TestKeyPressForString_MultiRuneFallback proves F3's total fallback:
// a multi-character string absent from specialKeyPresses synthesizes
// tea.KeyExtended (ultraviolet's own multi-rune code) with its own
// text, never the string's first rune alone.
func TestKeyPressForString_MultiRuneFallback(t *testing.T) {
	const s = "zzbogus"
	got := keyPressForString(s)

	if got.Code != tea.KeyExtended {
		t.Errorf("keyPressForString(%q).Code = %v, want tea.KeyExtended", s, got.Code)
	}
	if got.Text != s {
		t.Errorf("keyPressForString(%q).Text = %q, want %q", s, got.Text, s)
	}
	if got.String() != s {
		t.Errorf("keyPressForString(%q).String() = %q, want %q", s, got.String(), s)
	}
}

// TestResolveDoubleClick_WithinWindowUpgrades proves two clicks on the
// same synthetic target, the second arriving strictly inside
// doubleClickWindow of the first, upgrade to the open path and close
// the window.
func TestResolveDoubleClick_WithinWindowUpgrades(t *testing.T) {
	target := "row-1"
	opened := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	prev, double := resolveDoubleClick(pendingClick[string]{}, target, opened)
	if double {
		t.Fatal("the first click reported a double, want the opening single")
	}

	second := opened.Add(doubleClickWindow - time.Millisecond)
	next, double := resolveDoubleClick(prev, target, second)
	if !double {
		t.Fatalf("a second click on %q at window-1ms did not upgrade, want a double-click", target)
	}
	if next.open {
		t.Error("a double-click left the window open, want it cleared")
	}
}

// TestResolveDoubleClick_OutsideWindowSelectsTwice proves a second
// click on the same target arriving at or past doubleClickWindow does
// not upgrade: two plain selects, each opening its own fresh window.
func TestResolveDoubleClick_OutsideWindowSelectsTwice(t *testing.T) {
	target := "row-1"
	opened := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	prev, _ := resolveDoubleClick(pendingClick[string]{}, target, opened)

	second := opened.Add(doubleClickWindow + time.Millisecond)
	next, double := resolveDoubleClick(prev, target, second)
	if double {
		t.Fatalf("a second click on %q at window+1ms upgraded, want two plain selects", target)
	}
	if !next.open || next.target != target {
		t.Errorf("the outside-window click did not open its own fresh window: %+v", next)
	}
}

// TestResolveDoubleClick_DifferentTargetNeverUpgrades proves a second
// click within the window but on a different target is a plain
// select, not a double-click: same-target is part of the machine's
// test, not just timing.
func TestResolveDoubleClick_DifferentTargetNeverUpgrades(t *testing.T) {
	opened := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	prev, _ := resolveDoubleClick(pendingClick[string]{}, "row-1", opened)

	second := opened.Add(time.Millisecond)
	_, double := resolveDoubleClick(prev, "row-2", second)
	if double {
		t.Error("a click on a different target upgraded, want a plain select")
	}
}

// TestApp_DispatchClick_OnlyLeftButtonActs proves F1 (CRITICAL): every
// button but the left click is a no-op, even when its coordinates
// land squarely on a real hit span (a status digit, here): the
// pre-fix defect where any button, a middle-click near the footer
// included, fired whatever was underneath it.
func TestApp_DispatchClick_OnlyLeftButtonActs(t *testing.T) {
	tests := []struct {
		name string
		btn  tea.MouseButton
		acts bool
	}{
		{"left", tea.MouseLeft, true},
		{"middle", tea.MouseMiddle, false},
		{"right", tea.MouseRight, false},
		{"backward", tea.MouseBackward, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(testDeps(t))
			app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

			spans := StatusLineHitSpans(app.activeSurface(), StateDigitsSwitch, app.layout.StatusRow.Rect)
			click := tea.MouseClickMsg{Button: tt.btn, X: spans[SurfaceContacts].Rect.Min.X, Y: spans[SurfaceContacts].Rect.Min.Y}

			app = mustApp(t, first(app.Update(click)))
			got := app.activeSurface() == SurfaceContacts
			if got != tt.acts {
				t.Errorf("%s click at the digit-3 span: switched surface = %v, want %v", tt.name, got, tt.acts)
			}
		})
	}
}

// TestApp_ClickAtFloorRungResolvesNothing proves F2 (CRITICAL): the
// floor rung (width or height) paints no chrome at all (render.go's
// own early return), so dispatch resolves nothing there either,
// regardless of what a click's own coordinates land on.
func TestApp_ClickAtFloorRungResolvesNothing(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 40, Height: 10})))
	if app.layout.Class != WidthFloor || app.layout.HeightClass != HeightFloor {
		t.Fatalf("layout at 40x10 = %+v, want a floor rung", app.layout)
	}

	before := app.View().Content
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 5}
	updated, cmd := app.Update(click)
	app = mustApp(t, updated)
	if cmd != nil {
		t.Error("a click at the floor rung returned a non-nil Cmd, want none")
	}
	if got := app.View().Content; got != before {
		t.Error("a click at the floor rung changed the rendered view")
	}
}

// TestApp_ClickStatusDigit_SwitchesSurfaceInDigitsSwitchState proves
// the character-grain dispatch's first stop: clicking a status
// digit's rendered cell switches surfaces exactly as pressing that
// digit would, at a surface root (StateDigitsSwitch).
func TestApp_ClickStatusDigit_SwitchesSurfaceInDigitsSwitchState(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	spans := StatusLineHitSpans(app.activeSurface(), StateDigitsSwitch, app.layout.StatusRow.Rect)
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: spans[SurfaceContacts].Rect.Min.X, Y: spans[SurfaceContacts].Rect.Min.Y}

	app = mustApp(t, first(app.Update(click)))
	if got := app.activeSurface(); got != SurfaceContacts {
		t.Errorf("active surface after clicking digit 3 = %v, want SurfaceContacts", got)
	}
}

// TestApp_ClickStatusDigit_NoOpsOverModal proves the state rule's
// other direction: the same coordinates, with a StateModal screen on
// the stack, leave the active surface and the modal both untouched,
// exactly as the keyboard digit already does (TestApp_ModalEatsDigits).
func TestApp_ClickStatusDigit_NoOpsOverModal(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	spans := StatusLineHitSpans(app.activeSurface(), StateDigitsSwitch, app.layout.StatusRow.Rect)
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: spans[SurfaceContacts].Rect.Min.X, Y: spans[SurfaceContacts].Rect.Min.Y}

	app.stack = append(app.stack, &fakeModal{})
	app = mustApp(t, first(app.Update(click)))

	if got := app.activeSurface(); got != SurfaceMail {
		t.Errorf("active surface after a digit click over a modal = %v, want unchanged SurfaceMail", got)
	}
	if len(app.stack) != 1 {
		t.Fatalf("stack length after a digit click over a modal = %d, want 1 (the modal stays)", len(app.stack))
	}
}

// TestApp_ClickOverModalNeverResolvesTheCoveredBanner proves F2
// (CRITICAL): a StateModal front resolves only through its own
// HitSpans() (the F5 interface), never the status/footer/banner spans
// a full-terminal modal render painted over, even a click at the
// exact coordinate the banner's own dismiss glyph occupied before the
// modal covered it.
func TestApp_ClickOverModalNeverResolvesTheCoveredBanner(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(BannerMsg{Message: "No keyring found."})))

	front := app.frontEntry()
	dismiss := BannerHitSpans(app.banner, front.SwitchState, app.theme, app.layout.Banner.Rect)[0]

	app.stack = append(app.stack, testConfirm(t, nil, nil))

	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: dismiss.Rect.Min.X, Y: dismiss.Rect.Min.Y}
	app = mustApp(t, first(app.Update(click)))

	if !app.banner.Active {
		t.Error("a click over the modal at the covered banner's own dismiss column dismissed it, want it untouched")
	}
	if len(app.stack) != 1 {
		t.Fatalf("stack length after the click = %d, want 1 (unaffected)", len(app.stack))
	}
}

// TestApp_ClickFooterHint_RunsItsVerb proves clicking a rendered
// footer hint's cell fires exactly the Cmd pressing its key would:
// the mail placeholder's own "q quit" hint dispatched to tea.Quit.
func TestApp_ClickFooterHint_RunsItsVerb(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	front := app.frontEntry()
	spans := FooterHitSpans(front, front.SwitchState, app.layout.Footer.Rect)
	quit, ok := hitSpanAt(spans, spans[0].Rect.Min)
	if !ok || quit.Verb.Help().Desc != GrammarKeys.Quit.Help().Desc {
		t.Fatalf("the mail placeholder's first footer span = %+v, want the Quit hint", quit)
	}

	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: quit.Rect.Min.X, Y: quit.Rect.Min.Y}
	_, cmd := app.Update(click)
	if cmd == nil {
		t.Fatal("clicking the quit hint returned a nil Cmd, want tea.Quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("clicking the quit hint yielded %#v, want tea.QuitMsg", msg)
	}
}

// TestApp_ClickFooterNavigateHint_ScrollsDown proves RULING F6
// (task-10-findings-r2.md): a multi-key hint's click synthesizes its
// primary key, the forward direction: clicking help's own "j/k"
// footer hint scrolls down, never up.
func TestApp_ClickFooterNavigateHint_ScrollsDown(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 80, Height: 15})))
	app = mustApp(t, first(app.Update(helpKey())))

	front := app.frontEntry()
	spans := FooterHitSpans(front, front.SwitchState, app.layout.Footer.Rect)
	navigate, ok := hitSpanAt(spans, spans[0].Rect.Min)
	if !ok || navigate.Verb.Help().Desc != GrammarKeys.Navigate.Help().Desc {
		t.Fatalf("the help overlay's first footer span = %+v, want the Navigate hint", navigate)
	}

	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: navigate.Rect.Min.X, Y: navigate.Rect.Min.Y}
	app = mustApp(t, first(app.Update(click)))

	help, ok := app.stack[0].(HelpScreen)
	if !ok {
		t.Fatalf("stack[0] is %T, want HelpScreen", app.stack[0])
	}
	if help.scroll != 1 {
		t.Errorf("HelpScreen.scroll after clicking the Navigate hint = %d, want 1 (down, the primary key)", help.scroll)
	}
}

// TestApp_ClickBannerDismiss_Dismisses proves clicking the banner's
// dismiss glyph clears it, the same outcome Esc already produces
// (TestApp_EscDismissesBannerAtSurfaceRoot).
func TestApp_ClickBannerDismiss_Dismisses(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(BannerMsg{Message: "No keyring found."})))
	if !app.layout.BannerRow {
		t.Fatal("banner did not grow a row at HeightFull, want BannerRow true")
	}

	front := app.frontEntry()
	spans := BannerHitSpans(app.banner, front.SwitchState, app.theme, app.layout.Banner.Rect)
	if len(spans) != 1 {
		t.Fatalf("BannerHitSpans() = %d spans, want 1", len(spans))
	}

	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: spans[0].Rect.Min.X, Y: spans[0].Rect.Min.Y}
	app = mustApp(t, first(app.Update(click)))
	if app.banner.Active {
		t.Error("clicking the dismiss glyph left the banner active")
	}
}

// TestApp_ClickConfirmAnswer_Answers proves clicking a modal confirm's
// y/n cell answers exactly as the key would, through the same
// ConfirmAnsweredMsg round trip TestApp_ConfirmOnStack_AnswersYesNoAndPops
// drives with a real keypress. F9: app itself takes a real
// tea.WindowSizeMsg (not just the Confirm's own testConfirm layout),
// since dispatchClick's F2 floor guard reads a.layout directly.
func TestApp_ClickConfirmAnswer_Answers(t *testing.T) {
	yesCalled := false
	c := testConfirm(t, func() tea.Msg { yesCalled = true; return nil }, nil)

	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app.stack = append(app.stack, c)

	spans := ConfirmHitSpans(c)
	yes := spans[0]
	if yes.Verb.Help().Desc != "quit" {
		t.Fatalf("ConfirmHitSpans()[0] = %+v, want the yes answer", yes)
	}

	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: yes.Rect.Min.X, Y: yes.Rect.Min.Y}
	app, cmd := answerConfirm(t, app, click)

	if len(app.stack) != 0 {
		t.Fatalf("stack length after clicking yes = %d, want 0 (the modal pops on answer)", len(app.stack))
	}
	if cmd == nil {
		t.Fatal("clicking yes answered with a nil Cmd, want YesCmd")
	}
	cmd()
	if !yesCalled {
		t.Error("clicking yes did not invoke YesCmd")
	}
}

// TestApp_WheelScrollsChromeNoOps proves a wheel gesture whose
// coordinates land on a chrome band (the status row, here) is a
// no-op, even while a scrollable screen (the help overlay) sits on
// the stack: ADR-0017's chrome-band case, the counterpart to
// TestApp_WheelScrollsHelpWhileStacked's Main-band case.
func TestApp_WheelScrollsChromeNoOps(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 80, Height: 15})))
	app = mustApp(t, first(app.Update(helpKey())))

	app = mustApp(t, first(app.Update(WheelMsg{X: 0, Y: 0, Delta: 3})))

	help, ok := app.stack[0].(HelpScreen)
	if !ok {
		t.Fatalf("stack[0] is %T, want HelpScreen", app.stack[0])
	}
	if help.scroll != 0 {
		t.Errorf("HelpScreen.scroll after a wheel gesture over the status row = %d, want 0 (no-op)", help.scroll)
	}
}

// TestApp_WheelOverFooterNoOps mirrors TestApp_WheelScrollsChromeNoOps
// for the footer band (F9).
func TestApp_WheelOverFooterNoOps(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 80, Height: 15})))
	app = mustApp(t, first(app.Update(helpKey())))

	footer := app.layout.Footer.Rect
	app = mustApp(t, first(app.Update(WheelMsg{X: footer.Min.X, Y: footer.Min.Y, Delta: 3})))

	help, ok := app.stack[0].(HelpScreen)
	if !ok {
		t.Fatalf("stack[0] is %T, want HelpScreen", app.stack[0])
	}
	if help.scroll != 0 {
		t.Errorf("HelpScreen.scroll after a wheel gesture over the footer = %d, want 0 (no-op)", help.scroll)
	}
}

// TestApp_WheelOverBannerNoOps mirrors TestApp_WheelScrollsChromeNoOps
// for the banner band (F9): the banner only grows a row at
// HeightFull, so this uses a taller window than the status/footer
// no-op cases.
func TestApp_WheelOverBannerNoOps(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(BannerMsg{Message: "No keyring found."})))
	app = mustApp(t, first(app.Update(helpKey())))
	if !app.layout.BannerRow {
		t.Fatal("banner did not grow a row, want BannerRow true")
	}

	banner := app.layout.Banner.Rect
	app = mustApp(t, first(app.Update(WheelMsg{X: banner.Min.X, Y: banner.Min.Y, Delta: 3})))

	help, ok := app.stack[0].(HelpScreen)
	if !ok {
		t.Fatalf("stack[0] is %T, want HelpScreen", app.stack[0])
	}
	if help.scroll != 0 {
		t.Errorf("HelpScreen.scroll after a wheel gesture over the banner = %d, want 0 (no-op)", help.scroll)
	}
}

// TestApp_WheelOverModalNoOps proves a wheel gesture over a StateModal
// front (PointerWheel's own illegal state, registry.go's
// pointerLegalStates) is a no-op regardless of where its coordinates
// land, exactly as the j/k keys already are at a modal confirm.
func TestApp_WheelOverModalNoOps(t *testing.T) {
	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})

	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app.stack = append(app.stack, &fakeModal{})

	updated, cmd := app.Update(WheelMsg{X: 5, Y: 5, Delta: 3})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Error("a wheel gesture over a modal returned a non-nil Cmd, want none")
	}
	if len(app.stack) != 1 {
		t.Fatalf("stack length after a wheel gesture over a modal = %d, want 1 (untouched)", len(app.stack))
	}
}

// fakeWheelScreen is a minimal Screen recording whether it saw a
// WheelMsg, standing in for a stacked screen whose own registry entry
// may or may not declare PointerWheel (F4's own fixture).
type fakeWheelScreen struct {
	entry    ScreenEntry
	sawWheel bool
}

func (f *fakeWheelScreen) Init() tea.Cmd { return nil }

func (f *fakeWheelScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(WheelMsg); ok {
		f.sawWheel = true
	}
	return f, nil
}

func (f *fakeWheelScreen) View() tea.View { return tea.NewView("") }

func (f *fakeWheelScreen) Entry() ScreenEntry { return f.entry }

// TestApp_WheelUnregisteredFrontNoOps proves F4 (MAJOR): the registry
// is the authority for wheel routing, not merely pointerLegalStates'
// state table: a front whose own registry entry names no PointerWheel
// binding never sees a WheelMsg, even at a legal state and a legal
// coordinate, while a front that does register one does.
func TestApp_WheelUnregisteredFrontNoOps(t *testing.T) {
	resetRegistry(t)
	Register[*fakeWheelScreen](ScreenEntry{SwitchState: StateDigitsSwitch})

	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	noPointer := &fakeWheelScreen{entry: ScreenEntry{SwitchState: StateDigitsSwitch}}
	app.stack = append(app.stack, noPointer)

	updated, cmd := app.Update(WheelMsg{X: 5, Y: 5, Delta: 1})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Error("a wheel gesture over a front with no PointerWheel binding returned a non-nil Cmd, want none")
	}
	if noPointer.sawWheel {
		t.Error("a wheel gesture reached a front whose registry entry names no PointerWheel binding")
	}

	withPointer := &fakeWheelScreen{entry: ScreenEntry{
		SwitchState: StateDigitsSwitch,
		Pointer:     []PointerBinding{{Target: PointerWheel, Key: GrammarKeys.Navigate}},
	}}
	app.stack = []Screen{withPointer}

	app = mustApp(t, first(app.Update(WheelMsg{X: 5, Y: 5, Delta: 1})))
	if !withPointer.sawWheel {
		t.Error("a wheel gesture did not reach a front whose registry entry names PointerWheel")
	}
}

// TestApp_NoOpClickLeavesTheGoldenUnchanged proves a click that lands
// inside the Main band but on neither a hit span nor anything
// clickable (a bare content-pane cell, this pass) leaves App's
// rendered view byte-for-byte unchanged: pointer input changes state,
// never rendering rules (ADR-0017's testing section). F9: the click
// lands inside Main, a real hover-free miss, not merely past the
// terminal's own bounds.
func TestApp_NoOpClickLeavesTheGoldenUnchanged(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(mailStatsMsg{stats: store.MailStats{Messages: 5, Mailboxes: 1}})))

	before := app.View().Content

	content := app.layout.Content().Rect
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: content.Min.X + 2, Y: content.Min.Y + 2}
	app = mustApp(t, first(app.Update(click)))

	after := app.View().Content
	if before != after {
		t.Errorf("a no-op click changed the rendered view:\nbefore: %q\nafter:  %q", before, after)
	}
}

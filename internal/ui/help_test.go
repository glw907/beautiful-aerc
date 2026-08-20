package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

func helpKey() tea.KeyPressMsg { return tea.KeyPressMsg{Code: '?', Text: "?"} }

// TestApp_HelpOpensFromEverySurface is UX-5's own acceptance criterion
// and F6: it drives off the Surface enum itself (range over
// Surface(len(surfaceNames)), never a hand-typed count), takes want
// from the live app.activeScreen().Entry() rather than a
// separately-constructed placeholder literal, and then cross-checks
// coverage against the registry directly: every Registered()
// digits-switch entry except help itself must have been reached by
// some iteration above, so a future fifth surface can never silently
// go untested.
func TestApp_HelpOpensFromEverySurface(t *testing.T) {
	reached := make(map[string]bool)

	for s := range Surface(len(surfaceNames)) {
		app := NewApp(testDeps(t))
		app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
		app.active.Set(app.account, s)
		want := app.activeScreen().Entry()

		app = mustApp(t, first(app.Update(helpKey())))

		if len(app.stack) != 1 {
			t.Fatalf("surface %v: stack length after ? = %d, want 1", s, len(app.stack))
		}
		help, ok := app.stack[0].(HelpScreen)
		if !ok {
			t.Fatalf("surface %v: stack[0] is %T, want HelpScreen", s, app.stack[0])
		}
		if help.Covered.Name != want.Name {
			t.Errorf("surface %v: Covered.Name = %q, want %q (the surface help was opened over)", s, help.Covered.Name, want.Name)
		}
		reached[want.Name] = true
	}

	for _, e := range Registered() {
		if e.SwitchState != StateDigitsSwitch || e.Name == helpScreenName {
			continue
		}
		if !reached[e.Name] {
			t.Errorf("registered digits-switch entry %q was never reached by the surface loop above", e.Name)
		}
	}
}

// TestApp_HelpDoesNotOpenOverAModal proves `?` is a no-op while a
// StateModal front is showing, matching every other key a modal does
// not itself bind (confirm.go's own "every other key is a no-op"
// rule): help never covers, or is covered by, a modal confirm.
func TestApp_HelpDoesNotOpenOverAModal(t *testing.T) {
	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})

	app := NewApp(testDeps(t))
	app.stack = append(app.stack, &fakeModal{})

	app = mustApp(t, first(app.Update(helpKey())))

	if len(app.stack) != 1 {
		t.Fatalf("stack length after ? over a modal = %d, want 1 (the modal stays, no help pushed)", len(app.stack))
	}
	if _, ok := app.stack[0].(*fakeModal); !ok {
		t.Fatalf("stack[0] is %T, want the modal unchanged", app.stack[0])
	}
}

// TestApp_HelpTogglesOpenAndClosed is C3's own ruling: `?` is a
// toggle (the mutt/aerc/less idiom), not a one-way open with a
// re-entry guard. A second `?` while help is already the front pops
// it, tested in both directions.
func TestApp_HelpTogglesOpenAndClosed(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	app = mustApp(t, first(app.Update(helpKey())))
	if len(app.stack) != 1 {
		t.Fatalf("stack length after the first ? = %d, want 1 (open)", len(app.stack))
	}
	if _, ok := app.stack[0].(HelpScreen); !ok {
		t.Fatalf("stack[0] is %T, want HelpScreen", app.stack[0])
	}

	app = mustApp(t, first(app.Update(helpKey())))
	if len(app.stack) != 0 {
		t.Errorf("stack length after the second ? = %d, want 0 (close)", len(app.stack))
	}
}

// TestHelpOpenEligible_GatesOnSwitchState is C2, CRITICAL: the toggle
// gate is StateDigitsSwitch only, mirroring undoEligible's own front
// gate (TestApp_UndoEligible_GatesOnStackAndSwitchState's shape).
// A StatePrintableEntry front must disqualify `?` from opening help,
// the same way it disqualifies `u` from answering an undo window, so
// a search bar or a picker filter field keeps its own `?` character.
func TestHelpOpenEligible_GatesOnSwitchState(t *testing.T) {
	if !helpOpenEligible(ScreenEntry{SwitchState: StateDigitsSwitch}) {
		t.Error("helpOpenEligible() with a StateDigitsSwitch front = false, want true")
	}
	if helpOpenEligible(ScreenEntry{SwitchState: StatePrintableEntry}) {
		t.Error("helpOpenEligible() with a StatePrintableEntry front = true, want false (C2 probe: ? stays that front's own character)")
	}
	if helpOpenEligible(ScreenEntry{SwitchState: StateModal}) {
		t.Error("helpOpenEligible() with a StateModal front = true, want false")
	}
}

// TestApp_HelpDoesNotInterceptAPrintableEntryQuestionMark is C2's own
// end-to-end case: `?` reaches a StatePrintableEntry front's normal
// Update untouched, rather than being intercepted as the help toggle.
func TestApp_HelpDoesNotInterceptAPrintableEntryQuestionMark(t *testing.T) {
	resetRegistry(t)
	Register[*fakeEntryScreen](ScreenEntry{SwitchState: StatePrintableEntry})

	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app.stack = append(app.stack, &fakeEntryScreen{})

	app = mustApp(t, first(app.Update(helpKey())))

	if len(app.stack) != 1 {
		t.Fatalf("stack length after ? over a printable-entry front = %d, want 1 (no help pushed)", len(app.stack))
	}
	if _, ok := app.stack[0].(*fakeEntryScreen); !ok {
		t.Fatalf("stack[0] is %T, want the entry screen unchanged (? stays that front's own character)", app.stack[0])
	}
}

// TestApp_DigitPopsHelpAndSwitchesSurface is UX-5's own carried
// ruling: digits switch surfaces from help, and pop it, rather than
// leaving the overlay stranded on top of the newly active surface.
func TestApp_DigitPopsHelpAndSwitchesSurface(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(helpKey())))

	app = mustApp(t, first(app.Update(digitKey("3"))))

	if len(app.stack) != 0 {
		t.Errorf("stack length after a digit from help = %d, want 0 (help pops)", len(app.stack))
	}
	if got := app.activeSurface(); got != SurfaceContacts {
		t.Errorf("active surface after a digit from help = %v, want SurfaceContacts", got)
	}
}

// TestApp_EscPopsHelp proves UX-5's own Esc acceptance criterion via
// the ordinary Back branch, HelpScreen's SwitchState being
// StateDigitsSwitch rather than StateModal.
func TestApp_EscPopsHelp(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(helpKey())))

	app = mustApp(t, first(app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})))

	if len(app.stack) != 0 {
		t.Errorf("stack length after Esc from help = %d, want 0", len(app.stack))
	}
}

// TestApp_FooterFollowsHelpWhileStacked is the CARRY fix from task
// 7's review: once help is open, the footer must show its own hints,
// not the covered surface's, since the surface's own verbs are no
// longer legal (UX-2's no-advertised-no-op MUST).
func TestApp_FooterFollowsHelpWhileStacked(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(helpKey())))

	content := ansi.Strip(app.View().Content)
	lines := strings.Split(content, "\n")
	footer := lines[len(lines)-1]

	if !strings.Contains(footer, "navigate") || !strings.Contains(footer, "back") {
		t.Errorf("footer while help is open = %q, want help's own hints", footer)
	}
	if strings.Contains(footer, "quit") {
		t.Errorf("footer while help is open = %q, want the covered mail placeholder's quit hint gone", footer)
	}
}

func helpTestTheme() theme.Theme { return theme.New(true, theme.ProfileTrueColor) }

// TestHelpScreen_ContentDerivesFromCoveredEntry is UX-5's own
// drift-impossibility acceptance criterion: mutating a fake covered
// entry changes the rendered overlay, since the This-screen section
// reads Covered live rather than through a copy taken at push time.
func TestHelpScreen_ContentDerivesFromCoveredEntry(t *testing.T) {
	entry := ScreenEntry{Name: "fake screen", Keys: flatKeyMap(bind("x", "x", "do a thing"))}
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(80, 24, false), Covered: entry}

	before := ansi.Strip(h.View().Content)
	if !strings.Contains(before, "do a thing") {
		t.Fatalf("help body = %q, want the covered entry's own binding description", before)
	}

	entry.Keys = flatKeyMap(bind("x", "x", "do a mutated thing"))
	h.Covered = entry
	after := ansi.Strip(h.View().Content)

	if !strings.Contains(after, "do a mutated thing") {
		t.Errorf("help body after mutating Covered = %q, want the mutated description", after)
	}
	if strings.Contains(after, "do a thing") && !strings.Contains(after, "do a mutated thing") {
		t.Errorf("help body after mutating Covered still shows the stale description: %q", after)
	}
}

// TestHelpScreen_GlobalSectionDerivesFromGrammarKeys proves the
// Global section's own rows come from GrammarKeys, never a re-typed
// literal: every helpGlobalKeys binding's own key appears in the
// rendered body, and so does its own description, except
// SurfaceSwitch's, which TestHelpScreen_GlobalSectionNamesSiblingSurfaces
// covers on its own (its row names every sibling surface instead of
// rendering "surface switch" verbatim, C1's own fix).
func TestHelpScreen_GlobalSectionDerivesFromGrammarKeys(t *testing.T) {
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(80, 24, false), Covered: MailPlaceholder{}.Entry()}
	body := ansi.Strip(h.View().Content)

	for _, b := range helpGlobalKeys {
		if !strings.Contains(body, b.Help().Key) {
			t.Errorf("help body missing Global row's key for %v", b.Help())
		}
		if b.Help().Desc == GrammarKeys.SurfaceSwitch.Help().Desc {
			continue
		}
		if !strings.Contains(body, b.Help().Desc) {
			t.Errorf("help body missing Global row's description for %v", b.Help())
		}
	}
}

// TestHelpScreen_OneColumnBelowStandard proves the layout switch
// (wireframe F5/F6) lands exactly at WidthStandard: below it, the
// Global and This-screen headers land on separate lines; at or above
// it, they share one.
func TestHelpScreen_OneColumnBelowStandard(t *testing.T) {
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(99, 30, false), Covered: MailPlaceholder{}.Entry()}
	if h.twoColumn() {
		t.Error("twoColumn() at 99 columns = true, want false")
	}

	body := ansi.Strip(h.View().Content)
	lines := strings.Split(body, "\n")
	if !lineHasOnly(lines, "Global") || !lineHasOnly(lines, "This screen") {
		t.Errorf("one-column body did not carry both section headers on their own lines:\n%s", body)
	}
}

func TestHelpScreen_TwoColumnAtStandard(t *testing.T) {
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(100, 30, false), Covered: MailPlaceholder{}.Entry()}
	if !h.twoColumn() {
		t.Error("twoColumn() at 100 columns = false, want true")
	}

	body := ansi.Strip(h.View().Content)
	var sharedRow bool
	for line := range strings.SplitSeq(body, "\n") {
		if strings.Contains(line, "Global") && strings.Contains(line, "This screen") {
			sharedRow = true
		}
	}
	if !sharedRow {
		t.Errorf("two-column body has no row carrying both section headers:\n%s", body)
	}
}

// lineHasOnly reports whether one of lines, trimmed, equals want.
func lineHasOnly(lines []string, want string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

// TestHelpScreen_WheelScrolls is UX-5's own wheel acceptance
// criterion: a coalesced WheelMsg (the typed message ADR-0017's
// dispatcher produces) moves the scroll position, clamped to the
// body's own bounds.
func TestHelpScreen_WheelScrolls(t *testing.T) {
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(80, 15, false), Covered: MailPlaceholder{}.Entry()}

	updated, _ := h.Update(WheelMsg{Delta: 1})
	h = updated.(HelpScreen) //nolint:errcheck // HelpScreen's own Update always returns a HelpScreen

	if h.scroll != 1 {
		t.Fatalf("scroll after WheelMsg{Delta: 1} = %d, want 1", h.scroll)
	}

	updated, _ = h.Update(WheelMsg{Delta: -100})
	h = updated.(HelpScreen) //nolint:errcheck // HelpScreen's own Update always returns a HelpScreen
	if h.scroll != 0 {
		t.Errorf("scroll after a large negative WheelMsg = %d, want clamped to 0", h.scroll)
	}
}

// TestApp_WheelScrollsHelpWhileStacked proves the coalesced WheelMsg
// App's own dispatchWheel routes to a stacked HelpScreen (task 10)
// when the gesture's own coordinates land within the Main band the
// overlay fills (FullRegion): the position-aware routing that
// replaced the old coordinate-blind updateChildren broadcast
// (task-8-findings-r1.md ruling F4).
func TestApp_WheelScrollsHelpWhileStacked(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 80, Height: 15})))
	app = mustApp(t, first(app.Update(helpKey())))

	app = mustApp(t, first(app.Update(WheelMsg{X: 5, Y: 5, Delta: 2})))

	help, ok := app.stack[0].(HelpScreen)
	if !ok {
		t.Fatalf("stack[0] is %T, want HelpScreen", app.stack[0])
	}
	if help.scroll != 2 {
		t.Errorf("HelpScreen.scroll after a WheelMsg through App = %d, want 2", help.scroll)
	}
}

// TestHelpScreen_NavigateAndPageKeysScroll proves the navigation
// family's own two halves (j/k, Space/b) step the body by one line and
// one viewport respectively, and Home/End jump to either extreme.
func TestHelpScreen_NavigateAndPageKeysScroll(t *testing.T) {
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(80, 15, false), Covered: MailPlaceholder{}.Entry()}

	updated, _ := h.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	h = updated.(HelpScreen) //nolint:errcheck // HelpScreen's own Update always returns a HelpScreen
	if h.scroll != 1 {
		t.Fatalf("scroll after j = %d, want 1", h.scroll)
	}

	updated, _ = h.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	h = updated.(HelpScreen) //nolint:errcheck // HelpScreen's own Update always returns a HelpScreen
	if h.scroll != 0 {
		t.Fatalf("scroll after k = %d, want 0", h.scroll)
	}

	updated, _ = h.Update(tea.KeyPressMsg{Code: tea.KeyEnd, Text: "end"})
	h = updated.(HelpScreen) //nolint:errcheck // HelpScreen's own Update always returns a HelpScreen
	if h.scroll == 0 {
		t.Fatal("scroll after End = 0, want the body's own bottom")
	}

	updated, _ = h.Update(tea.KeyPressMsg{Code: tea.KeyHome, Text: "home"})
	h = updated.(HelpScreen) //nolint:errcheck // HelpScreen's own Update always returns a HelpScreen
	if h.scroll != 0 {
		t.Errorf("scroll after Home = %d, want 0", h.scroll)
	}
}

// keyPressNamed builds a tea.KeyPressMsg whose String() equals name,
// for a name one of GrammarKeys' own bindings carries: a special
// key's String() derives from Code (the decoder's own table), never
// Text, unlike a plain letter's, so this resolves each name to the
// right one.
func keyPressNamed(name string) tea.KeyPressMsg {
	switch name {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	default:
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
}

// TestHelpNavigateUpPartitionsNavigateKeys and
// TestHelpPageBackPartitionsPageKeys are F7: the two helpers must
// classify every one of GrammarKeys' own bound keys, and only those.
// A grammar amendment that adds, removes, or renames a key here must
// fail this test loudly rather than leave applyKey silently
// misreading a key it no longer recognizes.
func TestHelpNavigateUpPartitionsNavigateKeys(t *testing.T) {
	up := map[string]bool{"k": true, "up": true, "j": false, "down": false}
	keys := GrammarKeys.Navigate.Keys()
	if len(keys) != len(up) {
		t.Fatalf("GrammarKeys.Navigate.Keys() = %v, want exactly %d keys (helpNavigateUp's own partition)", keys, len(up))
	}
	for _, k := range keys {
		want, known := up[k]
		if !known {
			t.Fatalf("GrammarKeys.Navigate carries key %q, unknown to helpNavigateUp's own partition", k)
		}
		if got := helpNavigateUp(keyPressNamed(k)); got != want {
			t.Errorf("helpNavigateUp(%q) = %v, want %v", k, got, want)
		}
	}
}

func TestHelpPageBackPartitionsPageKeys(t *testing.T) {
	back := map[string]bool{"b": true, "pgup": true, "space": false, "pgdown": false}
	keys := GrammarKeys.Page.Keys()
	if len(keys) != len(back) {
		t.Fatalf("GrammarKeys.Page.Keys() = %v, want exactly %d keys (helpPageBack's own partition)", keys, len(back))
	}
	for _, k := range keys {
		want, known := back[k]
		if !known {
			t.Fatalf("GrammarKeys.Page carries key %q, unknown to helpPageBack's own partition", k)
		}
		if got := helpPageBack(keyPressNamed(k)); got != want {
			t.Errorf("helpPageBack(%q) = %v, want %v", k, got, want)
		}
	}
}

// TestHelpScreen_HomeLiteralIsBoundToExtremes is F7's own third leg:
// applyKey's "home" literal, the jump-to-top branch, must actually
// name one of GrammarKeys.Extremes' own bound keys, so a grammar
// amendment renaming it fails this test loudly rather than leaving
// Home silently falling through to the jump-to-bottom branch.
func TestHelpScreen_HomeLiteralIsBoundToExtremes(t *testing.T) {
	if !slices.Contains(GrammarKeys.Extremes.Keys(), "home") {
		t.Errorf("GrammarKeys.Extremes.Keys() = %v, want \"home\" among them (applyKey's own literal)", GrammarKeys.Extremes.Keys())
	}
}

// TestJoinHelpColumns_TruncatesAWideLeftColumn is F8: a left column
// wider than colWidth is truncated with th's own ellipsis token
// (decision 12) rather than overflowing into the right column's own
// gutter.
func TestJoinHelpColumns_TruncatesAWideLeftColumn(t *testing.T) {
	th := helpTestTheme()
	left := []rowSeg{{text: strings.Repeat("x", 50), role: theme.RoleFg}}
	right := []rowSeg{{text: "y", role: theme.RoleFg}}

	got := joinHelpColumns(th, left, right, 10)
	plain := segsPlainText(got)

	if !strings.Contains(plain, th.Glyphs().Ellipsis) {
		t.Errorf("joinHelpColumns over-width left column = %q, want the ellipsis token marking the cut", plain)
	}
	if !strings.HasSuffix(plain, "y") {
		t.Errorf("joinHelpColumns = %q, want the right column intact after the truncated left one", plain)
	}
}

// TestHelpScreen_GlobalSectionNamesSiblingSurfaces is C1, CRITICAL:
// the SurfaceSwitch row names every sibling surface, derived from
// surfaceNames itself (the same array the status line's own cluster
// renders), never a re-typed literal: the row this package's own
// product was entirely missing before this fix round.
func TestHelpScreen_GlobalSectionNamesSiblingSurfaces(t *testing.T) {
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(80, 24, false), Covered: MailPlaceholder{}.Entry()}
	body := ansi.Strip(h.View().Content)

	for _, name := range surfaceNames {
		if !strings.Contains(body, name) {
			t.Errorf("help body missing sibling surface name %q", name)
		}
	}
	if strings.Contains(body, GrammarKeys.SurfaceSwitch.Help().Desc) {
		t.Errorf("help body still carries SurfaceSwitch's generic %q description, want the sibling-names row instead", GrammarKeys.SurfaceSwitch.Help().Desc)
	}
}

package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

func helpKey() tea.KeyPressMsg { return tea.KeyPressMsg{Code: '?', Text: "?"} }

// TestApp_HelpOpensFromEverySurface is UX-5's own acceptance
// criterion: `?` opens the help overlay from every registered surface
// screen, carrying that surface's own registered entry as Covered
// (the section the overlay's own This-screen listing derives from).
func TestApp_HelpOpensFromEverySurface(t *testing.T) {
	tests := []struct {
		name    string
		surface Surface
		want    ScreenEntry
	}{
		{"mail", SurfaceMail, MailPlaceholder{}.Entry()},
		{"calendar", SurfaceCalendar, CalendarPlaceholder{}.Entry()},
		{"contacts", SurfaceContacts, ContactsPlaceholder{}.Entry()},
		{"config", SurfaceConfig, ConfigPlaceholder{}.Entry()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(testDeps(t))
			app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
			app.active.Set(app.account, tt.surface)

			app = mustApp(t, first(app.Update(helpKey())))

			if len(app.stack) != 1 {
				t.Fatalf("stack length after ? = %d, want 1", len(app.stack))
			}
			help, ok := app.stack[0].(HelpScreen)
			if !ok {
				t.Fatalf("stack[0] is %T, want HelpScreen", app.stack[0])
			}
			if help.Covered.Name != tt.want.Name {
				t.Errorf("HelpScreen.Covered.Name = %q, want %q (the surface help was opened over)", help.Covered.Name, tt.want.Name)
			}
		})
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

// TestApp_HelpDoesNotDoublePush proves `?` while help is already the
// front is a no-op rather than pushing a second overlay: `?` is absent
// from HelpScreen's own keymap, so this also exercises App's own
// re-entry guard ahead of that absence.
func TestApp_HelpDoesNotDoublePush(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	app = mustApp(t, first(app.Update(helpKey())))
	app = mustApp(t, first(app.Update(helpKey())))

	if len(app.stack) != 1 {
		t.Errorf("stack length after a second ? = %d, want 1 (no double push)", len(app.stack))
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
// literal: every helpGlobalKeys binding's key and description appear
// in the rendered body.
func TestHelpScreen_GlobalSectionDerivesFromGrammarKeys(t *testing.T) {
	h := HelpScreen{theme: helpTestTheme(), layout: ComputeLayout(80, 24, false), Covered: MailPlaceholder{}.Entry()}
	body := ansi.Strip(h.View().Content)

	for _, b := range helpGlobalKeys {
		if !strings.Contains(body, b.Help().Key) || !strings.Contains(body, b.Help().Desc) {
			t.Errorf("help body missing Global row for %v", b.Help())
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
// App's own wheel gesture flushes reaches a stacked HelpScreen through
// the generic updateChildren forwarding (task-8-findings-r1.md ruling
// F4), the same path a resize or theme change already takes.
func TestApp_WheelScrollsHelpWhileStacked(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 80, Height: 15})))
	app = mustApp(t, first(app.Update(helpKey())))

	app = mustApp(t, first(app.Update(WheelMsg{Delta: 2})))

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

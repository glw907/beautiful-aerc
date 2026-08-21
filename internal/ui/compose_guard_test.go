package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/glw907/poplar/internal/ui/fixtures"
)

// composeGuardResistant names the two fixtures.All entries this test
// cannot seed through App's exported surface: MailLoaded pins a
// 36,102-message, 14-mailbox count, and Calendar pins a 42-event
// count, neither reachable without seeding that many real store rows
// through storetest.Insert/OpenStore, which the write-call analyzer
// (ADR-0003) refuses from internal/ui: this package sits on the
// writer/intent rule's read-only side, so no test here may reach
// store.Writer regardless of what a real running App would eventually
// do through the writer goroutine. TestComposeView_MatchesMailLoadedFixture
// and TestComposeView_MatchesCalendarFixture
// (compose_guard_internal_test.go, package ui) cover both instead,
// through App's unexported mail/calendar fields directly rather than
// a real Init load.
var composeGuardResistant = map[string]bool{
	fixtures.MailLoaded.Name: true,
	fixtures.Calendar.Name:   true,
}

// composeGuardSeeds pairs each of fixtures.All's other entries with
// the real, exported App flow that reaches the same state the
// fixture pins: NewApp plus Init (a fresh, empty storetest pool,
// matching every mail-family fixture's zero mailStats) and, where the
// fixture's state needs one, a real public Update message (SyncStateMsg,
// ToastMsg, BannerMsg, a key press) rather than any unexported field.
// mail-syncing's spinner frame is the one state with no single message
// for it: SyncStateMsg only arms the first tick, so advanceSpinner
// runs the real tick chain forward by hand.
var composeGuardSeeds = map[string]func(t *testing.T, th theme.Theme) (app ui.App, width, height int){
	"mail": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		return newSizedTestApp(t, th, 100, 30), 100, 30
	},
	"mail-syncing": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		updated, cmd := app.Update(ui.SyncStateMsg{State: ui.SyncStateSyncing, Done: 4312, Total: 36102})
		return advanceSpinner(t, asApp(t, updated), cmd, 2), 100, 30
	},
	"mail-offline": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		return updateApp(t, app, ui.SyncStateMsg{State: ui.SyncStateOffline}), 100, 30
	},
	"mail-backing-off": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		return updateApp(t, app, ui.SyncStateMsg{State: ui.SyncStateBackingOff, Retry: 12}), 100, 30
	},
	"mail-toast": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		offer := ui.UndoOffer{Label: "3 messages archived", Undo: func() tea.Msg { return nil }}
		// A one-shot Update, cmd discarded rather than run through
		// updateApp/runCmd: showToast's returned Cmd is the toast
		// countdown's own re-arming tick chain, and draining it fully
		// would count all the way down to the window closing, ten
		// real seconds later, rather than stopping at the fixture's
		// pinned Remaining: 9.
		updated, _ := app.Update(ui.ToastMsg{Offer: offer})
		return asApp(t, updated), 100, 30
	},
	"mail-banner": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		msg := ui.BannerMsg{Message: "No keyring found, so your token is stored in a plain file."}
		return updateApp(t, app, msg), 100, 30
	},
	"modal-confirm": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		app = updateApp(t, app, ui.OutboxCountMsg{Queued: 2})
		return updateApp(t, app, tea.KeyPressMsg{Code: 'q', Text: "q"}), 100, 30
	},
	"help": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		return updateApp(t, app, tea.KeyPressMsg{Code: '?', Text: "?"}), 100, 30
	},
	"contacts": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		return updateApp(t, app, tea.KeyPressMsg{Code: '3', Text: "3"}), 100, 30
	},
	"config": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		app := newSizedTestApp(t, th, 100, 30)
		return updateApp(t, app, tea.KeyPressMsg{Code: '4', Text: "4"}), 100, 30
	},
	"floor": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		return newSizedTestApp(t, th, 40, 10), 40, 10
	},
	"short": func(t *testing.T, th theme.Theme) (ui.App, int, int) {
		return newSizedTestApp(t, th, 100, 16), 100, 16
	},
}

// TestComposeView_MatchesFixtureEquivalentAppState is task 2 of pass
// 2c's equality guard, App's side of it (fix round 1's rewrite): for
// every fixture in fixtures.All (the same list cmd/sketch cycles), a
// real App driven through NewApp, Init, and the real public messages
// composeGuardSeeds sends renders a frame byte-identical to
// ui.ComposeView's own render of the fixture's direct construction,
// split onto ComposeView's active or stack argument by f.Stack (the
// same field cmd/sketch's computeFrame reads). The two sides reach
// modal-confirm's and help's stacked state through entirely different
// means (App through a real quit-with-outbox key press and a real ?
// key press; the fixture side through f.Stack), so a fixture whose
// Stack field disagrees with what App would actually do fails here,
// not just the tautological cmd/sketch-side check in
// cmd/sketch/main_test.go. A fixture missing from composeGuardSeeds,
// and not named in composeGuardResistant, fails loudly rather than
// silently rendering an empty want/got pair; composeGuardResistant's
// two entries (mail-loaded, calendar) are covered by
// compose_guard_internal_test.go instead.
func TestComposeView_MatchesFixtureEquivalentAppState(t *testing.T) {
	th := theme.New(ui.DefaultDark, theme.ProfileTrueColor)

	for _, f := range fixtures.All {
		if composeGuardResistant[f.Name] {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			seed, ok := composeGuardSeeds[f.Name]
			if !ok {
				t.Fatalf("fixture %s has no equality-guard seed in composeGuardSeeds", f.Name)
			}
			app, width, height := seed(t, th)

			lm := ui.ComputeLayout(width, height, f.Banner.Active)
			updated, _ := f.Build(th).Update(ui.LayoutMsg{Layout: lm})
			scr, ok := updated.(ui.Screen)
			if !ok {
				t.Fatalf("Update returned %T, want ui.Screen", updated)
			}

			var active ui.Screen
			var stack []ui.Screen
			if f.Stack {
				stack = []ui.Screen{scr}
			} else {
				active = scr
			}
			want := ui.ComposeView(lm, th, f.Status, f.Banner, active, stack)

			got := app.View()
			if got.Content != want.Content {
				t.Errorf("App.View().Content diverged from ComposeView's own frame for the fixture-equivalent state")
			}
			if (got.Cursor == nil) != (want.Cursor == nil) {
				t.Errorf("App.View().Cursor = %v, want %v", got.Cursor, want.Cursor)
			} else if got.Cursor != nil && *got.Cursor != *want.Cursor {
				t.Errorf("App.View().Cursor = %v, want %v", *got.Cursor, *want.Cursor)
			}
		})
	}
}

// newSizedTestApp returns an App wired to a fresh, empty storetest
// pool (so its mail placeholder's real store facts are the same zero
// counts fixtures.Mail pins) themed th, with Init's whole Cmd tree
// drained and a width×height WindowSizeMsg applied: repaint_test.go's
// newTestApp parameterized on size and theme, since this file needs
// both the floor and short rungs and a theme value it shares with the
// fixture-driven "want" side.
func newSizedTestApp(t *testing.T, th theme.Theme, width, height int) ui.App {
	t.Helper()
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	deps := ui.Deps{Store: reads, Theme: th, Profile: theme.ProfileTrueColor, Account: "geoff@907.life"}
	app := ui.NewApp(deps)
	app = runCmd(t, app, app.Init())
	return updateApp(t, app, tea.WindowSizeMsg{Width: width, Height: height})
}

// advanceSpinner runs cmd exactly n times, feeding each tick msg back
// into app.Update, to reach a known spinner frame without draining
// the chain runCmd would: a Syncing state's spinner tick re-arms
// itself forever, so full recursive draining never returns.
func advanceSpinner(t *testing.T, app ui.App, cmd tea.Cmd, n int) ui.App {
	t.Helper()
	for range n {
		if cmd == nil {
			t.Fatalf("advanceSpinner: nil Cmd before %d ticks ran", n)
		}
		updated, next := app.Update(cmd())
		app = asApp(t, updated)
		cmd = next
	}
	return app
}

// asApp asserts model is an ui.App, the same assertion
// repaint_test.go's updateApp makes inline: App.Update always returns
// an App, so a mismatch is this file's own bug, not the product's.
func asApp(t *testing.T, model tea.Model) ui.App {
	t.Helper()
	app, ok := model.(ui.App)
	if !ok {
		t.Fatalf("Update returned %T, want ui.App", model)
	}
	return app
}

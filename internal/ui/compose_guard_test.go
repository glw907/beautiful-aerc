package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
)

// composeGuardCase pairs one of cmd/sketch's fixtures with the App
// state that reaches App.View's real path to the same frame. Both
// halves are literal here rather than reached through
// internal/ui/fixtures: that package imports ui, so ui's own test
// package cannot import it back without a cycle, and unexported App
// fields (theme, sync, toast, banner, stack) are this test's whole
// reason to live in package ui rather than ui_test. The literal
// values mirror internal/ui/fixtures and cmd/sketch's
// sketchStackFixtures exactly; a drift between the two is what this
// test exists to catch.
type composeGuardCase struct {
	name   string
	width  int
	height int
	status StatusLine
	banner Banner
	// seedApp returns an App whose real fields (mail/calendar/...,
	// sync, toast, active surface, stack) reach the same frame
	// fixtureScreen's direct construction reaches.
	seedApp func(th theme.Theme) App
	// fixtureScreen builds the screen ComposeView renders directly,
	// mirroring the corresponding internal/ui/fixtures Build closure.
	fixtureScreen func(th theme.Theme) Screen
	// stack is true when fixtureScreen belongs on ComposeView's stack
	// argument (cmd/sketch's sketchStackFixtures) rather than its
	// active one: the modal-confirm and help fixtures, both screens
	// App pushes rather than switches to.
	stack bool
}

// composeGuardCases is every fixture cmd/sketch's sketchFixtures
// cycles, one composeGuardCase apiece, in the same order.
func composeGuardCases() []composeGuardCase {
	mailZero := store.MailStats{}
	mailLoaded := store.MailStats{Messages: 36102, Mailboxes: 14}

	return []composeGuardCase{
		{
			name: "mail", width: 100, height: 30,
			seedApp:       func(th theme.Theme) App { return App{theme: th, mail: NewMailPlaceholder(th, mailZero)} },
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
		{
			name: "mail-loaded", width: 100, height: 30,
			seedApp:       func(th theme.Theme) App { return App{theme: th, mail: NewMailPlaceholder(th, mailLoaded)} },
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailLoaded) },
		},
		{
			name: "mail-syncing", width: 100, height: 30,
			status: StatusLine{Sync: SyncStateMsg{State: SyncStateSyncing, Done: 4312, Total: 36102}, Spinner: 2},
			seedApp: func(th theme.Theme) App {
				return App{theme: th, mail: NewMailPlaceholder(th, mailZero), sync: SyncStateMsg{State: SyncStateSyncing, Done: 4312, Total: 36102}, spinnerFrame: 2}
			},
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
		{
			name: "mail-offline", width: 100, height: 30,
			status: StatusLine{Sync: SyncStateMsg{State: SyncStateOffline}},
			seedApp: func(th theme.Theme) App {
				return App{theme: th, mail: NewMailPlaceholder(th, mailZero), sync: SyncStateMsg{State: SyncStateOffline}}
			},
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
		{
			name: "mail-backing-off", width: 100, height: 30,
			status: StatusLine{Sync: SyncStateMsg{State: SyncStateBackingOff, Retry: 12}},
			seedApp: func(th theme.Theme) App {
				return App{theme: th, mail: NewMailPlaceholder(th, mailZero), sync: SyncStateMsg{State: SyncStateBackingOff, Retry: 12}}
			},
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
		{
			name: "mail-toast", width: 100, height: 30,
			status: StatusLine{Toast: Toast{Active: true, Label: "3 messages archived", Remaining: 9, Undoable: true}},
			seedApp: func(th theme.Theme) App {
				return App{
					theme: th, mail: NewMailPlaceholder(th, mailZero),
					toastActive: true, toastRemaining: 9,
					toastOffer: UndoOffer{Label: "3 messages archived", Undo: func() tea.Msg { return nil }},
				}
			},
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
		{
			name: "mail-banner", width: 100, height: 30,
			banner: Banner{Active: true, Message: "No keyring found, so your token is stored in a plain file."},
			seedApp: func(th theme.Theme) App {
				return App{
					theme: th, mail: NewMailPlaceholder(th, mailZero),
					banner: Banner{Active: true, Message: "No keyring found, so your token is stored in a plain file."},
				}
			},
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
		{
			name: "modal-confirm", width: 100, height: 30, stack: true,
			seedApp: func(th theme.Theme) App {
				return App{theme: th, mail: NewMailPlaceholder(th, mailZero), stack: []Screen{confirmGuardScreen(th)}}
			},
			fixtureScreen: confirmGuardScreen,
		},
		{
			name: "help", width: 100, height: 30, stack: true,
			seedApp: func(th theme.Theme) App {
				return App{theme: th, mail: NewMailPlaceholder(th, mailZero), stack: []Screen{helpGuardScreen(th)}}
			},
			fixtureScreen: helpGuardScreen,
		},
		{
			name: "calendar", width: 100, height: 30,
			status: StatusLine{Active: SurfaceCalendar},
			seedApp: func(th theme.Theme) App {
				a := App{theme: th, calendar: NewCalendarPlaceholder(th, 42)}
				a.active.Set(a.account, SurfaceCalendar)
				return a
			},
			fixtureScreen: func(th theme.Theme) Screen { return NewCalendarPlaceholder(th, 42) },
		},
		{
			name: "contacts", width: 100, height: 30,
			status: StatusLine{Active: SurfaceContacts},
			seedApp: func(th theme.Theme) App {
				a := App{theme: th, contacts: newContactsPlaceholder(th)}
				a.active.Set(a.account, SurfaceContacts)
				return a
			},
			fixtureScreen: func(th theme.Theme) Screen { return newContactsPlaceholder(th) },
		},
		{
			name: "config", width: 100, height: 30,
			status: StatusLine{Active: SurfaceConfig},
			seedApp: func(th theme.Theme) App {
				a := App{theme: th, config: newConfigPlaceholder(th)}
				a.active.Set(a.account, SurfaceConfig)
				return a
			},
			fixtureScreen: func(th theme.Theme) Screen { return newConfigPlaceholder(th) },
		},
		{
			name: "floor", width: 40, height: 10,
			seedApp:       func(th theme.Theme) App { return App{theme: th, mail: NewMailPlaceholder(th, mailZero)} },
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
		{
			name: "short", width: 100, height: 16,
			seedApp:       func(th theme.Theme) App { return App{theme: th, mail: NewMailPlaceholder(th, mailZero)} },
			fixtureScreen: func(th theme.Theme) Screen { return NewMailPlaceholder(th, mailZero) },
		},
	}
}

// confirmGuardScreen mirrors internal/ui/fixtures.ModalConfirm's
// confirmBuild: F7's pinned quit-with-unsent-messages exemplar.
func confirmGuardScreen(th theme.Theme) Screen {
	return Confirm{
		theme:       th,
		Question:    "Quit with 2 unsent messages?",
		Consequence: "They'll send the next time you open poplar.",
		YesLabel:    "quit",
		NoLabel:     "stay",
	}
}

// helpGuardScreen mirrors internal/ui/fixtures.Help's helpBuild: the
// help overlay opened over the mail placeholder's registered entry.
func helpGuardScreen(th theme.Theme) Screen {
	return HelpScreen{theme: th, Covered: MailPlaceholder{}.Entry(), Title: "Mail"}
}

// TestComposeView_MatchesFixtureEquivalentAppState is the equality
// guard task 2 of pass 2c requires: for every fixture cmd/sketch
// cycles, an App seeded with that fixture's state (seedApp) renders a
// frame byte-identical to ComposeView's own render of the fixture's
// direct construction (fixtureScreen), the same call cmd/sketch
// makes. A revert of cmd/sketch to a direct ui.Render call does not
// touch this test at all; it exists to prove App and ComposeView
// agree, which is what makes cmd/sketch's later call to ComposeView
// meaningful evidence rather than an unchecked assertion.
func TestComposeView_MatchesFixtureEquivalentAppState(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	for _, tc := range composeGuardCases() {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.seedApp(th)
			updated, _ := app.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
			app, ok := updated.(App)
			if !ok {
				t.Fatalf("Update returned %T, want App", updated)
			}

			lm := ComputeLayout(tc.width, tc.height, tc.banner.Active)
			updatedScreen, _ := tc.fixtureScreen(th).Update(LayoutMsg{Layout: lm})
			scr, ok := updatedScreen.(Screen)
			if !ok {
				t.Fatalf("Update returned %T, want Screen", updatedScreen)
			}

			var active Screen
			var stack []Screen
			if tc.stack {
				stack = []Screen{scr}
			} else {
				active = scr
			}
			want := ComposeView(lm, th, tc.status, tc.banner, active, stack)

			got := app.View()
			if got.Content != want.Content {
				t.Errorf("%s: App.View().Content diverged from ComposeView's own frame for the fixture-equivalent state", tc.name)
			}
			if (got.Cursor == nil) != (want.Cursor == nil) {
				t.Errorf("%s: App.View().Cursor = %v, want %v", tc.name, got.Cursor, want.Cursor)
			} else if got.Cursor != nil && *got.Cursor != *want.Cursor {
				t.Errorf("%s: App.View().Cursor = %v, want %v", tc.name, *got.Cursor, *want.Cursor)
			}
		})
	}
}

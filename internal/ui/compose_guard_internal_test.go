package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
)

// TestComposeView_MatchesMailLoadedFixture and
// TestComposeView_MatchesCalendarFixture are
// TestComposeView_MatchesFixtureEquivalentAppState's two exceptions
// (compose_guard_test.go, package ui_test; fix round 1's ruling):
// internal/ui/fixtures.MailLoaded pins a 36,102-message, 14-mailbox
// count and internal/ui/fixtures.Calendar pins a 42-event count,
// neither reachable from a real Init load without seeding that many
// real store rows, which the write-call analyzer (ADR-0003) refuses
// from this package regardless: internal/ui sits on the writer/intent
// rule's read-only side, so no test here, ui_test included, may reach
// store.Writer. Both tests reach App's unexported mail/calendar field
// directly instead. They live in package ui, not ui_test, because
// those unexported fields are the only route in:
// internal/ui/fixtures imports this package, so this package's own
// tests cannot import fixtures back without a cycle, which is also
// why the two fixtures' pinned facts are literals here rather than a
// reference to fixtures.MailLoaded/fixtures.Calendar.
func TestComposeView_MatchesMailLoadedFixture(t *testing.T) {
	th := theme.New(DefaultDark, theme.ProfileTrueColor)
	stats := store.MailStats{Messages: 36102, Mailboxes: 14}

	app := App{theme: th, mail: NewMailPlaceholder(th, stats)}
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app, ok := updated.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", updated)
	}

	lm := ComputeLayout(100, 30, false)
	updatedScreen, _ := NewMailPlaceholder(th, stats).Update(LayoutMsg{Layout: lm})
	scr, ok := updatedScreen.(Screen)
	if !ok {
		t.Fatalf("Update returned %T, want Screen", updatedScreen)
	}
	want := ComposeView(lm, th, StatusLine{}, Banner{}, scr, nil)

	got := app.View()
	if got.Content != want.Content {
		t.Errorf("App.View().Content diverged from ComposeView's own frame for the mail-loaded fixture's state")
	}
	if (got.Cursor == nil) != (want.Cursor == nil) {
		t.Errorf("App.View().Cursor = %v, want %v", got.Cursor, want.Cursor)
	} else if got.Cursor != nil && *got.Cursor != *want.Cursor {
		t.Errorf("App.View().Cursor = %v, want %v", *got.Cursor, *want.Cursor)
	}
}

// TestComposeView_MatchesCalendarFixture is this file's other
// exception; see TestComposeView_MatchesMailLoadedFixture's doc
// comment for why it lives here.
func TestComposeView_MatchesCalendarFixture(t *testing.T) {
	th := theme.New(DefaultDark, theme.ProfileTrueColor)
	const events = 42

	app := App{theme: th, calendar: NewCalendarPlaceholder(th, events)}
	app.active.Set(app.account, SurfaceCalendar)
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app, ok := updated.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", updated)
	}

	lm := ComputeLayout(100, 30, false)
	updatedScreen, _ := NewCalendarPlaceholder(th, events).Update(LayoutMsg{Layout: lm})
	scr, ok := updatedScreen.(Screen)
	if !ok {
		t.Fatalf("Update returned %T, want Screen", updatedScreen)
	}
	want := ComposeView(lm, th, StatusLine{Active: SurfaceCalendar}, Banner{}, scr, nil)

	got := app.View()
	if got.Content != want.Content {
		t.Errorf("App.View().Content diverged from ComposeView's own frame for the calendar fixture's state")
	}
	if (got.Cursor == nil) != (want.Cursor == nil) {
		t.Errorf("App.View().Cursor = %v, want %v", got.Cursor, want.Cursor)
	} else if got.Cursor != nil && *got.Cursor != *want.Cursor {
		t.Errorf("App.View().Cursor = %v, want %v", *got.Cursor, *want.Cursor)
	}
}

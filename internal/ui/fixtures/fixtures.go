// Package fixtures holds internal/ui's pinned, deterministic screen
// states (design decision 10, amendment D): the four surface
// placeholders, already carrying their own store facts, and the two
// layout edge cases the render seam must also prove (the floor state
// and short-height chrome compression), built on the mail
// placeholder, poplar's primary surface (decision 1). Every value
// here is a plain Go value with no I/O, so the gallery sweep and the
// seam's static goldens render them without a store or a running
// terminal.
package fixtures

import (
	"time"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// TZ and Clock pin the fixed zone and instant a fixture builds
// against. TZ is a fixed offset rather than a named IANA zone, so a
// fixture never depends on the machine's own tzdata. No fixture this
// pass renders a date, but a later date-bearing screen fixture pins
// against these same two values rather than inventing its own.
var (
	TZ    = time.FixedZone("AKDT", -8*3600)
	Clock = time.Date(2026, time.August, 19, 9, 41, 0, 0, TZ)
)

// Fixture is one named, deterministic screen: a Build closure over
// facts pinned in this package, plus the status line's own state
// (task 6) the render seam paints into the top band alongside it.
// Status defaults to its zero value, SurfaceMail active and synced,
// so every fixture that does not name a sync state renders the same
// idle band F1 pins.
type Fixture struct {
	Name   string
	Build  func(th theme.Theme) ui.Screen
	Status ui.StatusLine
}

// The pass-2 screen-state fixtures: the four surface placeholders
// (F1/F8), the floor and short-height layout edge cases (both the
// mail placeholder rendered at a size that triggers them), MailLoaded
// (a second mail state with realistic pinned counts), and the SY-5
// sync-state fixtures (task 6): every status the sync segment renders
// distinctly, each swept at truecolor and NO_COLOR only, since the
// content pane beneath them never varies.
var (
	Mail           = Fixture{Name: "mail", Build: mailBuild}
	MailLoaded     = Fixture{Name: "mail-loaded", Build: mailLoadedBuild}
	MailSyncing    = Fixture{Name: "mail-syncing", Build: mailBuild, Status: syncingStatus}
	MailOffline    = Fixture{Name: "mail-offline", Build: mailBuild, Status: offlineStatus}
	MailBackingOff = Fixture{Name: "mail-backing-off", Build: mailBuild, Status: backingOffStatus}
	Calendar       = Fixture{Name: "calendar", Build: calendarBuild, Status: ui.StatusLine{Active: ui.SurfaceCalendar}}
	Contacts       = Fixture{Name: "contacts", Build: contactsBuild, Status: ui.StatusLine{Active: ui.SurfaceContacts}}
	Config         = Fixture{Name: "config", Build: configBuild, Status: ui.StatusLine{Active: ui.SurfaceConfig}}
	Floor          = Fixture{Name: "floor", Build: mailBuild}
	Short          = Fixture{Name: "short", Build: mailBuild}
)

// syncingStatus, offlineStatus, and backingOffStatus pin the sync
// segment's three non-idle SY-5 states (Mail's own zero-value Status
// already pins the fourth, synced): a syncing state with both a
// spinner frame and a known total, an offline state, and a
// backing-off state with its own retry countdown.
var (
	syncingStatus    = ui.StatusLine{Sync: ui.SyncStateMsg{State: ui.SyncStateSyncing, Done: 4312, Total: 36102}, Spinner: 2}
	offlineStatus    = ui.StatusLine{Sync: ui.SyncStateMsg{State: ui.SyncStateOffline}}
	backingOffStatus = ui.StatusLine{Sync: ui.SyncStateMsg{State: ui.SyncStateBackingOff, Retry: 12}}
)

// mailStats is the mail placeholder fixture's pinned store facts:
// zero, matching a freshly opened, unsynced storetest pool exactly,
// so the render seam's App-driven tests (repaint_test.go) can compare
// a real App's own empty-store render against this fixture's
// gallery-committed file byte for byte.
var mailStats = store.MailStats{Messages: 0, Mailboxes: 0}

// mailLoadedStats is MailLoaded's own pinned store facts: a
// realistic loaded account, not the empty one Mail pins, so the
// gallery still commits at least one rendered instance of a grouped
// count (decision 12's "36,102"-style anatomy, formatCount's own
// reason to exist). Built without a store, the same NewMailPlaceholder
// path Mail itself uses.
var mailLoadedStats = store.MailStats{Messages: 36102, Mailboxes: 14}

// calendarEvents is the calendar placeholder fixture's pinned event
// count.
const calendarEvents = 42

func mailBuild(th theme.Theme) ui.Screen {
	return ui.NewMailPlaceholder(th, mailStats)
}

func mailLoadedBuild(th theme.Theme) ui.Screen {
	return ui.NewMailPlaceholder(th, mailLoadedStats)
}

func calendarBuild(th theme.Theme) ui.Screen {
	return ui.NewCalendarPlaceholder(th, calendarEvents)
}

func contactsBuild(th theme.Theme) ui.Screen {
	return themed(ui.ContactsPlaceholder{}, th)
}

func configBuild(th theme.Theme) ui.Screen {
	return themed(ui.ConfigPlaceholder{}, th)
}

// themed applies a ThemeMsg to a placeholder that carries no facts of
// its own, so its zero value plus a theme is its whole starting
// state.
func themed(screen ui.Screen, th theme.Theme) ui.Screen {
	updated, _ := screen.Update(ui.ThemeMsg{Theme: th})
	return updated.(ui.Screen) //nolint:errcheck // a Screen's own Update always returns a Screen; the assertion's panic is the message
}

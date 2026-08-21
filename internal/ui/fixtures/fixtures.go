// Package fixtures holds internal/ui's pinned, deterministic screen
// states (design decision 10, amendment D): the four surface
// placeholders, already carrying their store facts, and the two
// layout edge cases the render seam must also prove (the floor state
// and short-height chrome compression), built on the mail
// placeholder, poplar's primary surface (decision 1). Every value
// here is a plain Go value with no I/O, so the gallery sweep and the
// seam's static goldens render them without a store or a running
// terminal.
package fixtures

import (
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// Fixture is one named, deterministic screen: a Build closure over
// facts pinned in this package, plus the status line's state
// (task 6) and the banner state (task 8) the render seam paints into
// the top bands alongside it. Status and Banner both default to their
// zero values, SurfaceMail active and synced with no banner showing,
// so every fixture that does not name either renders the same idle
// band F1 pins.
type Fixture struct {
	Name   string
	Build  func(th theme.Theme) ui.Screen
	Status ui.StatusLine
	Banner ui.Banner
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
	MailToast      = Fixture{Name: "mail-toast", Build: mailBuild, Status: toastStatus}
	MailBanner     = Fixture{Name: "mail-banner", Build: mailBuild, Banner: bannerState}
	ModalConfirm   = Fixture{Name: "modal-confirm", Build: confirmBuild}
	Help           = Fixture{Name: "help", Build: helpBuild}
	Calendar       = Fixture{Name: "calendar", Build: calendarBuild, Status: ui.StatusLine{Active: ui.SurfaceCalendar}}
	Contacts       = Fixture{Name: "contacts", Build: contactsBuild, Status: ui.StatusLine{Active: ui.SurfaceContacts}}
	Config         = Fixture{Name: "config", Build: configBuild, Status: ui.StatusLine{Active: ui.SurfaceConfig}}
	Floor          = Fixture{Name: "floor", Build: mailBuild}
	Short          = Fixture{Name: "short", Build: mailBuild}
)

// syncingStatus, offlineStatus, and backingOffStatus pin the sync
// segment's three non-idle SY-5 states (Mail's zero-value Status
// already pins the fourth, synced): a syncing state with both a
// spinner frame and a known total, an offline state, and a
// backing-off state with its retry countdown.
var (
	syncingStatus    = ui.StatusLine{Sync: ui.SyncStateMsg{State: ui.SyncStateSyncing, Done: 4312, Total: 36102}, Spinner: 2}
	offlineStatus    = ui.StatusLine{Sync: ui.SyncStateMsg{State: ui.SyncStateOffline}}
	backingOffStatus = ui.StatusLine{Sync: ui.SyncStateMsg{State: ui.SyncStateBackingOff, Retry: 12}}
)

// toastStatus pins task 8's toast fixture: the UX-9 undo
// presentation mid-countdown, over an otherwise-idle synced band.
// Undoable is true (task-8-findings-r1.md F8): the fixture's
// offer actually has an undo to advertise, so the golden shows the
// hint rather than a dead `u`.
var toastStatus = ui.StatusLine{Toast: ui.Toast{Active: true, Label: "3 messages archived", Remaining: 9, Undoable: true}}

// bannerState pins task 8's banner fixture: the design language's
// pinned exemplar copy (banner_strip), a warning with no keyring
// backing the credential store.
var bannerState = ui.Banner{Active: true, Message: "No keyring found, so your token is stored in a plain file."}

// mailStats is the mail placeholder fixture's pinned store facts:
// zero, matching a freshly opened, unsynced storetest pool exactly,
// so the render seam's App-driven tests (repaint_test.go) can compare
// a real App's empty-store render against this fixture's
// gallery-committed file byte for byte.
var mailStats = store.MailStats{Messages: 0, Mailboxes: 0}

// mailLoadedStats is MailLoaded's pinned store facts: a
// realistic loaded account, not the empty one Mail pins, so the
// gallery still commits at least one rendered instance of a grouped
// count (decision 12's "36,102"-style anatomy, formatCount's
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

// confirmBuild pins task 8's modal fixture: the design language's
// pinned exemplar copy (modal()), a quit confirm over unsent
// messages. YesCmd/NoCmd stay nil; the gallery renders Confirm's
// composition, not its answer behavior.
func confirmBuild(th theme.Theme) ui.Screen {
	return themed(ui.Confirm{
		Question:    "Quit with 2 unsent messages?",
		Consequence: "They'll send the next time you open poplar.",
		YesLabel:    "quit",
		NoLabel:     "stay",
	}, th)
}

// helpBuild pins the help overlay fixture: opened over the mail
// placeholder's registered entry, the pinned exemplar's
// primary surface (decision 1).
func helpBuild(th theme.Theme) ui.Screen {
	return themed(ui.HelpScreen{Covered: ui.MailPlaceholder{}.Entry()}, th)
}

// themed applies a ThemeMsg to a placeholder that carries no facts,
// so its zero value plus a theme is its whole starting state.
func themed(screen ui.Screen, th theme.Theme) ui.Screen {
	updated, _ := screen.Update(ui.ThemeMsg{Theme: th})
	return updated.(ui.Screen) //nolint:errcheck // a Screen's Update always returns a Screen; the assertion's panic is the message
}

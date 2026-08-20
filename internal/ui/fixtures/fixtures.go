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
	"fmt"
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
// facts pinned in this package. Build takes the theme rather than
// baking one in, so the gallery sweep can theme and lay out the same
// starting state however many profiles it needs, with no I/O and no
// mutation of anything package-level.
type Fixture struct {
	Name  string
	Build func(th theme.Theme) ui.Screen
}

// The six pass-2 screen-state fixtures: the four surface
// placeholders (F1/F8), and the floor and short-height layout edge
// cases, both the mail placeholder rendered at a size that triggers
// them.
var (
	Mail     = Fixture{Name: "mail", Build: mailBuild}
	Calendar = Fixture{Name: "calendar", Build: calendarBuild}
	Contacts = Fixture{Name: "contacts", Build: contactsBuild}
	Config   = Fixture{Name: "config", Build: configBuild}
	Floor    = Fixture{Name: "floor", Build: mailBuild}
	Short    = Fixture{Name: "short", Build: mailBuild}
)

// mailStats is the mail placeholder fixture's pinned store facts:
// round, obviously-fabricated numbers, never a real account's.
var mailStats = store.MailStats{Messages: 12406, Mailboxes: 8}

// calendarEvents is the calendar placeholder fixture's pinned event
// count.
const calendarEvents = 42

func mailBuild(th theme.Theme) ui.Screen {
	return ui.NewMailPlaceholder(th, mailStats)
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
	scr, ok := updated.(ui.Screen)
	if !ok {
		panic(fmt.Sprintf("fixtures: %T's own Update returned a non-Screen tea.Model", screen))
	}
	return scr
}

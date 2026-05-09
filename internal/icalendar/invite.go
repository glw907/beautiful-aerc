// Package icalendar parses iCalendar (.ics) data into value types for display.
package icalendar

import (
	"bytes"
	"errors"
	"strings"
	"time"

	ical "github.com/arran4/golang-ical"
)

// ErrNoEvent is returned when the input contains no VEVENT component.
var ErrNoEvent = errors.New("icalendar: no VEVENT in part")

// Invite holds the display-relevant fields extracted from the first VEVENT
// in an iCalendar document.
type Invite struct {
	Summary       string
	Start, End    time.Time
	Location      string
	Organizer     string
	AttendeeCount int
	Method        string // REQUEST, PUBLISH, CANCEL, … (uppercased)
	Recurrence    string // humanized one-liner; "" if none or complex
}

// ParseInvite decodes b as iCalendar data and returns the first VEVENT.
// Empty input or missing VEVENT returns ErrNoEvent.
func ParseInvite(b []byte) (Invite, error) {
	if len(b) == 0 {
		return Invite{}, ErrNoEvent
	}

	cal, err := ical.ParseCalendar(bytes.NewReader(b))
	if err != nil {
		return Invite{}, ErrNoEvent
	}

	events := cal.Events()
	if len(events) == 0 {
		return Invite{}, ErrNoEvent
	}

	ev := events[0]
	inv := Invite{
		Method:        calendarMethod(cal),
		AttendeeCount: len(ev.Attendees()),
	}

	if p := ev.GetProperty(ical.ComponentPropertySummary); p != nil {
		inv.Summary = strings.TrimSpace(p.Value)
	}
	if inv.Summary == "" {
		inv.Summary = "(no title)"
	}

	if p := ev.GetProperty(ical.ComponentPropertyLocation); p != nil {
		inv.Location = strings.TrimSpace(p.Value)
	}

	inv.Organizer = organizerName(ev)
	inv.Start, _ = parseEventTime(ev, ical.ComponentPropertyDtStart)
	inv.End, _ = parseEventTime(ev, ical.ComponentPropertyDtEnd)

	if p := ev.GetProperty(ical.ComponentPropertyRrule); p != nil {
		inv.Recurrence = humanizeRRULE(p.Value)
	}

	return inv, nil
}

func calendarMethod(cal *ical.Calendar) string {
	for _, cp := range cal.CalendarProperties {
		if cp.IANAToken == string(ical.PropertyMethod) {
			return strings.ToUpper(cp.Value)
		}
	}
	return ""
}

func organizerName(ev *ical.VEvent) string {
	p := ev.GetProperty(ical.ComponentPropertyOrganizer)
	if p == nil {
		return ""
	}
	if cns, ok := p.ICalParameters["CN"]; ok && len(cns) > 0 && cns[0] != "" {
		return cns[0]
	}
	addr := p.Value
	if lower := strings.ToLower(addr); strings.HasPrefix(lower, "mailto:") {
		addr = addr[len("mailto:"):]
	}
	return addr
}

func parseEventTime(ev *ical.VEvent, prop ical.ComponentProperty) (time.Time, error) {
	// Try datetime first, then all-day date.
	t, err := ev.GetStartAt()
	if prop == ical.ComponentPropertyDtEnd {
		t, err = ev.GetEndAt()
	}
	if err == nil {
		return t, nil
	}
	if prop == ical.ComponentPropertyDtStart {
		return ev.GetAllDayStartAt()
	}
	return ev.GetAllDayEndAt()
}

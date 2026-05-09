package compose

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrUnrecognized reports input that didn't match any accepted shape.
var ErrUnrecognized = errors.New("compose: not a recognized date — try \"tomorrow 3pm\" or \"2026-05-15 09:00\"")

// ParseSchedule parses a user-typed schedule string against now.
// Accepts ISO/US/English dates, time-only strings (today or rolled
// to tomorrow), keyword shortcuts (tomorrow, tonight, next <day>,
// <day>), and offsets (+Nm/+Nh/+Nd). Year defaults to now.Year() and
// past results roll forward by one unit.
func ParseSchedule(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrUnrecognized
	}
	lower := strings.ToLower(s)

	if t, ok := parseRelative(lower, now); ok {
		return t, nil
	}
	if t, ok := parseKeyword(lower, now); ok {
		return t, nil
	}
	return parseLayouts(s, now)
}

var relRe = regexp.MustCompile(`^\+(\d+)([mhd])$`)

func parseRelative(s string, now time.Time) (time.Time, bool) {
	m := relRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "m":
		return now.Add(time.Duration(n) * time.Minute), true
	case "h":
		return now.Add(time.Duration(n) * time.Hour), true
	case "d":
		return now.Add(time.Duration(n) * 24 * time.Hour), true
	}
	return time.Time{}, false
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

func parseKeyword(s string, now time.Time) (time.Time, bool) {
	day, timePart := splitTimeTail(s)

	hh, mm, hasTime := parseTimeOnly(timePart)
	if timePart != "" && !hasTime {
		return time.Time{}, false
	}

	defH, defM := 9, 0

	switch {
	case day == "tomorrow":
		t := now.AddDate(0, 0, 1)
		return AtHM(t, pick(hh, defH, hasTime), pick(mm, defM, hasTime)), true
	case day == "tonight":
		t := AtHM(now, 21, 0)
		if t.Before(now) {
			t = t.AddDate(0, 0, 1)
		}
		return t, true
	}

	if rest, ok := strings.CutPrefix(day, "next "); ok {
		if wd, ok := weekdays[rest]; ok {
			t := NextWeekday(now, wd, true)
			return AtHM(t, pick(hh, defH, hasTime), pick(mm, defM, hasTime)), true
		}
	}
	if wd, ok := weekdays[day]; ok {
		t := NextWeekday(now, wd, false)
		return AtHM(t, pick(hh, defH, hasTime), pick(mm, defM, hasTime)), true
	}

	return time.Time{}, false
}

// splitTimeTail returns (head, tail) split on the first space whose
// suffix parses as a time-only string. Falls back to (s, "").
func splitTimeTail(s string) (head, tail string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != ' ' {
			continue
		}
		if _, _, ok := parseTimeOnly(s[i+1:]); ok {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
		}
	}
	return s, ""
}

var timeOnlyLayouts = []string{"15:04", "3:04 PM", "3:04PM", "3 PM", "3PM", "3pm", "3am"}

func parseTimeOnly(s string) (hh, mm int, ok bool) {
	if s == "" {
		return 0, 0, false
	}
	upper := strings.ToUpper(s)
	for _, layout := range timeOnlyLayouts {
		if t, err := time.Parse(layout, upper); err == nil {
			return t.Hour(), t.Minute(), true
		}
	}
	return 0, 0, false
}

func AtHM(t time.Time, h, m int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, t.Location())
}

func pick(v, def int, has bool) int {
	if has {
		return v
	}
	return def
}

// NextWeekday returns the next date at midnight whose weekday matches wd.
// If skipThisWeek, advances by another 7 days.
func NextWeekday(now time.Time, wd time.Weekday, skipThisWeek bool) time.Time {
	delta := (int(wd) - int(now.Weekday()) + 7) % 7
	if delta == 0 {
		delta = 7
	}
	t := now.AddDate(0, 0, delta)
	if skipThisWeek {
		t = t.AddDate(0, 0, 7)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// dateLayouts: layouts without a year (no "2006") default to now.Year()
// and roll forward one year when the result is in the past.
var dateLayouts = []struct {
	layout  string
	hasYear bool
	hasTime bool
}{
	{"2006-01-02 15:04", true, true},
	{"2006-01-02 3:04 PM", true, true},
	{"2006-01-02 3 PM", true, true},
	{"2006-01-02 3pm", true, true},
	{"2006-01-02", true, false},
	{"01/02/2006 15:04", true, true},
	{"01/02/2006 3:04 PM", true, true},
	{"01/02/2006", true, false},
	{"01/02 15:04", false, true},
	{"1/2 15:04", false, true},
	{"01/02 3:04 PM", false, true},
	{"1/2 3:04 PM", false, true},
	{"01/02 3pm", false, true},
	{"1/2 3pm", false, true},
	{"01/02 3am", false, true},
	{"1/2 3am", false, true},
	{"01/02", false, false},
	{"1/2", false, false},
	{"Jan 2 2006 15:04", true, true},
	{"Jan 2 2006", true, false},
	{"Jan 2 15:04", false, true},
	{"Jan 2 3:04 PM", false, true},
	{"Jan 2 3pm", false, true},
	{"Jan 2", false, false},
	{"2 Jan 2006 15:04", true, true},
	{"2 Jan 2006", true, false},
	{"2 Jan 15:04", false, true},
	{"2 Jan 3pm", false, true},
	{"2 Jan", false, false},
}

func parseLayouts(s string, now time.Time) (time.Time, error) {
	if t, ok := parseTimeAlone(s, now); ok {
		return t, nil
	}
	for _, l := range dateLayouts {
		t, err := time.ParseInLocation(l.layout, s, now.Location())
		if err != nil {
			continue
		}
		if !l.hasYear {
			t = time.Date(now.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), 0, 0, now.Location())
			if t.Before(now) {
				t = t.AddDate(1, 0, 0)
			}
		}
		return t, nil
	}
	return time.Time{}, ErrUnrecognized
}

// parseTimeAlone matches H[:MM][am/pm] / HH:MM and rolls to tomorrow
// when the result is before now.
func parseTimeAlone(s string, now time.Time) (time.Time, bool) {
	hh, mm, ok := parseTimeOnly(s)
	if !ok {
		return time.Time{}, false
	}
	t := AtHM(now, hh, mm)
	if t.Before(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t, true
}

package icalendar

import (
	"strconv"
	"strings"
)

// humanizeRRULE converts a raw RRULE value string to a brief English phrase.
// Only trivial rules (FREQ + optional INTERVAL, no other keys) are humanized;
// anything more complex returns "".
func humanizeRRULE(raw string) string {
	parts := strings.Split(raw, ";")
	freq := ""
	interval := 1
	for _, part := range parts {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(k) {
		case "FREQ":
			freq = strings.ToUpper(v)
		case "INTERVAL":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return ""
			}
			interval = n
		default:
			// Any other key (BYDAY, COUNT, UNTIL, …) makes it complex.
			return ""
		}
	}

	unit, ok := freqUnit[freq]
	if !ok {
		return ""
	}
	if interval == 1 {
		return "Every " + unit[0]
	}
	return "Every " + strconv.Itoa(interval) + " " + unit[1]
}

// freqUnit maps FREQ value → [singular label, plural label].
var freqUnit = map[string][2]string{
	"DAILY":   {"day", "days"},
	"WEEKLY":  {"week", "weeks"},
	"MONTHLY": {"month", "months"},
	"YEARLY":  {"year", "years"},
}

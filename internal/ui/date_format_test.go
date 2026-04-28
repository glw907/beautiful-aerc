// SPDX-License-Identifier: MIT

package ui

import (
	"testing"
	"time"
)

func TestFormatRelativeDate(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC) // Mon 2026-04-13 noon
	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{"same day morning", time.Date(2026, 4, 13, 9, 45, 0, 0, time.UTC), "9:45 AM"},
		{"same day two-digit hour", time.Date(2026, 4, 13, 10, 23, 0, 0, time.UTC), "10:23 AM"},
		{"same day evening", time.Date(2026, 4, 13, 22, 15, 0, 0, time.UTC), "10:15 PM"},
		{"yesterday", time.Date(2026, 4, 12, 15, 47, 0, 0, time.UTC), "Sun 2026-04-12"},
		{"this week wednesday", time.Date(2026, 4, 8, 16, 22, 0, 0, time.UTC), "Wed 2026-04-08"},
		{"months ago", time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC), "Thu 2025-12-25"},
		{"zero time", time.Time{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatRelativeDate(tc.in, now); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Verifies the same-day branch honors now's location rather than the
// t value's location — a message whose UTC timestamp rolls past
// midnight should still read as "today" in a user west of UTC.
func TestFormatRelativeDateRespectsNowLocation(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	// 2026-04-13 23:30 LA time = 2026-04-14 06:30 UTC.
	nowLA := time.Date(2026, 4, 13, 23, 30, 0, 0, la)
	msgUTC := time.Date(2026, 4, 14, 6, 45, 0, 0, time.UTC) // same LA day
	got := formatRelativeDate(msgUTC, nowLA)
	if got != "11:45 PM" {
		t.Errorf("got %q, want %q", got, "11:45 PM")
	}
}

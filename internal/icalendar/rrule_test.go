package icalendar

import "testing"

func TestHumanizeRRULE(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"daily", "FREQ=DAILY", "Every day"},
		{"daily interval 1", "FREQ=DAILY;INTERVAL=1", "Every day"},
		{"daily interval 3", "FREQ=DAILY;INTERVAL=3", "Every 3 days"},
		{"weekly", "FREQ=WEEKLY", "Every week"},
		{"weekly interval 1", "FREQ=WEEKLY;INTERVAL=1", "Every week"},
		{"weekly interval 2", "FREQ=WEEKLY;INTERVAL=2", "Every 2 weeks"},
		{"monthly", "FREQ=MONTHLY", "Every month"},
		{"monthly interval 6", "FREQ=MONTHLY;INTERVAL=6", "Every 6 months"},
		{"yearly", "FREQ=YEARLY", "Every year"},
		{"yearly interval 2", "FREQ=YEARLY;INTERVAL=2", "Every 2 years"},
		{"byday drops", "FREQ=WEEKLY;BYDAY=MO,WE,FR", ""},
		{"count drops", "FREQ=DAILY;COUNT=5", ""},
		{"until drops", "FREQ=DAILY;UNTIL=20261231T000000Z", ""},
		{"bymonthday drops", "FREQ=MONTHLY;BYMONTHDAY=1", ""},
		{"bymonth drops", "FREQ=YEARLY;BYMONTH=3", ""},
		{"wkst drops", "FREQ=WEEKLY;WKST=MO", ""},
		{"bysetpos drops", "FREQ=MONTHLY;BYDAY=MO;BYSETPOS=1", ""},
		{"unknown freq", "FREQ=HOURLY", ""},
		{"empty", "", ""},
		{"no freq", "INTERVAL=2", ""},
		{"bad interval", "FREQ=DAILY;INTERVAL=abc", ""},
		{"zero interval", "FREQ=DAILY;INTERVAL=0", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := humanizeRRULE(tt.raw)
			if got != tt.want {
				t.Errorf("humanizeRRULE(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

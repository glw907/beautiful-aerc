// internal/compose/scheduleparse_test.go
package compose

import (
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	now := time.Date(2026, 5, 9, 14, 30, 0, 0, time.Local) // Sat
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2026-05-15 09:00", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"2026-05-15 9 AM", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"2026-05-15", time.Date(2026, 5, 15, 0, 0, 0, 0, time.Local)},
		{"05/15 09:00", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"5/15 9am", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"05/15/2026 09:00", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"May 15 9am", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"15 May 9am", time.Date(2026, 5, 15, 9, 0, 0, 0, time.Local)},
		{"09:00", time.Date(2026, 5, 10, 9, 0, 0, 0, time.Local)}, // past today → tomorrow
		{"15:00", time.Date(2026, 5, 9, 15, 0, 0, 0, time.Local)},
		{"3pm", time.Date(2026, 5, 9, 15, 0, 0, 0, time.Local)},
		{"tomorrow", time.Date(2026, 5, 10, 9, 0, 0, 0, time.Local)},
		{"tomorrow 3pm", time.Date(2026, 5, 10, 15, 0, 0, 0, time.Local)},
		{"tonight", time.Date(2026, 5, 9, 21, 0, 0, 0, time.Local)},
		{"next monday", time.Date(2026, 5, 18, 9, 0, 0, 0, time.Local)}, // next-week monday
		{"monday", time.Date(2026, 5, 11, 9, 0, 0, 0, time.Local)},      // first upcoming mon
		{"monday 8am", time.Date(2026, 5, 11, 8, 0, 0, 0, time.Local)},
		{"+30m", now.Add(30 * time.Minute)},
		{"+2h", now.Add(2 * time.Hour)},
		{"+3d", now.Add(72 * time.Hour)},
		{"03/05", time.Date(2027, 3, 5, 0, 0, 0, 0, time.Local)}, // past in current year → next year
	}
	for _, c := range cases {
		got, err := ParseSchedule(c.in, now)
		if err != nil {
			t.Errorf("%q: err %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("%q: got %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseSchedule_Errors(t *testing.T) {
	now := time.Date(2026, 5, 9, 14, 30, 0, 0, time.Local)
	for _, in := range []string{"", "garbage", "32:00", "13/40"} {
		if _, err := ParseSchedule(in, now); err == nil {
			t.Errorf("%q: want error, got nil", in)
		}
	}
}

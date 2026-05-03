// SPDX-License-Identifier: MIT

package ui

import (
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestFormatRelativeDateCompact(t *testing.T) {
	now := time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"now <5min", now.Add(-3 * time.Minute), "now"},
		{"5m", now.Add(-5 * time.Minute), "5m "},
		{"30m", now.Add(-30 * time.Minute), "30m"},
		{"59m", now.Add(-59 * time.Minute), "59m"},
		{"1h", now.Add(-1 * time.Hour), "1h "},
		{"23h", now.Add(-23 * time.Hour), "23h"},
		{"1d", now.AddDate(0, 0, -1), "1d "},
		{"6d", now.AddDate(0, 0, -6), "6d "},
		{"1w", now.AddDate(0, 0, -8), "1w "},
		{"3w", now.AddDate(0, 0, -23), "3w "},
		{"month same year", time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC), "Jan"},
		{"prior year", time.Date(2024, 7, 4, 9, 0, 0, 0, time.UTC), "'24"},
		{"zero time", time.Time{}, "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeDateCompact(tt.t, now)
			if got != tt.want {
				t.Errorf("formatRelativeDateCompact() = %q, want %q", got, tt.want)
			}
			if len(got) != 3 {
				t.Errorf("formatRelativeDateCompact() length = %d, want 3 (got %q)", len(got), got)
			}
		})
	}
}

func TestFormatRelativeDateShort(t *testing.T) {
	now := time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"same day morning", time.Date(2026, 5, 2, 9, 5, 0, 0, time.UTC), "9:05a"},
		{"same day afternoon", time.Date(2026, 5, 2, 15, 41, 0, 0, time.UTC), "3:41p"},
		{"same day midnight hour", time.Date(2026, 5, 2, 0, 30, 0, 0, time.UTC), "12:30a"},
		{"yesterday", now.AddDate(0, 0, -1), "05-01"},
		{"prior month", time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC), "04-30"},
		{"prior year", time.Date(2024, 7, 4, 9, 0, 0, 0, time.UTC), "07-04"},
		{"zero time", time.Time{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeDateShort(tt.t, now)
			if got != tt.want {
				t.Errorf("formatRelativeDateShort() = %q, want %q", got, tt.want)
			}
			if len(got) > 6 {
				t.Errorf("formatRelativeDateShort() length = %d, want ≤6", len(got))
			}
		})
	}
}

func TestDisplayDate_SelectsByWidth(t *testing.T) {
	now := time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC)
	when := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	msg := mail.MessageInfo{SentAt: when}

	if got := displayDate(msg, now, 0); got != "" {
		t.Errorf("displayDate(width=0) = %q, want empty", got)
	}
	if got := displayDate(msg, now, 3); got != "2d " {
		t.Errorf("displayDate(width=3) = %q, want %q", got, "2d ")
	}
	if got := displayDate(msg, now, 5); got != "04-30" {
		t.Errorf("displayDate(width=5) = %q, want %q", got, "04-30")
	}
}

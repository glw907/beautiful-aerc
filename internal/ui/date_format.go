// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// formatRelativeDateCompact returns a 3-cell relative date string,
// always right-padded to width 3. Used in Intermediate-tier message
// lists (W=90–99) where date awareness matters but cells are scarce.
//
//	now (last 5 min) → "now"
//	<1 hour          → "5m ", "59m"
//	<1 day           → "1h ", "23h"
//	<1 week          → "1d ", "6d "
//	<1 month         → "1w ", "3w "
//	same year        → "Jan", "Dec"
//	prior years      → "'24", "'25"
//	zero time        → "   " (three spaces)
func formatRelativeDateCompact(t, now time.Time) string {
	if t.IsZero() {
		return "   "
	}
	t = t.In(now.Location())
	delta := now.Sub(t)
	switch {
	case delta < 5*time.Minute && delta >= 0:
		return "now"
	case delta < time.Hour:
		return padRight(fmt.Sprintf("%dm", int(delta.Minutes())), 3)
	case delta < 24*time.Hour:
		return padRight(fmt.Sprintf("%dh", int(delta.Hours())), 3)
	case delta < 7*24*time.Hour:
		return padRight(fmt.Sprintf("%dd", int(delta.Hours()/24)), 3)
	case delta < 28*24*time.Hour:
		return padRight(fmt.Sprintf("%dw", int(delta.Hours()/(24*7))), 3)
	case t.Year() == now.Year():
		return t.Format("Jan")
	default:
		yy := t.Year() % 100
		return fmt.Sprintf("'%02d", yy)
	}
}

// formatRelativeDateShort returns a 5-cell short date string. Same-day
// values render as 12-hour time with a single-letter AM/PM suffix
// ("3:41p", "9:05a"). Other days render as "MM-DD". Zero time → empty.
//
// Note: the 12-hour display can run 6 cells for `12:30a` etc. The
// caller's column truncation handles that. The 5-cell budget is the
// design target, not a hard cap.
func formatRelativeDateShort(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(now.Location())
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	if ty == ny && tm == nm && td == nd {
		hour := t.Hour() % 12
		if hour == 0 {
			hour = 12
		}
		ap := "a"
		if t.Hour() >= 12 {
			ap = "p"
		}
		return fmt.Sprintf("%d:%02d%s", hour, t.Minute(), ap)
	}
	return t.Format("01-02")
}

// displayDate returns the date string for a message row at the given
// column width. Width 0 means "no date column" (caller skips the
// column entirely). Width 3 selects the compact relative format;
// width 5 selects the short absolute format. Other widths are
// treated as 5 (defensive, should not happen if LayoutMode is the
// only caller).
func displayDate(msg mail.MessageInfo, now time.Time, width int) string {
	if width == 0 {
		return ""
	}
	t := msg.SentAt
	if t.IsZero() {
		// Legacy fixtures predating SentAt. Return the wire string
		// as-is. Width-aware truncation is the caller's job.
		return msg.Date
	}
	switch width {
	case 3:
		return formatRelativeDateCompact(t, now)
	default:
		return formatRelativeDateShort(t, now)
	}
}

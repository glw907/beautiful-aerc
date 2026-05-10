package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/mail"
)

// padRight right-pads s with spaces to width display cells. Inputs are
// plain text (no ANSI), so lipgloss.Width measures correctly.
func padRight(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

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
// ("3:41p", "9:05a"); other days render as "MM-DD"; zero time renders
// empty. The 12-hour display can spill to 6 cells for "12:30a"; the
// caller's column truncation handles that.
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

// displayDate returns the date string for a message row. Width 0 hides
// the column, 3 selects compact relative, 5 selects short absolute.
// Other widths fall through to short absolute as a safety net.
func displayDate(msg mail.MessageInfo, now time.Time, width int) string {
	if width == 0 {
		return ""
	}
	t := msg.SentAt
	if t.IsZero() {
		// Legacy fixtures predating SentAt return the wire string as-is.
		return msg.Date
	}
	switch width {
	case 3:
		return formatRelativeDateCompact(t, now)
	default:
		return formatRelativeDateShort(t, now)
	}
}

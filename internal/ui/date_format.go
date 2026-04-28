// SPDX-License-Identifier: MIT

package ui

import (
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// formatRelativeDate renders t as a display string relative to now.
//
//   - Same calendar day as now: 12-hour time, e.g. "10:23 AM".
//   - Any other day: "Mon 2006-01-02" (3-letter weekday + ISO date).
//   - Zero time: empty string.
//
// Both the same-day comparison and the returned string are in now's
// location — a caller passing a UTC `t` with a local `now` gets back
// a local-time string. This matches the common case (backend wire
// format is UTC, user wants local display) without forcing callers
// to convert beforehand.
func formatRelativeDate(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(now.Location())
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	if ty == ny && tm == nm && td == nd {
		return t.Format("3:04 PM")
	}
	return t.Format("Mon 2006-01-02")
}

// displayDate picks the right date string for a message row. When a
// backend populates SentAt, the UI is the single source of truth for
// date formatting via formatRelativeDate. The msg.Date fallback is
// only for legacy unit-test fixtures that never set SentAt; real
// workers are expected to always fill SentAt.
func displayDate(msg mail.MessageInfo, now time.Time) string {
	if !msg.SentAt.IsZero() {
		return formatRelativeDate(msg.SentAt, now)
	}
	return msg.Date
}

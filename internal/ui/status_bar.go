// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ConnectionState represents the mail connection status.
type ConnectionState int

const (
	Connected    ConnectionState = iota
	Offline
	Reconnecting
)

// StatusMode is the left-side content shown in the status bar.
// Account mode shows message + unread counts. Viewer mode shows the
// scroll percentage of the open message body.
type StatusMode int

const (
	StatusAccount StatusMode = iota
	StatusViewer
)

// StatusBar renders the bottom frame edge with combined status indicator.
type StatusBar struct {
	styles    Styles
	total     int
	unread    int
	scrollPct int
	mode      StatusMode
	connState ConnectionState
	// Outbox segment: rendered between counts and the connection
	// indicator. Hidden when both counts are zero.
	outboxInflight int // pending + executing + failed
	outboxConflict int // conflict only
}

// NewStatusBar creates a StatusBar with the given styles.
func NewStatusBar(styles Styles) StatusBar {
	return StatusBar{
		styles:    styles,
		connState: Connected,
	}
}

func (sb StatusBar) SetCounts(total, unread int) StatusBar {
	sb.total = total
	sb.unread = unread
	return sb
}

func (sb StatusBar) SetConnectionState(state ConnectionState) StatusBar {
	sb.connState = state
	return sb
}

func (sb StatusBar) ConnectionState() ConnectionState { return sb.connState }

// SetOutboxDepth returns a copy of sb with the outbox segment counts
// updated. inflight = pending + executing + failed. conflict is rendered
// separately so the warning glyph dominates.
func (sb StatusBar) SetOutboxDepth(inflight, conflict int) StatusBar {
	if inflight < 0 {
		inflight = 0
	}
	if conflict < 0 {
		conflict = 0
	}
	sb.outboxInflight = inflight
	sb.outboxConflict = conflict
	return sb
}

func (sb StatusBar) SetMode(mode StatusMode) StatusBar {
	sb.mode = mode
	return sb
}

// Only meaningful when mode == StatusViewer.
func (sb StatusBar) SetScrollPct(pct int) StatusBar {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	sb.scrollPct = pct
	return sb
}

// buildFill creates a horizontal line of width chars with ┴ at dividerCol.
func buildFill(width, dividerCol int) string {
	var buf strings.Builder
	buf.Grow(width * 3) // UTF-8 box-drawing chars are 3 bytes
	for i := 0; i < width; i++ {
		if dividerCol > 0 && i == dividerCol {
			buf.WriteRune('┴')
		} else {
			buf.WriteRune('─')
		}
	}
	return buf.String()
}

// View renders the status bar at the given width. dividerCol is the
// column position of the panel divider (0 to skip the junction).
func (sb StatusBar) View(width, dividerCol int) string {
	var counts string
	switch sb.mode {
	case StatusViewer:
		counts = fmt.Sprintf("%d%%", sb.scrollPct)
	default:
		counts = fmt.Sprintf("%d messages", sb.total)
		if sb.unread > 0 {
			counts += fmt.Sprintf(" · %d unread", sb.unread)
		}
	}

	var connIcon, connText string
	var connStyle lipgloss.Style
	switch sb.connState {
	case Connected:
		connIcon = "●"
		connText = "connected"
		connStyle = sb.styles.StatusConnected
	case Offline:
		connIcon = "○"
		connText = "offline"
		connStyle = sb.styles.StatusOffline
	case Reconnecting:
		connIcon = "◐"
		connText = "reconnecting"
		connStyle = sb.styles.StatusReconnect
	}

	// Build the styled segments first, then measure them with the
	// same primitive (lipgloss.Width) used everywhere else. Measuring
	// the plain-text template with a different primitive caused
	// rounding drift on the connection icons and forced a corrective
	// re-render.
	countsPart := sb.styles.StatusBar.Render(" " + counts + " · ")
	outboxPart := sb.renderOutboxSegment()
	connIconPart := connStyle.Render(connIcon)
	connTextPart := sb.styles.StatusBar.Render(" " + connText + " ")
	endPart := sb.styles.TopLine.Render("─╯")
	rightWidth := lipgloss.Width(countsPart) + lipgloss.Width(outboxPart) +
		lipgloss.Width(connIconPart) + lipgloss.Width(connTextPart) +
		lipgloss.Width(endPart)

	fillWidth := max(0, width-rightWidth)
	fillPart := sb.styles.TopLine.Render(buildFill(fillWidth, dividerCol))
	return fillPart + countsPart + outboxPart + connIconPart + connTextPart + endPart
}

// renderOutboxSegment formats the outbox depth indicator. Returns the
// empty string when both counts are zero (segment omitted).
func (sb StatusBar) renderOutboxSegment() string {
	if sb.outboxInflight == 0 && sb.outboxConflict == 0 {
		return ""
	}
	if sb.outboxConflict > 0 {
		total := sb.outboxInflight + sb.outboxConflict
		glyph := sb.styles.StatusOffline.Render("⚠")
		body := sb.styles.StatusBar.Render(strconv.Itoa(total) + " · ")
		return glyph + body
	}
	glyph := sb.styles.StatusBar.Render("⇅")
	body := sb.styles.StatusBar.Render(strconv.Itoa(sb.outboxInflight) + " · ")
	return glyph + body
}

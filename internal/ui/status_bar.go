package ui

import (
	"fmt"
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

// StatusBar renders the bottom frame edge with combined status indicator.
type StatusBar struct {
	styles    Styles
	total     int
	unread    int
	connState ConnectionState
}

// NewStatusBar creates a StatusBar with the given styles.
func NewStatusBar(styles Styles) StatusBar {
	return StatusBar{
		styles:    styles,
		connState: Connected,
	}
}

// SetCounts updates the message and unread counts.
func (sb *StatusBar) SetCounts(total, unread int) {
	sb.total = total
	sb.unread = unread
}

// SetConnected sets the connection state to connected or offline.
func (sb *StatusBar) SetConnected(connected bool) {
	if connected {
		sb.connState = Connected
	} else {
		sb.connState = Offline
	}
}

// SetConnectionState sets the connection state directly.
func (sb *StatusBar) SetConnectionState(state ConnectionState) {
	sb.connState = state
}

// View renders the status bar at the given width. dividerCol is the
// column position of the panel divider (0 to skip the junction).
func (sb StatusBar) View(width, dividerCol int) string {
	// Build the right portion: " 10 messages · 3 unread · ● connected ─╯"
	counts := fmt.Sprintf("%d messages", sb.total)
	if sb.unread > 0 {
		counts += fmt.Sprintf(" · %d unread", sb.unread)
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

	// Measure right portion width using plain text (no ANSI).
	rightPlain := " " + counts + " · " + connIcon + " " + connText + " ─╯"
	rightWidth := lipgloss.Width(rightPlain)

	// Build left fill with ┴ at dividerCol.
	fillWidth := maxInt(0, width-rightWidth)
	var buf strings.Builder
	for i := 0; i < fillWidth; i++ {
		if dividerCol > 0 && i == dividerCol {
			buf.WriteRune('┴')
		} else {
			buf.WriteRune('─')
		}
	}

	// Render each segment with styles. The fill uses TopLine style (frame color).
	fillPart := sb.styles.TopLine.Render(buf.String())
	countsPart := sb.styles.StatusBar.Render(" " + counts + " · ")
	connIconPart := connStyle.Render(connIcon)
	connTextPart := sb.styles.StatusBar.Render(" " + connText + " ")
	endPart := sb.styles.TopLine.Render("─╯")

	result := fillPart + countsPart + connIconPart + connTextPart + endPart

	// Clamp to exact width if lipgloss rounding causes drift.
	actual := lipgloss.Width(result)
	if actual < width {
		result += strings.Repeat("─", width-actual)
	} else if actual > width {
		// Trim the fill to compensate.
		trimmed := fillWidth - (actual - width)
		if trimmed < 0 {
			trimmed = 0
		}
		var buf2 strings.Builder
		for i := 0; i < trimmed; i++ {
			if dividerCol > 0 && i == dividerCol {
				buf2.WriteRune('┴')
			} else {
				buf2.WriteRune('─')
			}
		}
		fillPart = sb.styles.TopLine.Render(buf2.String())
		result = fillPart + countsPart + connIconPart + connTextPart + endPart
	}

	return result
}

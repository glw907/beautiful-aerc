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

// SetConnectionState sets the connection state directly.
func (sb *StatusBar) SetConnectionState(state ConnectionState) {
	sb.connState = state
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

	fillWidth := max(0, width-rightWidth)
	fillPart := sb.styles.TopLine.Render(buildFill(fillWidth, dividerCol))
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
		trimmed := max(0, fillWidth-(actual-width))
		fillPart = sb.styles.TopLine.Render(buildFill(trimmed, dividerCol))
		result = fillPart + countsPart + connIconPart + connTextPart + endPart
	}

	return result
}

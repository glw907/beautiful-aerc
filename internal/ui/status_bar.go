package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// ConnectionState represents the mail connection status.
type ConnectionState int

const (
	Connected    ConnectionState = iota
	Offline
	Reconnecting
)

// StatusMode is the left-side content shown in the status bar.
// Account mode shows message + unread counts; Viewer mode shows the
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
}

// NewStatusBar creates a StatusBar with the given styles.
func NewStatusBar(styles Styles) StatusBar {
	return StatusBar{
		styles:    styles,
		connState: Connected,
	}
}

// SetCounts returns a copy of sb with the message and unread counts updated.
func (sb StatusBar) SetCounts(total, unread int) StatusBar {
	sb.total = total
	sb.unread = unread
	return sb
}

// SetConnectionState returns a copy of sb with the connection state set.
func (sb StatusBar) SetConnectionState(state ConnectionState) StatusBar {
	sb.connState = state
	return sb
}

// ConnectionState returns the current connection state. Used in tests.
func (sb StatusBar) ConnectionState() ConnectionState { return sb.connState }

// SetMode returns a copy of sb in the given display mode.
func (sb StatusBar) SetMode(mode StatusMode) StatusBar {
	sb.mode = mode
	return sb
}

// SetScrollPct returns a copy of sb with the viewer scroll percentage
// updated. Only meaningful when mode == StatusViewer.
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

	// Measure right portion width using plain text (no ANSI).
	rightPlain := " " + counts + " · " + connIcon + " " + connText + " ─╯"
	rightWidth := runewidth.StringWidth(rightPlain)

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

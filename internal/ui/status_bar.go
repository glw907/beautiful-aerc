package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type ConnectionState int

const (
	Connected ConnectionState = iota
	Offline
	Reconnecting
)

// StatusMode picks the left-side content. Account shows message + unread
// counts; Viewer shows the scroll percentage of the open body.
type StatusMode int

const (
	StatusAccount StatusMode = iota
	StatusViewer
)

// StatusBar renders the bottom frame edge with the combined status
// indicator.
type StatusBar struct {
	styles    Styles
	total     int
	unread    int
	scrollPct int
	mode      StatusMode
	connState ConnectionState
	// Outbox segment between counts and connection indicator, hidden
	// when both counts are zero.
	outboxInflight int // pending + executing + failed
	outboxConflict int // conflict only
	// Backfill segment between outbox and connection indicator.
	backfillDone   int
	backfillTotal  int
	backfillPaused bool
	backfillWarn   bool
}

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

// SetOutboxDepth updates the outbox segment counts. inflight bundles
// pending+executing+failed; conflict is rendered separately so the
// warning glyph dominates.
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

// SetBackfill updates the backfill progress segment. The segment is
// hidden when total == 0 or done >= total.
func (sb StatusBar) SetBackfill(done, total int, paused, warn bool) StatusBar {
	sb.backfillDone = done
	sb.backfillTotal = total
	sb.backfillPaused = paused
	sb.backfillWarn = warn
	return sb
}

func (sb StatusBar) SetMode(mode StatusMode) StatusBar {
	sb.mode = mode
	return sb
}

// SetScrollPct is only consulted when mode == StatusViewer.
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

// buildFill builds a horizontal rule of width characters with the ┴
// junction placed at dividerCol.
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

// View renders the status bar at width. dividerCol places the ┴
// junction; pass 0 to omit it.
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

	// Build the styled segments first, then measure them with
	// lipgloss.Width. Measuring the plain-text template with a
	// different primitive caused rounding drift on the connection icons
	// and forced a corrective re-render.
	countsPart := sb.styles.StatusBar.Render(" " + counts + " · ")
	outboxPart := sb.renderOutboxSegment()
	backfillPart := sb.renderBackfillSegment(width)
	connIconPart := connStyle.Render(connIcon)
	connTextPart := sb.styles.StatusBar.Render(" " + connText + " ")
	endPart := sb.styles.TopLine.Render("─╯")
	rightWidth := lipgloss.Width(countsPart) + lipgloss.Width(outboxPart) +
		lipgloss.Width(backfillPart) +
		lipgloss.Width(connIconPart) + lipgloss.Width(connTextPart) +
		lipgloss.Width(endPart)

	fillWidth := max(0, width-rightWidth)
	fillPart := sb.styles.TopLine.Render(buildFill(fillWidth, dividerCol))
	return fillPart + countsPart + outboxPart + backfillPart + connIconPart + connTextPart + endPart
}

// renderBackfillSegment formats the ↓ N/M progress segment. width is
// the full status-bar width so the function can collapse to glyph-only
// in the Spartan tier. Returns "" when hidden.
func (sb StatusBar) renderBackfillSegment(width int) string {
	if sb.backfillTotal == 0 || sb.backfillDone >= sb.backfillTotal {
		return ""
	}
	spartan := width < 90
	switch {
	case sb.backfillWarn:
		if spartan {
			return sb.styles.StatusOffline.Render("↓⚠") + sb.styles.StatusBar.Render(" · ")
		}
		return sb.styles.StatusOffline.Render("↓ ⚠") + sb.styles.StatusBar.Render(" · ")
	case sb.backfillPaused:
		if spartan {
			return sb.styles.StatusBar.Render("↓⏸ · ")
		}
		return sb.styles.StatusBar.Render("↓ paused · ")
	default:
		if spartan {
			return sb.styles.StatusBar.Render("↓ · ")
		}
		return sb.styles.StatusBar.Render(fmt.Sprintf("↓ %d/%d · ", sb.backfillDone, sb.backfillTotal))
	}
}

// renderOutboxSegment formats the outbox depth indicator, returning ""
// when both counts are zero so the segment is omitted.
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

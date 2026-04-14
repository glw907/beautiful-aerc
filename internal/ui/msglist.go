package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/mattn/go-runewidth"
)

// Column widths for the message list. Subject takes whatever remains
// after the fixed columns. Flag cell is 1 cell wide because
// lipgloss.Width reports Nerd Font glyphs as 1 cell — the wireframe's
// "2-cell" labels describe visual width, not lipgloss math.
const (
	mlSenderWidth = 22
	mlDateWidth   = 12
	// cursor + flag + sp + sender + sp×2 + subject-pad + sp×2 + date + sp
	mlFixedWidth = 1 + 1 + 1 + mlSenderWidth + 2 + 2 + mlDateWidth + 1
)

// Nerd Font glyphs used in the message list.
const (
	mlCursorGlyph  = "▐"
	mlIconUnread   = "󰇮"
	mlIconAnswered = "󰑚"
	mlIconFlagged  = "󰈻"
)

// SortOrder is the thread-level sort direction. Children inside a
// thread always sort chronologically ascending; SortOrder controls
// only the order of thread roots (and of unthreaded messages, which
// are single-message threads).
type SortOrder int

const (
	SortDateDesc SortOrder = iota // newest activity first (default)
	SortDateAsc                   // oldest activity first
)

// displayRow is one rendered row in the message list. The slice of
// these is computed from the source []MessageInfo by the build
// pipeline (group, sort, flatten). Hidden rows still occupy indices
// in the slice; the renderer skips them and j/k navigation walks
// past them.
type displayRow struct {
	msg          mail.MessageInfo
	prefix       string // "", "├─ ", "└─ ", "│  └─ ", or "[N] " for a folded root
	isThreadRoot bool
	threadSize   int   // set on roots only; 1 for unthreaded
	hidden       bool  // true when collapsed under a folded root
	depth        uint8 // 0 = root; derived during prefix computation
}

// MessageList renders the message list panel: flags, sender, subject,
// and date columns. Hand-rolled (not bubbles/list) to match the
// sidebar pattern and allow the ▐ cursor + selection background.
//
// MessageList owns thread grouping, fold state, and sort direction.
// The source slice is preserved alongside a derived []displayRow so
// fold mutations re-flatten without a backend refetch.
type MessageList struct {
	source   []mail.MessageInfo
	rows     []displayRow
	folded   map[mail.UID]bool
	sort     SortOrder
	selected int
	offset   int
	styles   Styles
	width    int
	height   int
}

// NewMessageList creates a MessageList with the given messages and size.
func NewMessageList(styles Styles, msgs []mail.MessageInfo, width, height int) MessageList {
	m := MessageList{
		styles: styles,
		width:  width,
		height: height,
		folded: map[mail.UID]bool{},
		sort:   SortDateDesc,
	}
	m.SetMessages(msgs)
	return m
}

// SetMessages replaces the source slice and rebuilds the displayRow
// list. Resets fold state, cursor, and viewport.
func (m *MessageList) SetMessages(msgs []mail.MessageInfo) {
	m.source = msgs
	m.folded = map[mail.UID]bool{}
	m.selected = 0
	m.offset = 0
	m.rebuild()
}

// rebuild runs the group → sort → flatten pipeline against m.source
// and applies fold state, producing m.rows. Called from SetMessages
// and from any fold-mutating method. Tasks 5-10 build out the full
// pipeline; for now this is a trivial one-row-per-message pass.
func (m *MessageList) rebuild() {
	rows := make([]displayRow, 0, len(m.source))
	for _, msg := range m.source {
		rows = append(rows, displayRow{
			msg:          msg,
			isThreadRoot: true,
			threadSize:   1,
			depth:        0,
		})
	}
	m.rows = rows
}

// SetSize updates the panel dimensions.
func (m *MessageList) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.clampOffset()
}

// Selected returns the index of the currently selected message.
func (m MessageList) Selected() int { return m.selected }

// SelectedMessage returns the currently selected message. ok is false
// if the list is empty.
func (m MessageList) SelectedMessage() (mail.MessageInfo, bool) {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return mail.MessageInfo{}, false
	}
	return m.rows[m.selected].msg, true
}

// Count returns the number of source messages in the list.
func (m MessageList) Count() int { return len(m.source) }

// moveBy shifts the cursor by delta rows, clamped to the displayRow
// range, and re-clamps the viewport offset. Hidden-row skipping is
// added in Task 12; for now this matches previous behavior because
// the trivial pipeline produces no hidden rows.
func (m *MessageList) moveBy(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.selected = max(0, min(len(m.rows)-1, m.selected+delta))
	m.clampOffset()
}

// MoveDown advances the cursor by one row.
func (m *MessageList) MoveDown() { m.moveBy(1) }

// MoveUp retreats the cursor by one row.
func (m *MessageList) MoveUp() { m.moveBy(-1) }

// MoveToTop jumps the cursor to the first message.
func (m *MessageList) MoveToTop() {
	m.selected = 0
	m.offset = 0
}

// MoveToBottom jumps the cursor to the last message.
func (m *MessageList) MoveToBottom() { m.moveBy(len(m.rows)) }

// HalfPageDown moves the cursor down by half the visible height.
func (m *MessageList) HalfPageDown() { m.moveBy(max(1, m.height/2)) }

// HalfPageUp moves the cursor up by half the visible height.
func (m *MessageList) HalfPageUp() { m.moveBy(-max(1, m.height/2)) }

// PageDown moves the cursor down by one full visible page.
func (m *MessageList) PageDown() { m.moveBy(max(1, m.height)) }

// PageUp moves the cursor up by one full visible page.
func (m *MessageList) PageUp() { m.moveBy(-max(1, m.height)) }

// clampOffset adjusts the viewport so the cursor stays visible.
func (m *MessageList) clampOffset() {
	if m.height <= 0 {
		m.offset = 0
		return
	}
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+m.height {
		m.offset = m.selected - m.height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// View renders the visible window of message rows. Empty state shows
// a centered "No messages" placeholder.
func (m MessageList) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.rows) == 0 {
		return m.renderEmpty()
	}

	plainBg := m.styles.MsgListBg
	selectedBg := m.styles.MsgListSelected

	end := m.offset + m.height
	if end > len(m.rows) {
		end = len(m.rows)
	}

	lines := make([]string, 0, m.height)
	for i := m.offset; i < end; i++ {
		bg := plainBg
		if i == m.selected {
			bg = selectedBg
		}
		lines = append(lines, m.renderRow(i, bg))
	}
	for len(lines) < m.height {
		lines = append(lines, m.renderBlankLine())
	}
	return strings.Join(lines, "\n")
}

// renderRow renders one message row at the configured width.
func (m MessageList) renderRow(idx int, bgStyle lipgloss.Style) string {
	row := m.rows[idx]
	msg := row.msg
	isSelected := idx == m.selected
	isUnread := msg.Flags&mail.FlagSeen == 0

	// Cursor cell (1 col): ▐ when selected, blank otherwise.
	var cursor string
	if isSelected {
		cursor = applyBg(m.styles.MsgListCursor, bgStyle).Render(mlCursorGlyph)
	} else {
		cursor = bgStyle.Render(" ")
	}

	flag := m.renderFlagCell(msg, isUnread, bgStyle)

	// Sender / subject foreground depends on read state.
	senderStyle := m.styles.MsgListReadSender
	subjectStyle := m.styles.MsgListReadSubject
	if isUnread {
		senderStyle = m.styles.MsgListUnreadSender
		subjectStyle = m.styles.MsgListUnreadSubject
	}

	senderText := padRight(truncateCells(msg.From, mlSenderWidth), mlSenderWidth)
	sender := applyBg(senderStyle, bgStyle).Render(senderText)

	dateText := padLeft(truncateCells(msg.Date, mlDateWidth), mlDateWidth)
	date := applyBg(m.styles.MsgListDate, bgStyle).Render(dateText)

	subjectWidth := max(1, m.width-mlFixedWidth)
	subjectText := padRight(truncateCells(msg.Subject, subjectWidth), subjectWidth)
	subject := applyBg(subjectStyle, bgStyle).Render(subjectText)

	line := cursor +
		flag +
		bgStyle.Render(" ") +
		sender +
		bgStyle.Render("  ") +
		subject +
		bgStyle.Render("  ") +
		date +
		bgStyle.Render(" ")

	return fillRowToWidth(line, m.width, bgStyle)
}

// renderFlagCell renders the 1-cell flag column. Priority: flagged >
// answered > unread > none. Read state wins over flag state for color
// — only the unread+flagged case escalates to the warning accent. Read
// rows always use the dim icon style so the glyph dims with the rest
// of the row.
func (m MessageList) renderFlagCell(msg mail.MessageInfo, isUnread bool, bgStyle lipgloss.Style) string {
	iconStyle := m.styles.MsgListIconRead
	if isUnread {
		iconStyle = m.styles.MsgListIconUnread
	}
	var glyph string
	switch {
	case msg.Flags&mail.FlagFlagged != 0:
		glyph = mlIconFlagged
		if isUnread {
			iconStyle = m.styles.MsgListFlagFlagged
		}
	case msg.Flags&mail.FlagAnswered != 0:
		glyph = mlIconAnswered
	case isUnread:
		glyph = mlIconUnread
	default:
		return bgStyle.Render(" ")
	}
	return applyBg(iconStyle, bgStyle).Render(glyph)
}

// renderBlankLine returns a blank line at panel width with the base
// message-list background.
func (m MessageList) renderBlankLine() string {
	return m.styles.MsgListBg.Width(m.width).Render("")
}

// renderEmpty renders the centered "No messages" placeholder.
func (m MessageList) renderEmpty() string {
	label := "No messages"
	labelLine := m.styles.MsgListBg.Width(m.width).
		Foreground(m.styles.Dim.GetForeground()).
		Align(lipgloss.Center).
		Render(label)

	mid := m.height / 2
	lines := make([]string, m.height)
	for i := range lines {
		if i == mid {
			lines[i] = labelLine
		} else {
			lines[i] = m.renderBlankLine()
		}
	}
	return strings.Join(lines, "\n")
}

// truncateCells cuts s to fit width display cells, appending an
// ellipsis when truncated. Inputs are plain mail header text (no ANSI
// escapes), so runewidth handles cell measurement directly without
// the ANSI-stripping pass that lipgloss.Width would do.
func truncateCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= width {
		return s
	}
	return runewidth.Truncate(s, width, "…")
}

// padRight right-pads s with spaces to width display cells. Input is
// plain text (post-truncateCells), so runewidth measures directly.
func padRight(s string, width int) string {
	if w := runewidth.StringWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// padLeft left-pads s with spaces to width display cells. Input is
// plain text (post-truncateCells), so runewidth measures directly.
func padLeft(s string, width int) string {
	if w := runewidth.StringWidth(s); w < width {
		return strings.Repeat(" ", width-w) + s
	}
	return s
}

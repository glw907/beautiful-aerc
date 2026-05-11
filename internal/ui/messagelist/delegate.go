package messagelist

import (
	"fmt"
	"io"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// rowDelegate carries the per-frame render context bubbles/v2/list
// needs to render each displayRow. messagelist.Model holds it as
// *rowDelegate so context refreshes (SetSize, SetLayout,
// SetSearchResults, SetMessages) mutate fields in place without
// rebuilding the list's item slice. The pointer is a deliberate
// escape from the Elm immutable-model contract, analogous to
// ADR-0130's overlay-cache field but without memoization.
type rowDelegate struct {
	styles      Styles
	layout      uicore.LayoutMode
	icons       uicore.IconSet
	measurer    ansix.Measurer
	now         time.Time
	width       int
	resultsMode bool
	originByUID map[mail.UID]string
}

func (d *rowDelegate) Height() int                             { return 1 }
func (d *rowDelegate) Spacing() int                            { return 0 }
func (d *rowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d *rowDelegate) Render(w io.Writer, lm list.Model, idx int, item list.Item) {
	row, ok := item.(displayRow)
	if !ok {
		return
	}
	fmt.Fprint(w, d.renderRow(row, idx == lm.Index()))
}

// renderRow draws one message-list row. Width math, SPUA-A flag-cell
// adjustment, sender column, results-mode [Folder] prefix, thread
// prefix, subject budget, date column, and FillRowToWidth are
// unchanged from Model.renderRow.
func (d *rowDelegate) renderRow(row displayRow, isSelected bool) string {
	msg := row.msg
	isUnread := msg.Flags&mail.FlagSeen == 0

	bgStyle := d.styles.MsgListBg
	if isSelected {
		bgStyle = d.styles.MsgListSelected
	}

	var cursor string
	if isSelected {
		cursor = uicore.ApplyBg(d.styles.MsgListCursor, bgStyle).Render(mlCursorGlyph)
	} else {
		cursor = bgStyle.Render(" ")
	}

	senderStyle := d.styles.MsgListReadSender
	subjectStyle := d.styles.MsgListReadSubject
	if isUnread {
		senderStyle = d.styles.MsgListUnreadSender
		subjectStyle = d.styles.MsgListUnreadSubject
	}

	senderText := padRight(truncateCells(d.senderWithOrigin(msg), d.layout.Sender), d.layout.Sender)
	sender := uicore.ApplyBg(senderStyle, bgStyle).Render(senderText)

	var date string
	if d.layout.Date > 0 {
		dateText := padLeft(truncateCells(row.dateText, d.layout.Date), d.layout.Date)
		date = uicore.ApplyBg(d.styles.MsgListDate, bgStyle).Render(dateText)
	}

	// Fixed overhead: cursor(1) + 3×sp2(separators) + sp(trail) = 8 cells.
	// Flag column adds flag(2) + sp2 = 12 cells. When Date=0 the trailing
	// sp2+date block is omitted. FillRowToWidth absorbs the slack.
	var flag string
	fixed := 8
	if d.layout.FlagColumn {
		flag = d.renderFlagCell(msg, isUnread, bgStyle)
		fixed = 12
	}

	flagAdjust := 0
	if d.measurer.CellWidth() > 1 && d.layout.FlagColumn {
		flagAdjust = ansix.SpuaCount(flag) * (d.measurer.CellWidth() - 1)
	}
	subjectWidth := max(1, d.width-fixed-d.layout.Sender-d.layout.Date-flagAdjust)
	prefixCells := lipgloss.Width(row.prefix)
	subjectCells := max(0, subjectWidth-prefixCells)

	prefixStyled := uicore.ApplyBg(d.styles.MsgListThreadPrefix, bgStyle).Render(row.prefix)
	subjectText := padRight(truncateCells(msg.Subject, subjectCells), subjectCells)
	subjectStyled := uicore.ApplyBg(subjectStyle, bgStyle).Render(subjectText)
	subject := prefixStyled + subjectStyled

	sp2 := bgStyle.Render("  ")
	sp1 := bgStyle.Render(" ")

	var rowStr string
	if d.layout.FlagColumn {
		rowStr = cursor + sp2 + flag + sp2 + sender + sp2 + subject
	} else {
		rowStr = cursor + sp2 + sender + sp2 + subject
	}
	if d.layout.Date > 0 {
		rowStr += sp2 + date
	}
	rowStr += sp1

	return uicore.FillRowToWidth(d.measurer, rowStr, d.width, bgStyle)
}

func (d *rowDelegate) senderWithOrigin(msg mail.MessageInfo) string {
	if !d.resultsMode {
		return msg.From
	}
	folder := d.originByUID[msg.UID]
	if folder == "" {
		return msg.From
	}
	return "[" + folder + "] " + msg.From
}

// renderFlagCell renders the flag column. Glyph priority is flagged,
// answered, unread, none; output is always exactly mlFlagWidth display
// cells regardless of icon mode.
func (d *rowDelegate) renderFlagCell(msg mail.MessageInfo, isUnread bool, bgStyle lipgloss.Style) string {
	iconStyle := d.styles.MsgListIconRead
	if isUnread {
		iconStyle = d.styles.MsgListIconUnread
	}
	var glyph string
	switch {
	case msg.Flags&mail.FlagFlagged != 0:
		glyph = d.icons.FlagFlagged
		if isUnread {
			iconStyle = d.styles.MsgListFlagFlagged
		}
	case msg.Flags&mail.FlagAnswered != 0:
		glyph = d.icons.FlagAnswered
	case isUnread:
		glyph = d.icons.FlagUnread
	default:
		return bgStyle.Render("  ")
	}
	rendered := uicore.ApplyBg(iconStyle, bgStyle).Render(glyph)
	for d.measurer.Width(rendered) < mlFlagWidth {
		rendered += bgStyle.Render(" ")
	}
	return rendered
}

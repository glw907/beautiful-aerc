package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/ui/sidebar"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// renderFrame builds the full-screen layout string. View calls it
// before compositing overlays.
func (m App) renderFrame() string {
	if m.contactsMode {
		return m.renderContactsFrame()
	}
	var rawContent string
	switch {
	case m.outboxView != nil:
		rawContent = m.acct.RenderWithRightPane(m.outboxView.View())
	case m.compose != nil:
		rawContent = m.acct.RenderWithRightPane(m.compose.View())
	default:
		rawContent = m.acct.View()
	}
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	for i := range contentLines {
		contentLines[i] = contentLines[i] + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	dividerCol := uicore.ComputeLayout(m.width).Sidebar
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, dividerCol)
	var foot string
	if m.compose != nil {
		tidyVisible := m.compose.TidyEnabled() && m.compose.IsFocusBody()
		foot = m.footer.ViewGroups(composeFooterGroups(m.compose.HasSignatures(), m.compose.IsFocusFrom(), tidyVisible), m.width)
	} else if m.viewerOpen && m.acct.Viewer().Unsubscribe().Available() {
		foot = m.footer.ViewGroups(viewerFooterGroupsWithUnsub(), m.width)
	} else {
		foot = m.footer.View(m.width)
	}

	parts := []string{topLine, content}
	if bannerRow := m.chromeBannerRow(m.width); bannerRow != "" {
		parts = append(parts, bannerRow)
	}
	parts = append(parts, status, foot)
	// strings.Join over lipgloss.JoinVertical: JoinVertical pads to the
	// widest row using lipgloss.Width, which undercounts SPUA-A glyphs by
	// 1 cell each and would push rows outside the terminal (ADR-0084).
	return strings.Join(parts, "\n")
}

func (m App) windowTitle() string {
	name := m.acct.Cache().Name()
	if name == "" {
		return "poplar"
	}
	return "poplar — " + name
}

func (m App) view(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = m.windowTitle()
	return v
}

func (m App) viewWithCursor(content string, cur *tea.Cursor) tea.View {
	v := m.view(content)
	v.Cursor = cur
	return v
}

func (m App) viewOverlay(box string, x, y int, frame string) tea.View {
	return m.view(uicore.PlaceOverlay(x, y, box, frame))
}

// View composes the full-screen layout. Overlays composite over the
// undimmed account frame via PlaceOverlay. Dim is reserved for unwired
// rows (ADR-0072). The underlay is never dimmed (ADR-0202).
func (m App) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("")
		v.AltScreen = true
		v.WindowTitle = m.windowTitle()
		return v
	}

	frame := m.renderFrame()

	if m.helpOpen {
		box, tooNarrow := m.help.Box(m.width, m.height)
		if tooNarrow != "" {
			x, y := (m.width-lipgloss.Width(tooNarrow))/2, m.height/2
			if x < 0 {
				x = 0
			}
			return m.viewOverlay(tooNarrow, x, y, frame)
		}
		x, y := m.help.Position(box, m.width, m.height)
		return m.viewOverlay(box, x, y, frame)
	}

	if m.confirm.IsOpen() {
		box := m.confirm.Box(m.width, m.height)
		x, y := m.confirm.Position(box, m.width, m.height)
		return m.viewOverlay(box, x, y, frame)
	}

	if m.conflictOpen {
		body := m.conflict.View()
		x, y := uicore.CenterOverlay(body, m.width, m.height)
		return m.viewOverlay(body, x, y, frame)
	}

	if m.outboxOpen {
		body := m.outbox.View()
		x, y := uicore.CenterOverlay(body, m.width, m.height)
		return m.viewOverlay(body, x, y, frame)
	}

	if m.linkPicker.IsOpen() {
		box := m.linkPicker.Box(m.width, m.height)
		x, y := m.linkPicker.Position(box, m.width, m.height)
		return m.viewOverlay(box, x, y, frame)
	}

	if m.attachPicker.IsOpen() {
		box := m.attachPicker.Box(m.width, m.height)
		x, y := m.attachPicker.Position(box, m.width, m.height)
		return m.viewOverlay(box, x, y, frame)
	}

	if m.compose != nil && m.compose.AttachPickerIsOpen() {
		box := m.compose.AttachPickerView()
		x, y := uicore.CenterOverlay(box, m.width, m.height)
		return m.viewOverlay(box, x, y, frame)
	}

	if m.compose != nil && m.compose.SchedulePickerIsOpen() {
		box := m.compose.SchedulePickerView()
		x, y := uicore.CenterOverlay(box, m.width, m.height)
		var cur *tea.Cursor
		if pc := m.compose.SchedulePickerCursor(); pc != nil {
			c := *pc
			c.Position.X += x
			c.Position.Y += y
			cur = &c
		}
		return m.viewWithCursor(uicore.PlaceOverlay(x, y, box, frame), cur)
	}

	if m.reschedule.picker != nil {
		box := m.reschedule.picker.View()
		x, y := uicore.CenterOverlay(box, m.width, m.height)
		var cur *tea.Cursor
		if pc := m.reschedule.picker.Cursor(); pc != nil {
			c := *pc
			c.Position.X += x
			c.Position.Y += y
			cur = &c
		}
		return m.viewWithCursor(uicore.PlaceOverlay(x, y, box, frame), cur)
	}

	if m.movePicker.IsOpen() {
		box := m.movePicker.Box(m.width, m.height)
		x, y := m.movePicker.Position(box, m.width, m.height)
		return m.viewOverlay(box, x, y, frame)
	}

	if m.form != nil && m.form.FromPopover() {
		box := m.form.Box(m.width, m.height)
		x, y := m.form.Position(box, m.width, m.height)
		var cur *tea.Cursor
		if fc := m.form.Cursor(); fc != nil {
			c := *fc
			// x+1 for "│" left border; y+1 for "┌─title─┐" top border row.
			c.Position.X += x + 1
			c.Position.Y += y + 1
			cur = &c
		}
		return m.viewWithCursor(uicore.PlaceOverlay(x, y, box, frame), cur)
	}

	if m.popover != nil {
		box := m.popover.Box(m.width, m.height)
		x, y := m.popover.Position(box, m.width, m.height)
		return m.viewOverlay(box, x, y, frame)
	}

	cur := m.frameCursor()
	return m.viewWithCursor(frame, cur)
}

// frameCursor computes the global cursor position for the non-overlay
// frame.
func (m App) frameCursor() *tea.Cursor {
	sidebarW := uicore.ComputeLayout(m.width).Sidebar
	if m.compose != nil {
		if cc := m.compose.Cursor(); cc != nil {
			c := *cc
			c.Position.X += sidebarW + 1
			c.Position.Y += 1 // topLine occupies row 0
			return &c
		}
	}
	if m.form != nil && !m.form.FromPopover() {
		if fc := m.form.Cursor(); fc != nil {
			c := *fc
			c.Position.X += sidebarW + 1
			c.Position.Y += 1
			return &c
		}
	}
	search := m.acct.SidebarColumnValue().SidebarSearch()
	if sc := search.Cursor(); sc != nil {
		c := *sc
		// Search X is sidebar-column-local; Y is shelf-local (Y=1 prompt).
		// Shelf top in the global frame = 1 (topLine) + contentHeight - ShelfRows.
		c.Position.Y += 1 + m.contentHeight() - sidebar.ShelfRows
		return &c
	}
	return nil
}

// chromeBannerRow renders the chrome row above the status bar, or
// "" when no banner, notice, or toast claims it.
func (m App) chromeBannerRow(width int) string {
	if banner := renderErrorBanner(m.lastErr, width, m.styles); banner != "" {
		return banner
	}
	if m.lastNotice != "" && !m.lastNoticeDeadline.IsZero() && time.Now().Before(m.lastNoticeDeadline) {
		return m.styles.Toast.Render(uicore.TruncateToWidth(m.lastNotice, width))
	}
	if m.toast.op == opSendUndo {
		remaining := time.Until(m.toast.deadline).Round(time.Second)
		if remaining < 0 {
			remaining = 0
		}
		text := fmt.Sprintf("Sending in %ds — u undo", int(remaining.Seconds()))
		return m.styles.Toast.Render(uicore.TruncateToWidth(text, width))
	}
	if !m.toast.IsZero() {
		return renderToast(m.toast, width, m.styles)
	}
	return ""
}

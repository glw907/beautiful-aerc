package ui

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/account"
	"github.com/glw907/poplar/internal/ui/outbox"
	"github.com/glw907/poplar/internal/ui/reader"
)

func (m App) updateSize(msg tea.WindowSizeMsg) (App, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.linkPicker = m.linkPicker.SetSize(m.width, m.height)
	m.linkPicker, cmd = m.linkPicker.Update(msg)
	cmds = append(cmds, cmd)
	m.attachPicker = m.attachPicker.SetSize(m.width, m.height)
	m.attachPicker, cmd = m.attachPicker.Update(msg)
	cmds = append(cmds, cmd)
	m.movePicker = m.movePicker.SetSize(m.width, m.height)
	m.movePicker, cmd = m.movePicker.Update(msg)
	cmds = append(cmds, cmd)
	m.confirm = m.confirm.SetSize(m.width, m.height)
	m.confirm, cmd = m.confirm.Update(msg)
	cmds = append(cmds, cmd)
	m.outbox = m.outbox.SetSize(m.width, m.height)
	m.conflict = m.conflict.SetSize(m.width, m.height)
	m.help = m.help.SetSize(m.width, m.height)
	contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
	m.acct, cmd = m.acct.Update(contentMsg)
	cmds = append(cmds, cmd)
	if m.compose != nil {
		w, h := m.rightPaneSize()
		m.compose.SetSize(w, h)
	}
	if m.outboxView != nil {
		w, h := m.rightPaneSize()
		m.outboxView.SetSize(w, h)
	}
	if m.reschedule.picker != nil {
		m.reschedule.picker.SetSize(m.width, m.height)
	}
	if m.contactsMode {
		m.contactsSidebar, m.contactsList = m.sizedContactsChildren()
	}
	if m.form != nil {
		w, h := m.formSize(m.form.FromPopover())
		next := m.form.SetSize(w, h)
		m.form = &next
	}
	return m, tea.Batch(cmds...)
}

// updateChromeMsg handles banner/toast/notice/error/triage/backend/
// cache-event/folder-loaded msgs. Returns claimed=true on match.
func (m App) updateChromeMsg(msg tea.Msg) (App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case reader.AttachmentSavedMsg:
		deadline := m.now().Add(time.Duration(m.undoSeconds) * time.Second)
		m, cmd := m.armToast(pendingAction{
			op:       opSaveAttachment,
			dest:     msg.Path,
			deadline: deadline,
		})
		return m, cmd, true

	case account.TriageStartedMsg:
		deadline := m.now().Add(time.Duration(m.undoSeconds) * time.Second)
		m, cmd := m.armToast(pendingAction{
			op:       msg.Op,
			n:        msg.N,
			dest:     msg.Dest,
			inverse:  msg.Inverse,
			deadline: deadline,
		})
		return m, cmd, true

	case toastExpireMsg:
		if m.toast.IsZero() || !msg.deadline.Equal(m.toast.deadline) {
			return m, nil, true
		}
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		m, rcmd := m.maybeResizeChild(hadBanner)
		return m, rcmd, true

	case undoCountdownTickMsg:
		if m.toast.op != opSendUndo || m.now().After(m.toast.deadline) {
			return m, nil, true
		}
		return m, undoCountdownTickCmd(), true

	case undoRequestedMsg:
		if m.toast.IsZero() {
			return m, nil, true
		}
		cmd := m.toast.inverse
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		var cmds []tea.Cmd
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...), true

	case UnsubscribeDoneMsg:
		m.lastNotice = "Unsubscribed from " + msg.Host
		m.lastNoticeDeadline = time.Now().Add(5 * time.Second)
		return m, clearNoticeAfter(5 * time.Second), true

	case noticeExpireMsg:
		if !m.lastNoticeDeadline.IsZero() && !time.Now().Before(m.lastNoticeDeadline) {
			m.lastNotice = ""
			m.lastNoticeDeadline = time.Time{}
		}
		return m, nil, true

	case ErrorMsg:
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		m.lastNotice = ""
		m.lastErr = msg
		cmds := make([]tea.Cmd, 0, 2)
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		m = m.deriveChromeFromAcct()
		cmds = append(cmds, fcmd)
		return m, tea.Batch(cmds...), true

	case account.FolderLoadedMsg:
		return m.handleFolderLoaded(msg)

	case backendUpdateMsg:
		return m.handleBackendUpdate(msg)

	case account.CacheEventMsg:
		return m.handleCacheEvent(msg)
	}
	return m, nil, false
}

func (m App) handleFolderLoaded(msg account.FolderLoadedMsg) (App, tea.Cmd, bool) {
	if msg.Name == mail.CanonicalOutbox {
		if m.outboxView == nil {
			m.outboxPrevFolder = m.acct.CurrentFolderName()
			ob := outbox.New(m.theme, m.measurer)
			w, h := m.rightPaneSize()
			ob.SetSize(w, h)
			m.outboxView = &ob
		}
		return m, loadOutboxScheduledCmd(m.acct.Cache()), true
	}
	// A fresh folder load commits any in-flight toast (msglist was
	// reset by selectionChangedCmds).
	if !m.toast.IsZero() && m.acct.MessageListCount() == 0 {
		hadBanner := m.hasBannerRow()
		m.toast = pendingAction{}
		var rcmd tea.Cmd
		m, rcmd = m.maybeResizeChild(hadBanner)
		acct, fcmd := m.acct.Update(msg)
		m.acct = acct
		m = m.deriveChromeFromAcct()
		cmds := []tea.Cmd{fcmd}
		if rcmd != nil {
			cmds = append(cmds, rcmd)
		}
		return m, tea.Batch(cmds...), true
	}
	return m, nil, false
}

func (m App) handleBackendUpdate(msg backendUpdateMsg) (App, tea.Cmd, bool) {
	cmds := []tea.Cmd{pumpUpdatesCmd(m.acct.Backend())}
	if msg.update.Type == mail.UpdateConnState {
		var cs ConnectionState
		switch msg.update.ConnState {
		case mail.ConnConnected:
			cs = Connected
			m.offlineHinted = false
		case mail.ConnReconnecting:
			cs = Reconnecting
		default:
			cs = Offline
			if !m.offlineHinted {
				d := m.lastOutboxDepth
				if d.Pending+d.Executing+d.Failed+d.Conflict > 0 {
					m.lastErr = ErrorMsg{
						Op:  "connection",
						Err: errors.New("offline — queued ops will sync on reconnect"),
					}
					m.offlineHinted = true
				}
			}
		}
		m.statusBar = m.statusBar.SetConnectionState(cs)
		m.acct.NotifyConnState(cs == Connected)
		m = m.refreshBackfillSegment()
	}
	return m, tea.Batch(cmds...), true
}

func (m App) handleCacheEvent(msg account.CacheEventMsg) (App, tea.Cmd, bool) {
	cmds := []tea.Cmd{refreshOutboxDepthCmd(m.acct.Cache())}
	if m.outboxOpen {
		cmds = append(cmds, loadOutboxSummaryCmd(m.acct.Cache()))
	}
	if m.conflictOpen {
		cmds = append(cmds, loadOutboxConflictsCmd(m.acct.Cache()))
	}
	if m.outboxView != nil {
		cmds = append(cmds, loadOutboxScheduledCmd(m.acct.Cache()))
	}
	if msg.Event.Note != "" {
		m.lastErr = ErrorMsg{Op: "draft", Err: errors.New(msg.Event.Note)}
	}
	acct, fcmd := m.acct.Update(msg)
	m.acct = acct
	cmds = append(cmds, fcmd)
	m = m.refreshBackfillSegment()
	return m, tea.Batch(cmds...), true
}

// hasBannerRow reports whether the chrome row above the status bar is
// occupied by an error banner, a success notice, or an active toast.
func (m App) hasBannerRow() bool {
	if m.lastErr.Err != nil || !m.toast.IsZero() {
		return true
	}
	return m.lastNotice != "" && !m.lastNoticeDeadline.IsZero() && time.Now().Before(m.lastNoticeDeadline)
}

// maybeResizeChild re-forwards a WindowSizeMsg to the child when the
// chrome banner row's occupancy has changed since hadBanner was
// captured.
func (m App) maybeResizeChild(hadBanner bool) (App, tea.Cmd) {
	if hadBanner == m.hasBannerRow() || m.width <= 0 || m.height <= 0 {
		return m, nil
	}
	contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
	acct, cmd := m.acct.Update(contentMsg)
	m.acct = acct
	return m, cmd
}

// armToast sets a pending toast, schedules its expiry tick, and
// reflows the account child when the chrome banner row changed
// occupancy.
func (m App) armToast(action pendingAction) (App, tea.Cmd) {
	hadBanner := m.hasBannerRow()
	m.toast = action
	deadline := action.deadline
	cmds := []tea.Cmd{tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
		return toastExpireMsg{deadline: deadline}
	})}
	m, rcmd := m.maybeResizeChild(hadBanner)
	if rcmd != nil {
		cmds = append(cmds, rcmd)
	}
	return m, tea.Batch(cmds...)
}

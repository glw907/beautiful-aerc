package ui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/account"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/outbox"
)

// updateOutboxMsg handles outbox queue, conflict, and overlay msgs.
func (m App) updateOutboxMsg(msg tea.Msg) (App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case outboxScheduledMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox", Err: msg.err}
			return m, nil, true
		}
		if m.outboxView != nil {
			m.outboxView.SetRows(msg.rows)
		}
		return m, nil, true

	case outboxCancelledMsg:
		if msg.err != nil && !errors.Is(msg.err, cache.ErrNotPending) {
			m.lastErr = ErrorMsg{Op: "cancel op", Err: msg.err}
		}
		return m, tea.Batch(loadOutboxScheduledCmd(m.acct.Cache()), refreshOutboxDepthCmd(m.acct.Cache())), true

	case rescheduleOpMsg:
		if msg.err != nil {
			if errors.Is(msg.err, cache.ErrNotPending) {
				m.lastErr = ErrorMsg{Op: "reschedule", Err: errors.New("op already dispatched")}
			} else {
				m.lastErr = ErrorMsg{Op: "reschedule", Err: msg.err}
			}
		}
		return m, tea.Batch(loadOutboxScheduledCmd(m.acct.Cache()), refreshOutboxDepthCmd(m.acct.Cache())), true

	case outboxDepthMsg:
		prev := m.lastOutboxDepth
		m.lastOutboxDepth = msg.depth
		inflight := msg.depth.Pending + msg.depth.Executing + msg.depth.Failed
		m.statusBar = m.statusBar.SetOutboxDepth(inflight, msg.depth.Conflict)
		total := msg.depth.Pending + msg.depth.Executing + msg.depth.Failed + msg.depth.Conflict
		prevTotal := prev.Pending + prev.Executing + prev.Failed + prev.Conflict
		if total != prevTotal {
			m.acct.SetOutboxCount(total)
		}
		return m, nil, true

	case outboxSummaryMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox summary", Err: msg.err}
			return m, nil, true
		}
		if m.outboxOpen {
			m.outbox = m.outbox.SetGroups(msg.groups)
		}
		return m, nil, true

	case outboxConflictsMsg:
		if msg.err != nil {
			m.lastErr = ErrorMsg{Op: "outbox conflicts", Err: msg.err}
			return m, nil, true
		}
		if m.conflictOpen {
			m.conflict = m.conflict.SetRows(msg.rows)
			if len(msg.rows) == 0 {
				m.conflict = m.conflict.Close()
				m.conflictOpen = false
			}
		}
		return m, nil, true

	case RetryConflictMsg:
		return m, retryConflictCmd(m.acct.Cache(), msg.OpID), true

	case DiscardConflictMsg:
		return m, discardConflictCmd(m.acct.Cache(), msg.OpID), true

	case OpenConflictsFromOutboxMsg:
		m.outboxOpen = false
		m.conflictOpen = true
		m.conflict = m.conflict.Open(nil)
		return m, loadOutboxConflictsCmd(m.acct.Cache()), true

	case conflictResolvedMsg:
		if msg.err != nil && !errors.Is(msg.err, cache.ErrNotConflict) {
			m.lastErr = ErrorMsg{Op: "resolve conflict", Err: msg.err}
		}
		return m, tea.Batch(loadOutboxConflictsCmd(m.acct.Cache()), refreshOutboxDepthCmd(m.acct.Cache())), true

	case outbox.CloseMsg:
		m.outboxView = nil
		// Restore previous folder; fall back to Inbox.
		prev := m.outboxPrevFolder
		if prev == "" {
			prev = mail.CanonicalInbox
		}
		m.outboxPrevFolder = ""
		acct, cmd := m.acct.Update(account.JumpFolderMsg{Canonical: prev})
		m.acct = acct
		m = m.deriveChromeFromAcct()
		return m, cmd, true

	case outbox.CancelMsg:
		return m, cancelOutboxOpCmd(m.acct.Cache(), msg.OpID), true

	case outbox.RescheduleMsg:
		p := uicompose.NewSchedulePicker(m.theme, m.now(), msg.Initial)
		p.SetSize(m.width, m.height)
		m.reschedule = pendingReschedule{picker: &p, opID: msg.OpID}
		return m, nil, true

	case outbox.EditAsDraftMsg:
		if msg.Draft == nil {
			return m, nil, true
		}
		return m, editAsDraftCmd(m.acct.Cache(), msg.OpID, msg.Draft), true
	}
	return m, nil, false
}

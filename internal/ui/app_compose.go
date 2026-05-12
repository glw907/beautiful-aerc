package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailcompose"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
)

// updateComposeMsg handles all uicompose.* msgs and draft-lifecycle
// msgs. ScheduleAcceptedMsg/ScheduleCancelledMsg fall to compose only
// when the outbox-side reschedule picker is not active.
func (m App) updateComposeMsg(msg tea.Msg) (App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case uicompose.ScheduleAcceptedMsg:
		if m.reschedule.picker != nil {
			opID := m.reschedule.opID
			m.reschedule = pendingReschedule{}
			return m, rescheduleOpCmd(m.acct.Cache(), opID, msg.When), true
		}
		if m.compose != nil {
			var cmd tea.Cmd
			m.compose, cmd = m.compose.Update(msg)
			return m, cmd, true
		}
		return m, nil, true

	case uicompose.ScheduleCancelledMsg:
		if m.reschedule.picker != nil {
			m.reschedule = pendingReschedule{}
			return m, nil, true
		}
		if m.compose != nil {
			var cmd tea.Cmd
			m.compose, cmd = m.compose.Update(msg)
			return m, cmd, true
		}
		return m, nil, true

	case uicompose.SendMsg:
		sent := resolveSentFolder(m.acct.Cache())
		if sent == "" {
			if m.compose != nil {
				m.compose.SetErr("no Sent folder configured")
			}
			return m, nil, true
		}
		d := msg.Draft
		acct := m.acct.Cache()
		cmds := []tea.Cmd{composeSendCmd(acct, sent, d, m.identities, m.undoSendWindow, msg.ScheduledFor)}
		if m.compose != nil && m.compose.DraftID() != "" {
			draftID := m.compose.DraftID()
			prevUID := mail.UID(m.compose.PrevServerUID())
			draftsFolder := resolveDraftsFolder(acct)
			cmds = append(cmds, discardDraftCmd(acct, draftID, draftsFolder, prevUID))
		}
		m.compose = nil
		return m, tea.Batch(cmds...), true

	case uicompose.SentMsg:
		if msg.ScheduledFor.IsZero() || !msg.ScheduledFor.After(m.now()) {
			return m, nil, true
		}
		m, cmd := m.armToast(pendingAction{
			op:        opSendUndo,
			deadline:  msg.ScheduledFor,
			sendOpIDs: msg.OpIDs,
			sendDraft: msg.Draft,
		})
		return m, tea.Batch(cmd, undoCountdownTickCmd()), true

	case uicompose.SeededMsg:
		m, cmd := m.openNewCompose(msg.Draft)
		return m, cmd, true

	case RestoreFromDraftMsg:
		m, cmd := m.openNewCompose(msg.Draft)
		return m, cmd, true

	case openDraftMsg:
		w, h := m.rightPaneSize()
		row := msg.row
		c := uicompose.Open(m.theme, uicompose.NewStyles(m.theme), m.acct.AccountEmail(), row.DraftID, msg.draft, m.suggestAddresses, m.measurer)
		c.SetSize(w, h)
		c.SetIdentities(m.identities)
		c.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
		c.SetCache(m.acct.Cache())
		c.SetDraftTarget(row.ServerFolder, string(row.ServerUID))
		m.compose = c
		return m, m.compose.Init(), true

	case uicompose.EnqueuePushDraftMsg:
		return m, enqueuePushDraftCmd(m.acct.Cache(), msg.DraftID, msg.Folder, msg.MIME, mail.UID(msg.PrevServerUID)), true

	case uicompose.CancelMsg:
		if m.compose == nil {
			return m, nil, true
		}
		if !msg.Dirty {
			m.compose = nil
			return m, nil, true
		}
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Save draft?",
			Body:  "[y] Save and close   [n] Discard   [Esc] Keep editing",
		})
		m.pendingComposeSave = true
		return m, nil, true

	case uicompose.AttachAcceptedMsg, uicompose.AttachCancelledMsg:
		if m.compose != nil {
			var cmd tea.Cmd
			m.compose, cmd = m.compose.Update(msg)
			return m, cmd, true
		}
		return m, nil, true
	}
	return m, nil, false
}

// openNewCompose mounts a fresh compose with the given seed draft.
// Used for SeededMsg and RestoreFromDraftMsg, where compose opens
// without a cache-side draft target. Closes any open viewer first
// so updateMouse's right-pane routing matches the rendered surface.
func (m App) openNewCompose(draft mailcompose.Draft) (App, tea.Cmd) {
	if m.viewerOpen {
		m.acct = m.acct.CloseViewer()
		m = m.deriveChromeFromAcct()
	}
	w, h := m.rightPaneSize()
	c := uicompose.New(m.theme, uicompose.NewStyles(m.theme), m.acct.AccountEmail(), m.suggestAddresses, m.measurer)
	c.SetSize(w, h)
	c.SetIdentities(m.identities)
	c.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
	c.Seed(draft)
	m.compose = c
	return m, c.Init()
}

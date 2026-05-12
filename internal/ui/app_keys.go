package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/content"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/contacts"
	"github.com/glw907/poplar/internal/ui/helppopover"
	"github.com/glw907/poplar/internal/ui/sidebar"
)

func (m App) updateKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	m.acct.NotifyActivity()
	if m2, cmd, claimed := m.routeOverlayKey(msg); claimed {
		return m2, cmd
	}
	if m.contactsMode {
		return m.updateContactsKey(msg)
	}
	return m.updateGlobalKey(msg)
}

// The overlay cascade runs help > confirm > conflict > outbox >
// reschedule > outboxView > linkPicker > attachPicker > movePicker >
// form > popover > compose. Help, while open, swallows every key but
// its own close binding.
func (m App) routeOverlayKey(msg tea.KeyPressMsg) (App, tea.Cmd, bool) {
	if m.helpOpen {
		if key.Matches(msg, m.keys.CloseHelp) {
			m.helpOpen = false
		}
		return m, nil, true
	}
	if m.confirm.IsOpen() {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd, true
	}
	if m.conflictOpen {
		var cmd tea.Cmd
		m.conflict, cmd = m.conflict.Update(msg)
		if !m.conflict.IsOpen() {
			m.conflictOpen = false
		}
		return m, cmd, true
	}
	if m.outboxOpen {
		var cmd tea.Cmd
		m.outbox, cmd = m.outbox.Update(msg)
		if !m.outbox.IsOpen() {
			m.outboxOpen = false
		}
		return m, cmd, true
	}
	if m.reschedule.picker != nil {
		p, cmd := m.reschedule.picker.Update(msg)
		m.reschedule.picker = &p
		return m, cmd, true
	}
	if m.outboxView != nil {
		next, cmd := m.outboxView.Update(msg)
		m.outboxView = &next
		return m, cmd, true
	}
	if m.linkPicker.IsOpen() {
		var cmd tea.Cmd
		m.linkPicker, cmd = m.linkPicker.Update(msg)
		return m, cmd, true
	}
	if m.attachPicker.IsOpen() {
		var cmd tea.Cmd
		m.attachPicker, cmd = m.attachPicker.Update(msg)
		return m, cmd, true
	}
	if m.movePicker.IsOpen() {
		var cmd tea.Cmd
		m.movePicker, cmd = m.movePicker.Update(msg)
		return m, cmd, true
	}
	if m.form != nil {
		next, cmd := m.form.Update(msg)
		m.form = &next
		return m, cmd, true
	}
	if m.popover != nil {
		next, cmd := m.popover.Update(msg)
		m.popover = &next
		return m, cmd, true
	}
	if m.compose != nil {
		next, cmd := m.compose.Update(msg)
		m.compose = next
		return m, cmd, true
	}
	return m, nil, false
}

// Unmatched keys fall through to acct.Update.
func (m App) updateGlobalKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	// In the Drafts folder, Enter opens compose instead of the viewer.
	if msg.Code == tea.KeyEnter && !m.viewerOpen {
		if info, ok := m.acct.SelectedMessage(); ok {
			draftsFolder := resolveDraftsFolder(m.acct.Cache())
			if draftsFolder != "" && m.acct.CurrentFolderName() == draftsFolder {
				if id, ok := draftLocalID(info.UID); ok {
					return m, openLocalDraftCmd(m.acct.Cache(), id)
				}
				return m, openDraftFromServerUIDCmd(m.acct.Cache(), info.UID, draftsFolder)
			}
		}
	}
	switch {
	case key.Matches(msg, m.keys.Compose):
		if m.viewerOpen {
			m.acct = m.acct.CloseViewer()
			m = m.deriveChromeFromAcct()
		}
		w, h := m.rightPaneSize()
		m.compose = uicompose.New(m.theme, uicompose.NewStyles(m.theme), m.acct.AccountEmail(), m.suggestAddresses, m.measurer)
		m.compose.SetSize(w, h)
		m.compose.SetIdentities(m.identities)
		m.compose.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
		m.compose.SetCache(m.acct.Cache())
		draftsFolder := resolveDraftsFolder(m.acct.Cache())
		m.compose.SetDraftTarget(draftsFolder, "")
		return m, m.compose.Init()
	case key.Matches(msg, m.keys.Reply), key.Matches(msg, m.keys.ReplyAll), key.Matches(msg, m.keys.Forward):
		parent, ok := m.selectedMessage()
		if !ok {
			break
		}
		kind := uicompose.SeedReply
		if key.Matches(msg, m.keys.ReplyAll) {
			kind = uicompose.SeedReplyAll
		} else if key.Matches(msg, m.keys.Forward) {
			kind = uicompose.SeedForward
		}
		return m, composeSeedCmd(m.acct.Cache(), parent, m.acct.AccountEmail(), kind)
	case key.Matches(msg, m.keys.Undo):
		// Undo is only live while a toast is active. Otherwise 'u' falls
		// through to AccountTab so other consumers can claim it.
		switch m.toast.op {
		case opNone:
			// no-op, fall through
		case opSendUndo:
			opIDs := m.toast.sendOpIDs
			draft := m.toast.sendDraft
			hadBanner := m.hasBannerRow()
			m.toast = pendingAction{}
			var rcmd tea.Cmd
			m, rcmd = m.maybeResizeChild(hadBanner)
			return m, tea.Batch(rcmd, undoSendCmd(m.acct.Cache(), opIDs, draft))
		default:
			return m, func() tea.Msg { return undoRequestedMsg{} }
		}
	case key.Matches(msg, m.keys.Quit):
		if m.viewerOpen {
			var cmd tea.Cmd
			m.acct, cmd = m.acct.Update(msg)
			m = m.deriveChromeFromAcct()
			return m, cmd
		}
		if m.acct.SearchState() != sidebar.SearchIdle {
			var cmd tea.Cmd
			m.acct, cmd = m.acct.Update(sidebar.ClearSearchMsg{})
			m = m.deriveChromeFromAcct()
			return m, cmd
		}
		return m, tea.Quit
	case key.Matches(msg, m.keys.ForceQuit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.OutboxOverlay):
		m.outboxOpen = true
		m.outbox = m.outbox.Open(nil)
		return m, loadOutboxSummaryCmd(m.acct.Cache())
	case key.Matches(msg, m.keys.ConflictOverlay):
		m.conflictOpen = true
		m.conflict = m.conflict.Open(nil)
		return m, loadOutboxConflictsCmd(m.acct.Cache())
	case key.Matches(msg, m.keys.Help):
		m.helpOpen = true
		ctx := helppopover.Account
		switch {
		case m.viewerOpen:
			ctx = helppopover.Viewer
		case m.compose != nil:
			ctx = helppopover.Compose
		}
		m.help = helppopover.New(helppopover.NewStyles(m.theme), ctx).WithKbdCaps(m.kbdCaps).SetSize(m.width, m.height)
		return m, nil
	case key.Matches(msg, m.keys.SenderPopover):
		if !m.viewerOpen {
			info, ok := m.acct.SelectedMessage()
			if ok {
				displayName, email := parseSender(info.From)
				return m, func() tea.Msg {
					return contacts.OpenPopoverMsg{DisplayName: displayName, Email: email}
				}
			}
		}
	case key.Matches(msg, m.keys.ContactsMode):
		return m, func() tea.Msg { return contacts.EnterContactsModeMsg{} }
	}

	var cmd tea.Cmd
	m.acct, cmd = m.acct.Update(msg)
	m = m.deriveChromeFromAcct()
	return m, cmd
}

// updateContactsKey handles a key press in contacts mode.
func (m App) updateContactsKey(msg tea.KeyPressMsg) (App, tea.Cmd) {
	if key.Matches(msg, m.keys.MailMode) {
		return m, func() tea.Msg { return contacts.ExitContactsModeMsg{} }
	}
	if key.Matches(msg, m.keys.Quit) || key.Matches(msg, m.keys.ForceQuit) {
		return m, tea.Quit
	}

	prevLetter := m.contactsSidebar.SelectionLetter()
	var sbCmd, listCmd tea.Cmd
	m.contactsSidebar, sbCmd = m.contactsSidebar.Update(msg)
	m.contactsList, listCmd = m.contactsList.Update(msg)

	newLetter := m.contactsSidebar.SelectionLetter()
	if newLetter != prevLetter && newLetter != 0 {
		m.contactsList = m.contactsList.SetSelectionLetter(newLetter)
	}

	return m, tea.Batch(sbCmd, listCmd)
}

// parseSender splits the From display string into (displayName, email).
// Falls back to (from, from) when parsing fails.
func parseSender(from string) (displayName, email string) {
	addrs := content.ParseAddressList(from)
	if len(addrs) == 0 {
		return from, from
	}
	a := addrs[0]
	if a.Name != "" {
		return a.Name, a.Email
	}
	return a.Email, a.Email
}

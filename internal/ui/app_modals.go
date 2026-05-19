package ui

import (
	"net/url"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailcompose"
	"github.com/glw907/poplar/internal/ui/account"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/movepicker"
	"github.com/glw907/poplar/internal/ui/reader"
)

// updateModalsMsg handles overlay open/close msgs and confirm-modal
// resolution. Returns claimed=true when a case fires.
func (m App) updateModalsMsg(msg tea.Msg) (App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case reader.OpenLinkPickerMsg:
		m.linkPicker = m.linkPicker.Open(msg.Links)
		return m, nil, true

	case reader.LinkPickerClosedMsg:
		m.linkPicker = m.linkPicker.Close()
		return m, nil, true

	case movepicker.OpenMsg:
		m.movePicker = m.movePicker.Open(msg.UIDs, msg.Src, msg.Folders)
		return m, nil, true

	case movepicker.ClosedMsg:
		m.movePicker = m.movePicker.Close()
		return m, nil, true

	case reader.OpenAttachPickerMsg:
		m.attachPicker = m.attachPicker.Open(msg.UID, msg.Items)
		return m, nil, true

	case reader.AttachPickerClosedMsg:
		m.attachPicker = m.attachPicker.Close()
		return m, nil, true

	case reader.OpenAttachmentMsg:
		return m, openAttachmentCmd(m.acct.Cache(), m.opener, msg.UID, msg.Att), true

	case reader.SaveAttachmentMsg:
		return m, saveAttachmentCmd(m.acct.Cache(), m.downloadDir, msg.UID, msg.Att), true

	case reader.LaunchURLMsg:
		return m, launchURLCmd(m.opener, msg.URL), true

	case reader.OpenUnsubscribeConfirmMsg:
		host := unsubscribeHost(msg.Unsub)
		stash := msg
		m.pendingUnsub = &stash
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Unsubscribe",
			Body:  "Send unsubscribe request to " + host + "?",
		})
		return m, nil, true

	case account.OpenConfirmEmptyMsg:
		body := strconv.Itoa(msg.Total) + " messages will be permanently deleted."
		m.pendingEmpty = pendingEmptyConfirm{folder: msg.Folder, source: msg.Source}
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Empty " + msg.Folder,
			Body:  body,
		})
		return m, nil, true

	case ConfirmModalYesMsg:
		return m.confirmYes()

	case ConfirmModalNoMsg:
		return m.confirmNo()

	case ConfirmModalClosedMsg:
		return m.confirmClosed()
	}
	return m, nil, false
}

func (m App) confirmYes() (App, tea.Cmd, bool) {
	m.confirm = m.confirm.Close()
	switch {
	case m.pendingFormDiscard:
		m.pendingFormDiscard = false
		m.form = nil
		return m, nil, true
	case m.pendingContactDelete != "":
		uid := m.pendingContactDelete
		m.pendingContactDelete = ""
		m.form = nil
		return m, queueContactDeleteCmd(m.acct.Cache(), uid), true
	case m.pendingComposeSave:
		m.pendingComposeSave = false
		if m.compose == nil {
			return m, nil, true
		}
		draftsFolder := resolveDraftsFolder(m.acct.Cache())
		d := m.compose.CurrentDraft()
		draftID := m.compose.DraftID()
		prevUID := mail.UID(m.compose.PrevServerUID())
		m.compose = nil
		if draftsFolder != "" {
			return m, upsertAndPushDraftCmd(m.acct.Cache(), draftID, draftsFolder, d, prevUID, m.identities), true
		}
		return m, nil, true
	case m.pendingUnsub != nil:
		pu := *m.pendingUnsub
		m.pendingUnsub = nil
		return m, m.dispatchUnsubscribe(pu.Unsub), true
	case m.pendingEmpty.folder != "":
		folder, source := m.pendingEmpty.folder, m.pendingEmpty.source
		m.pendingEmpty = pendingEmptyConfirm{}
		return m, func() tea.Msg {
			return account.EmptyFolderConfirmedMsg{Folder: folder, Source: source}
		}, true
	}
	return m, nil, true
}

func (m App) confirmNo() (App, tea.Cmd, bool) {
	m.confirm = m.confirm.Close()
	m.pendingUnsub = nil
	if m.pendingFormDiscard {
		m.pendingFormDiscard = false
		return m, nil, true
	}
	if m.pendingContactDelete != "" {
		m.pendingContactDelete = ""
		return m, nil, true
	}
	if m.pendingComposeSave {
		m.pendingComposeSave = false
		if m.compose != nil {
			draftID := m.compose.DraftID()
			prevUID := mail.UID(m.compose.PrevServerUID())
			draftsFolder := resolveDraftsFolder(m.acct.Cache())
			m.compose = nil
			return m, discardDraftCmd(m.acct.Cache(), draftID, draftsFolder, prevUID), true
		}
	}
	return m, nil, true
}

func (m App) confirmClosed() (App, tea.Cmd, bool) {
	// Esc on discard-changes keeps the form mounted.
	if m.pendingFormDiscard {
		m.pendingFormDiscard = false
		m.confirm = m.confirm.Close()
		return m, nil, true
	}
	if m.pendingContactDelete != "" {
		m.pendingContactDelete = ""
		m.confirm = m.confirm.Close()
		return m, nil, true
	}
	// Esc on save-draft keeps compose mounted.
	if m.pendingComposeSave {
		m.pendingComposeSave = false
		m.confirm = m.confirm.Close()
		return m, nil, true
	}
	if m.pendingUnsub != nil {
		m.pendingUnsub = nil
		m.confirm = m.confirm.Close()
		return m, nil, true
	}
	m.pendingEmpty = pendingEmptyConfirm{}
	m.confirm = m.confirm.Close()
	return m, nil, true
}

// unsubscribeHost returns the user-visible host string for the confirm
// prompt. Picks from the action that will fire (one-click → mailto →
// http) and falls back to a fixed label when every URL is malformed.
func unsubscribeHost(u content.Unsubscribe) string {
	switch {
	case u.OneClick != "":
		if p, err := url.Parse(u.OneClick); err == nil && p.Host != "" {
			return p.Host
		}
	case u.Mailto != "":
		if p, err := url.Parse(u.Mailto); err == nil && p.Opaque != "" {
			at := strings.IndexByte(p.Opaque, '?')
			if at < 0 {
				return p.Opaque
			}
			return p.Opaque[:at]
		}
	case u.HTTP != "":
		if p, err := url.Parse(u.HTTP); err == nil && p.Host != "" {
			return p.Host
		}
	}
	return "this list"
}

// dispatchUnsubscribe routes a confirmed unsubscribe by RFC 8058
// precedence: one-click POST > mailto compose seed > plain http.
func (m App) dispatchUnsubscribe(u content.Unsubscribe) tea.Cmd {
	switch {
	case u.OneClick != "":
		return unsubscribePostCmd(u.OneClick)
	case u.Mailto != "":
		d, err := mailcompose.SeedFromMailto(u.Mailto, m.acct.AccountEmail())
		if err != nil {
			return func() tea.Msg {
				return ErrorMsg{Op: "unsubscribe (mailto)", Err: err}
			}
		}
		return func() tea.Msg { return uicompose.SeededMsg{Draft: d} }
	case u.HTTP != "":
		return launchURLCmd(m.opener, u.HTTP)
	}
	return nil
}

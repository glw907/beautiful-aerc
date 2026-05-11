package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ui/contacts"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// updateContactsMsg handles contacts.* msgs and the contacts-sync
// ticker.
func (m App) updateContactsMsg(msg tea.Msg) (App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case contacts.OpenPopoverMsg:
		p := contacts.NewPopover(m.contactsStyles)
		p.SetSize(m.width, m.height)
		match, _, found := m.acct.Cache().LookupContact(context.Background(), msg.Email)
		p.SetMatch(msg.DisplayName, msg.Email, match, found)
		m.popover = &p
		return m, nil, true

	case contacts.ClosePopoverMsg:
		m.popover = nil
		return m, nil, true

	case contacts.OpenFormMsg:
		m.popover = nil
		saveTo := []string{"Local file"}
		if email := m.acct.AccountEmail(); email != "" {
			saveTo = append(saveTo, email)
		}
		f := contacts.NewForm(m.contactsStyles, msg.Initial, msg.FromPopover, saveTo).
			WithExistingUID(msg.UID)
		w, h := m.formSize(msg.FromPopover)
		f = f.SetSize(w, h)
		m.form = &f
		return m, nil, true

	case contacts.OpenContactDeleteConfirmMsg:
		m.pendingContactDelete = msg.UID
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Delete contact?",
			Body:  msg.DisplayName + " will be removed from this address book.",
		})
		return m, nil, true

	case contacts.ContactSaveMsg:
		uid := ""
		if m.form != nil {
			uid = m.form.ExistingUID()
		}
		m.form = nil
		return m, queueContactPutCmd(m.acct.Cache(), uid, msg.Contact), true

	case contacts.ContactCancelMsg:
		if m.form == nil {
			return m, nil, true
		}
		if !msg.Dirty {
			m.form = nil
			return m, nil, true
		}
		m.pendingFormDiscard = true
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Discard changes?",
			Body:  "Unsaved edits to this contact will be lost.",
		})
		return m, nil, true

	case contacts.EnterContactsModeMsg:
		m.contactsMode = true
		m.contactsSidebar, m.contactsList = m.sizedContactsChildren()
		return m, nil, true

	case contacts.ExitContactsModeMsg:
		m.contactsMode = false
		return m, nil, true

	case contactsTickMsg:
		if m.contactsCfg == nil {
			return m, nil, true
		}
		return m, tea.Batch(
			syncContactsCmd(m.acct.Cache(), m.contactsCfg),
			scheduleSyncCmd(m.contactsRefresh),
		), true

	case contactsSyncedMsg:
		return m, nil, true
	}
	return m, nil, false
}

// contactsColumnWidths returns (sidebarW, listW, detailW). Below 60
// cells the layout collapses to list-only with sidebarW=detailW=0.
func (m App) contactsColumnWidths() (sidebarW, listW, detailW int) {
	contentW := m.width - 1
	if contentW < 60 {
		return 0, contentW, 0
	}
	const sidebarFloor = 14
	const dividers = 2
	listMin := 30
	detail := contentW - sidebarFloor - dividers - listMin
	if detail < 0 {
		detail = 0
		listMin = contentW - sidebarFloor - dividers
		if listMin < 1 {
			listMin = 1
		}
	}
	return sidebarFloor, listMin, detail
}

func (m App) sizedContactsChildren() (contacts.Sidebar, contacts.List) {
	h := m.contactsBodyHeight()
	sbW, listW, _ := m.contactsColumnWidths()
	sb := m.contactsSidebar.SetSize(sbW, h)
	ls := m.contactsList.SetSize(listW, h)
	return sb, ls
}

func (m App) contactsBodyHeight() int {
	h := m.contentHeight() - 1
	if h < 1 {
		return 1
	}
	return h
}

func (m App) renderContactsFrame() string {
	sbW, listW, detailW := m.contactsColumnWidths()
	contentH := m.contactsBodyHeight()

	var content string
	if sbW == 0 {
		content = m.contactsList.View()
	} else {
		sbLines := strings.Split(m.contactsSidebar.View(), "\n")
		listLines := strings.Split(m.contactsList.View(), "\n")
		divLine := m.styles.FrameBorder.Render("│")

		var detailLines []string
		if m.form != nil && !m.form.FromPopover() {
			detailLines = strings.Split(m.form.View(), "\n")
		} else {
			cursor := m.contactsList.Cursor()
			detailLines = strings.Split(contacts.RenderDetailCard(cursor, detailW, m.contactsStyles), "\n")
		}

		assembled := make([]string, contentH)
		for i := range contentH {
			sb := ""
			if i < len(sbLines) {
				sb = sbLines[i]
			}
			sb = uicore.PadOrTruncate(sb, sbW)
			ls := ""
			if i < len(listLines) {
				ls = listLines[i]
			}
			ls = uicore.PadOrTruncate(ls, listW)
			dl := ""
			if detailW > 0 {
				if i < len(detailLines) {
					dl = detailLines[i]
				}
				dl = uicore.PadOrTruncate(dl, detailW)
			}
			assembled[i] = sb + divLine + ls + divLine + dl
		}
		content = strings.Join(assembled, "\n")
	}

	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(content, "\n")
	for i := range contentLines {
		contentLines[i] = contentLines[i] + rightBorder
	}
	body := strings.Join(contentLines, "\n")

	header := uicore.PadOrTruncate("CONTACTS · All sources", m.width-2) + rightBorder
	footerLine := m.footer.SetContext(ContactsContext).View(m.width)

	parts := []string{m.topLine.View(m.width, sbW+1), header, body}
	if bannerRow := m.chromeBannerRow(m.width); bannerRow != "" {
		parts = append(parts, bannerRow)
	}
	parts = append(parts, m.statusBar.View(m.width, sbW+1), footerLine)
	return strings.Join(parts, "\n")
}

// formSize returns (width, height) the contact form should occupy.
// Modal mode (fromPopover) uses the full terminal; right-pane mode
// mirrors the detail column.
func (m App) formSize(fromPopover bool) (int, int) {
	if fromPopover {
		return m.width, m.height
	}
	_, _, detailW := m.contactsColumnWidths()
	return detailW, m.contactsBodyHeight()
}

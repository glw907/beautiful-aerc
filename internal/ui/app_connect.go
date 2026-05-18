package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func (m App) updateConnectMsg(msg tea.Msg) (App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case BackendReadyMsg:
		m.backendState = uicore.BackendConnected
		m.backendErr = nil
		cmds := []tea.Cmd{pumpUpdatesCmd(m.backend)}
		if m.contactsCfg != nil {
			cmds = append(cmds,
				syncContactsCmd(m.acct.Cache(), m.contactsCfg),
				scheduleSyncCmd(m.contactsRefresh),
			)
		}
		return m, tea.Batch(cmds...), true
	case BackendErrMsg:
		m.backendState = uicore.BackendFailed
		m.backendErr = msg.Err
		return m, nil, true
	}
	return m, nil, false
}

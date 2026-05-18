package ui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ui/account"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func (m App) updateConnectMsg(msg tea.Msg) (App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case BackendReadyMsg:
		m.backendState = uicore.BackendConnected
		m.backendErr = nil
		cmds := []tea.Cmd{
			pumpUpdatesCmd(m.backend),
			account.LoadFoldersCmd(m.acct.Cache()),
		}
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

// updateConnectKey claims r only when backendState == BackendFailed; unclaimed
// otherwise so r reaches the reply binding.
func (m App) updateConnectKey(msg tea.KeyPressMsg) (App, tea.Cmd, bool) {
	if m.backendState != uicore.BackendFailed {
		return m, nil, false
	}
	if !key.Matches(msg, m.keys.RetryConnect) {
		return m, nil, false
	}
	m.backendState = uicore.BackendConnecting
	m.backendErr = nil
	m = m.deriveChromeFromAcct()
	return m, connectBackendCmd(context.Background(), m.backend, m.acct.Cache()), true
}

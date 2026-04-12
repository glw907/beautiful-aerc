package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/mail"
)

// foldersLoadedMsg carries the result of an initial ListFolders call.
type foldersLoadedMsg struct {
	folders []mail.Folder
}

// folderLoadedMsg carries the result of opening a folder and fetching
// its header list.
type folderLoadedMsg struct {
	name string
	msgs []mail.MessageInfo
}

// backendErrMsg wraps a backend error. Pass 2.5b-6 (status/toast) will
// surface this to the user; for now Update logs and moves on.
type backendErrMsg struct {
	err error
}

// FolderChangedMsg is emitted by AccountTab whenever the selected folder
// changes. App consumes it to update the status bar without reaching
// into child state.
type FolderChangedMsg struct {
	Name   string
	Exists int
	Unseen int
}

// loadFoldersCmd returns a Cmd that fetches the folder list from the
// backend. The result is delivered as a foldersLoadedMsg, or a
// backendErrMsg on failure.
func loadFoldersCmd(b mail.Backend) tea.Cmd {
	return func() tea.Msg {
		folders, err := b.ListFolders()
		if err != nil {
			return backendErrMsg{err: err}
		}
		return foldersLoadedMsg{folders: folders}
	}
}

// loadFolderCmd returns a Cmd that opens a folder and fetches its
// header list. The result is a folderLoadedMsg, or a backendErrMsg.
// Returns nil when name is empty — bubbletea treats a nil Cmd as "no
// work," so the Update loop skips an otherwise-wasted dispatch.
func loadFolderCmd(b mail.Backend, name string) tea.Cmd {
	if name == "" {
		return nil
	}
	return func() tea.Msg {
		if err := b.OpenFolder(name); err != nil {
			return backendErrMsg{err: err}
		}
		msgs, err := b.FetchHeaders(nil)
		if err != nil {
			return backendErrMsg{err: err}
		}
		return folderLoadedMsg{name: name, msgs: msgs}
	}
}

// folderChangedCmd returns a zero-latency Cmd that emits a
// FolderChangedMsg. Using a Cmd (rather than a direct mutation) keeps
// message flow inside bubbletea's Update loop.
func folderChangedCmd(f mail.Folder) tea.Cmd {
	return func() tea.Msg {
		return FolderChangedMsg{
			Name:   f.Name,
			Exists: f.Exists,
			Unseen: f.Unseen,
		}
	}
}

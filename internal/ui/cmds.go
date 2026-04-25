package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/content"
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

// SearchMode selects which fields the message filter matches against.
type SearchMode int

const (
	// SearchModeName matches subject + sender. Default.
	SearchModeName SearchMode = iota
	// SearchModeAll matches subject + sender + date text.
	SearchModeAll
)

// SearchState is the lifecycle state of the sidebar search UI.
type SearchState int

const (
	// SearchIdle — no filter, shelf shows hint row.
	SearchIdle SearchState = iota
	// SearchTyping — prompt focused, printable runes append to query,
	// filter updates live on each keystroke.
	SearchTyping
	// SearchActive — query is live but prompt is unfocused; normal
	// account-view key routing resumes.
	SearchActive
)

// SearchUpdatedMsg carries the live search query and mode from
// SidebarSearch up to AccountTab whenever either changes in Typing
// state.
type SearchUpdatedMsg struct {
	Query string
	Mode  SearchMode
}

// bodyLoadedMsg carries the parsed-block representation of a fetched
// message body. AccountTab compares uid against the viewer's current
// UID and drops mismatches (user closed and reopened on a different
// UID before the Cmd resolved).
type bodyLoadedMsg struct {
	uid    mail.UID
	blocks []content.Block
}

// ViewerOpenedMsg signals chrome (footer, status bar) that the viewer
// is now displayed. App switches the footer context and status mode.
type ViewerOpenedMsg struct{}

// ViewerClosedMsg is the inverse: the viewer just closed.
type ViewerClosedMsg struct{}

// ViewerScrollMsg carries the viewer's current scroll position as a
// 0..100 percentage. App routes it to the status bar.
type ViewerScrollMsg struct {
	Pct int
}

// loadBodyCmd fetches a message body, parses it into blocks, and
// delivers a bodyLoadedMsg. Errors fall through as backendErrMsg.
func loadBodyCmd(b mail.Backend, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		r, err := b.FetchBody(uid)
		if err != nil {
			return backendErrMsg{err: err}
		}
		buf, err := io.ReadAll(r)
		if err != nil {
			return backendErrMsg{err: err}
		}
		return bodyLoadedMsg{uid: uid, blocks: content.ParseBlocks(string(buf))}
	}
}

// markReadCmd flips the seen flag on the backend. Errors flow back
// as backendErrMsg so the eventual toast surface can pick them up;
// callers that don't yet have a toast surface drop the message in
// their default handler.
func markReadCmd(b mail.Backend, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		if err := b.MarkRead([]mail.UID{uid}); err != nil {
			return backendErrMsg{err: err}
		}
		return nil
	}
}

// launchURLCmd opens a URL via the openURL hook (xdg-open in
// production, swappable in tests). xdg-open detaches and its exit
// status is unreliable, so errors are intentionally discarded.
func launchURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		_ = openURL(url)
		return nil
	}
}

// viewerOpenedCmd, viewerClosedCmd, viewerScrollCmd are zero-latency
// emit Cmds. Using Cmds (not direct mutation) keeps the chrome
// updates inside the bubbletea Update loop.
func viewerOpenedCmd() tea.Cmd { return func() tea.Msg { return ViewerOpenedMsg{} } }
func viewerClosedCmd() tea.Cmd { return func() tea.Msg { return ViewerClosedMsg{} } }
func viewerScrollCmd(pct int) tea.Cmd {
	return func() tea.Msg { return ViewerScrollMsg{Pct: pct} }
}

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

// folderQueryDoneMsg carries a Query result; AccountTab follows up
// with fetchHeadersCmd to materialize the headers.
type folderQueryDoneMsg struct {
	name  string
	uids  []mail.UID
	total int
	reset bool // true on initial load, false on append
}

// headersAppliedMsg is the terminal message of an initial folder load.
type headersAppliedMsg struct {
	name string
	msgs []mail.MessageInfo
}

// headersAppendedMsg is the terminal message of a load-more.
type headersAppendedMsg struct {
	name string
	msgs []mail.MessageInfo
}

// ErrorMsg carries a failure from any tea.Cmd. App captures the most
// recent ErrorMsg into lastErr; the banner renders "⚠ <Op>: <Err>".
// Last-write-wins: a subsequent ErrorMsg replaces the prior one.
type ErrorMsg struct {
	Op  string
	Err error
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
// backend. The result is delivered as a foldersLoadedMsg, or an
// ErrorMsg on failure.
func loadFoldersCmd(b mail.Backend) tea.Cmd {
	return func() tea.Msg {
		folders, err := b.ListFolders()
		if err != nil {
			return ErrorMsg{Op: "list folders", Err: err}
		}
		return foldersLoadedMsg{folders: folders}
	}
}

// initialWindow is the number of UIDs requested on a fresh folder open.
const initialWindow = 500

// openFolderCmd opens a folder and queries the first window of UIDs.
// The result is a folderQueryDoneMsg{reset:true}, or an ErrorMsg.
// Returns nil when name is empty.
func openFolderCmd(b mail.Backend, name string) tea.Cmd {
	if name == "" {
		return nil
	}
	return func() tea.Msg {
		if err := b.OpenFolder(name); err != nil {
			return ErrorMsg{Op: "open folder", Err: err}
		}
		uids, total, err := b.QueryFolder(name, 0, initialWindow)
		if err != nil {
			return ErrorMsg{Op: "query folder", Err: err}
		}
		return folderQueryDoneMsg{name: name, uids: uids, total: total, reset: true}
	}
}

// loadMoreCmd queries the next window of UIDs starting at offset.
// The result is a folderQueryDoneMsg{reset:false}, or an ErrorMsg.
func loadMoreCmd(b mail.Backend, name string, offset int) tea.Cmd {
	return func() tea.Msg {
		uids, total, err := b.QueryFolder(name, offset, initialWindow)
		if err != nil {
			return ErrorMsg{Op: "load more", Err: err}
		}
		return folderQueryDoneMsg{name: name, uids: uids, total: total, reset: false}
	}
}

// fetchHeadersCmd materializes a UID list into MessageInfo slices.
// On success it returns headersAppliedMsg (reset=true) or
// headersAppendedMsg (reset=false). Errors return ErrorMsg.
func fetchHeadersCmd(b mail.Backend, name string, uids []mail.UID, reset bool) tea.Cmd {
	return func() tea.Msg {
		msgs, err := b.FetchHeaders(uids)
		if err != nil {
			return ErrorMsg{Op: "fetch headers", Err: err}
		}
		if reset {
			return headersAppliedMsg{name: name, msgs: msgs}
		}
		return headersAppendedMsg{name: name, msgs: msgs}
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
// delivers a bodyLoadedMsg. Errors fall through as ErrorMsg.
func loadBodyCmd(b mail.Backend, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		r, err := b.FetchBody(uid)
		if err != nil {
			return ErrorMsg{Op: "fetch body", Err: err}
		}
		buf, err := io.ReadAll(r)
		if err != nil {
			return ErrorMsg{Op: "read body", Err: err}
		}
		return bodyLoadedMsg{uid: uid, blocks: content.ParseBlocks(string(buf))}
	}
}

// markReadCmd flips the seen flag on the backend. Errors flow back
// as ErrorMsg; App captures the most recent into lastErr and renders
// it in the banner above the status bar.
func markReadCmd(b mail.Backend, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		if err := b.MarkRead([]mail.UID{uid}); err != nil {
			return ErrorMsg{Op: "mark read", Err: err}
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

// backendUpdateMsg wraps a single mail.Update in a tea.Msg.
type backendUpdateMsg struct{ update mail.Update }

// pumpUpdatesCmd waits for one mail.Update on the backend channel,
// returns it as a backendUpdateMsg, then re-arms itself. App's
// Update loop is responsible for re-dispatching this Cmd so the
// pump stays alive.
func pumpUpdatesCmd(b mail.Backend) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-b.Updates()
		if !ok {
			return backendUpdateMsg{update: mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnOffline}}
		}
		return backendUpdateMsg{update: u}
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

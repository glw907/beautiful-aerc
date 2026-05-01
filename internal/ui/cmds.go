// SPDX-License-Identifier: MIT

package ui

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"time"

	// Register non-UTF8 charset decoders (iso-8859-1, windows-1252,
	// etc.) into go-message's charset registry. Without this, MIME
	// parts with charset="iso-8859-1" — common for plain-text bodies
	// from Outlook/Exchange senders — fail to decode and the body is
	// silently dropped.
	_ "github.com/emersion/go-message/charset"

	tea "github.com/charmbracelet/bubbletea"
	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/filter"
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

// loadBodyCmd fetches a message body, parses it into blocks, and
// delivers a bodyLoadedMsg. Errors fall through as ErrorMsg.
//
// Real backends return raw RFC822 bytes; the mock returns markdown
// directly. extractDisplayText sniffs the format and walks MIME
// when present, falling back to raw bytes otherwise.
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
		text := extractDisplayText(buf)
		return bodyLoadedMsg{uid: uid, blocks: content.ParseBlocks(text)}
	}
}

// extractDisplayText converts a fetched body buffer into markdown ready
// for content.ParseBlocks. RFC822 input is walked via emersion/go-mail
// to extract the preferred inline text part (text/plain over text/html);
// non-RFC822 input (e.g. the mock backend's pre-cleaned markdown) is
// returned unchanged. The extracted text runs through filter.CleanPlain
// (which auto-routes to CleanHTML when the part is HTML) so the output
// is always normalized markdown.
func extractDisplayText(buf []byte) string {
	if !looksLikeRFC822(buf) {
		return string(buf)
	}
	mr, err := gomail.CreateReader(bytes.NewReader(buf))
	if err != nil {
		return string(buf)
	}
	defer mr.Close()

	var plain, html string
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		ih, ok := p.Header.(*gomail.InlineHeader)
		if !ok {
			io.Copy(io.Discard, p.Body)
			continue
		}
		ct, _, _ := ih.ContentType()
		body, rerr := io.ReadAll(p.Body)
		if rerr != nil {
			continue
		}
		switch ct {
		case "text/plain":
			if plain == "" {
				plain = string(body)
			}
		case "text/html":
			if html == "" {
				html = string(body)
			}
		}
	}
	switch {
	case plain != "":
		return filter.CleanPlain(plain)
	case html != "":
		return filter.CleanHTML(html)
	default:
		return ""
	}
}

// looksLikeRFC822 sniffs whether buf opens with a plausible mail header.
// Header lines have the shape `Field-Name: value`; a blank line ends the
// header block. The check looks at the first non-empty line only.
func looksLikeRFC822(buf []byte) bool {
	s := string(buf)
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	colon := strings.IndexByte(s, ':')
	if colon <= 0 || colon > 78 {
		return false
	}
	for _, r := range s[:colon] {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
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

// openURL is the URL launcher hook. Tests swap it to capture the URL
// instead of executing xdg-open. Shared by viewer numeric quick-launch
// and the link picker.
var openURL = func(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// LinkPickerClosedMsg signals the picker has closed (Esc, Tab, Enter,
// or numeric launch). Handled at the App level to flip linkPicker.open.
type LinkPickerClosedMsg struct{}

// LaunchURLMsg requests App fire launchURLCmd for the given URL.
// Emitted by the link picker on Enter or 1-9 in-range.
type LaunchURLMsg struct {
	URL string
}

// triageStartedMsg is emitted by AccountTab after an optimistic triage
// flip. App receives it, sets the toast, and schedules a tea.Tick for
// the undo timer. inverse runs on `u` or on an ErrorMsg rollback;
// onUndo applies the local MessageList rollback before the inverse
// Cmd fires.
type triageStartedMsg struct {
	op      string
	n       int
	uids    []mail.UID
	inverse tea.Cmd
	onUndo  func()
}

// toastExpireMsg fires when the undo timer elapses. App ignores it if
// deadline does not match the active toast (stale tick from a prior
// generation).
type toastExpireMsg struct {
	deadline time.Time
}

// undoRequestedMsg is emitted when the user presses `u` while a toast
// is active. App applies the local roll-back via onUndo and fires the
// inverse Cmd.
type undoRequestedMsg struct{}

// OpenMovePickerMsg asks App to open the move-to-folder picker.
type OpenMovePickerMsg struct {
	UIDs    []mail.UID
	Src     string
	Folders []FolderEntry
}

// MovePickerPickedMsg is emitted when the user selects a destination folder.
type MovePickerPickedMsg struct {
	UIDs []mail.UID
	Src  string
	Dest string
}

// MovePickerClosedMsg is emitted when the picker is dismissed without a pick.
type MovePickerClosedMsg struct{}

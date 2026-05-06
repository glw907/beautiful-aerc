// SPDX-License-Identifier: MIT

package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	// Register non-UTF8 charset decoders (iso-8859-1, windows-1252,
	// etc.) into go-message's charset registry. Without this, MIME
	// parts with charset="iso-8859-1", common for plain-text bodies
	// from Outlook/Exchange senders — fail to decode and the body is
	// silently dropped.
	_ "github.com/emersion/go-message/charset"

	tea "github.com/charmbracelet/bubbletea"
	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/filter"
	"github.com/glw907/poplar/internal/mail"
)

// foldersLoadedMsg carries the result of an initial sync + ListFolders
// call. The cache emits canonical display names already classified.
type foldersLoadedMsg struct {
	classified []mail.ClassifiedFolder
}

// folderLoadedMsg replaces the previous open→query→fetch chain. The
// cache returns headers in one call.
type folderLoadedMsg struct {
	name  string
	msgs  []mail.MessageInfo
	total int
}

// folderAppendedMsg is the load-more counterpart of folderLoadedMsg.
type folderAppendedMsg struct {
	name  string
	msgs  []mail.MessageInfo
	total int
}

type cacheEventMsg struct{ event cache.CacheEvent }

// ErrorMsg carries a failure from any tea.Cmd. App captures the most
// recent ErrorMsg into lastErr. The banner renders "⚠ <Op>: <Err>".
// Last-write-wins: a subsequent ErrorMsg replaces the prior one.
type ErrorMsg struct {
	Op  string
	Err error
}

// initialWindow is the number of UIDs requested on a fresh folder open.
const initialWindow = 500

// loadFoldersCmd syncs folder metadata from the backend then reads the
// classified list out of the cache.
func loadFoldersCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		if err := c.SyncFolders(context.Background()); err != nil {
			return ErrorMsg{Op: "list folders", Err: err}
		}
		cls, err := c.ListFolders()
		if err != nil {
			return ErrorMsg{Op: "list folders", Err: err}
		}
		return foldersLoadedMsg{classified: cls}
	}
}

// queryFolderCmd reads the first window of cached headers and emits
// a folderLoadedMsg. When sync is true (folder open), the backend is
// nudged to converge first. Sync errors don't fail the load. An empty
// name returns nil so callers can chain without nil-checks.
func queryFolderCmd(c *cache.Account, name string, sync bool) tea.Cmd {
	if name == "" {
		return nil
	}
	op := "refresh"
	if sync {
		op = "query folder"
	}
	return func() tea.Msg {
		if sync {
			_ = c.SyncFolder(context.Background(), name)
		}
		msgs, total, err := c.QueryFolder(name, 0, initialWindow)
		if err != nil {
			return ErrorMsg{Op: op, Err: err}
		}
		return folderLoadedMsg{name: name, msgs: msgs, total: total}
	}
}

// openFolderCmd queries with a backend sync first.
func openFolderCmd(c *cache.Account, name string) tea.Cmd {
	return queryFolderCmd(c, name, true)
}

// refreshFolderCmd queries without sync, picking up the cache's
// already-applied optimistic flip.
func refreshFolderCmd(c *cache.Account, name string) tea.Cmd {
	return queryFolderCmd(c, name, false)
}

// loadMoreCmd returns the next window of cached headers.
func loadMoreCmd(c *cache.Account, name string, offset int) tea.Cmd {
	return func() tea.Msg {
		msgs, total, err := c.QueryFolder(name, offset, initialWindow)
		if err != nil {
			return ErrorMsg{Op: "load more", Err: err}
		}
		return folderAppendedMsg{name: name, msgs: msgs, total: total}
	}
}

// queueOpsCmd enqueues one op per uid in folder, then refreshes.
// makeArgs lets each uid carry its own args (e.g. flag toggles whose
// value is uid-independent return the same args every time).
func queueOpsCmd(c *cache.Account, op, folder string, uids []mail.UID, makeArgs func(mail.UID) cache.OpArgs) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, u := range uids {
			if _, err := c.QueueOp(ctx, folder, u, makeArgs(u)); err != nil {
				return ErrorMsg{Op: op, Err: err}
			}
		}
		msgs, total, err := c.QueryFolder(folder, 0, initialWindow)
		if err != nil {
			return ErrorMsg{Op: op + " refresh", Err: err}
		}
		return folderLoadedMsg{name: folder, msgs: msgs, total: total}
	}
}

// enqueueDestroys queues a destroy op per uid against folder. Shared
// by emptyFolderCmd and destroyCmd. Returns the first error.
func enqueueDestroys(ctx context.Context, c *cache.Account, folder string, uids []mail.UID) error {
	for _, u := range uids {
		if _, err := c.QueueOp(ctx, folder, u, cache.DestroyArgs{}); err != nil {
			return err
		}
	}
	return nil
}

// bodyLoadedMsg carries the parsed-block representation of a fetched
// message body. AccountTab compares uid against the viewer's current
// UID and drops mismatches (user closed and reopened on a different
// UID before the Cmd resolved).
type bodyLoadedMsg struct {
	uid    mail.UID
	blocks []content.Block
}

// loadBodyCmd fetches a message body via the cache (Cache I delegates
// straight to the backend) and parses it into blocks. If ctx is
// cancelled before FetchBody returns the cmd returns nil and the
// result is dropped. The backend round-trip still completes.
func loadBodyCmd(ctx context.Context, c *cache.Account, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		resultCh := make(chan tea.Msg, 1)
		go func() {
			buf, err := c.FetchBody(uid)
			if err != nil {
				resultCh <- ErrorMsg{Op: "fetch body", Err: err}
				return
			}
			// Sniff the buffer for an RFC 822 header line ("Field-Name: value"
			// before the first newline). Non-RFC822 input (e.g. mock backend's
			// pre-cleaned markdown) is forwarded unchanged.
			isRFC822 := func(b []byte) bool {
				s := string(b)
				if i := strings.IndexByte(s, '\n'); i > 0 {
					s = s[:i]
				}
				colon := strings.IndexByte(s, ':')
				if colon <= 0 || colon > 78 {
					return false
				}
				for _, r := range s[:colon] {
					if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
						(r >= '0' && r <= '9') || r == '-' || r == '_') {
						return false
					}
				}
				return true
			}
			text := string(buf)
			if isRFC822(buf) {
				if mr, mrErr := gomail.CreateReader(bytes.NewReader(buf)); mrErr == nil {
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
					mr.Close()
					switch {
					case plain != "":
						text = filter.CleanPlain(plain)
					case html != "":
						text = filter.CleanHTML(html)
					default:
						text = ""
					}
				}
			}
			resultCh <- bodyLoadedMsg{uid: uid, blocks: content.ParseBlocks(text)}
		}()
		select {
		case <-ctx.Done():
			return nil
		case msg := <-resultCh:
			return msg
		}
	}
}

// markReadCmd queues an optimistic FlagSeen=true op for uid against
// folder, then re-reads the folder so the read-state flip surfaces.
func markReadCmd(c *cache.Account, folder string, uid mail.UID) tea.Cmd {
	return queueOpsCmd(c, "mark read", folder, []mail.UID{uid}, func(_ mail.UID) cache.OpArgs {
		return cache.FlagArgs{Flag: mail.FlagSeen, Set: true}
	})
}

// URLOpener launches a URL in the user's browser. App holds one;
// tests inject a stub via App.WithOpener.
type URLOpener func(string) error

// launchURLCmd opens url via opener. xdg-open detaches and its exit
// status is unreliable, so errors are intentionally discarded.
func launchURLCmd(opener URLOpener, url string) tea.Cmd {
	return func() tea.Msg {
		_ = opener(url)
		return nil
	}
}

// xdgOpenURL is the default URLOpener: shells out to xdg-open.
func xdgOpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
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

// pumpCacheCmd waits for one CacheEvent and re-arms itself. App's
// Update loop re-dispatches this Cmd after each event so the pump
// stays alive across the program lifetime.
func pumpCacheCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-c.Events()
		if !ok {
			return nil
		}
		return cacheEventMsg{event: ev}
	}
}

// ClearSidebarSearchMsg tells AccountTab to clear an active sidebar
// search shelf (Active or Typing). Used by App when q is pressed
// while search is non-idle so the keypress doesn't fall through to
// quit and the search gets a deterministic clear path.
type ClearSidebarSearchMsg struct{}

// OpenLinkPickerMsg requests App open the link picker with the given
// harvested URLs. Emitted by Viewer when the user presses Tab on a
// message that has at least one harvested link.
type OpenLinkPickerMsg struct {
	Links []string
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
// the undo timer. inverse runs on `u`: a compensating QueueOp via
// queueOpsCmd. The cache owns the optimistic state, so there is no
// onUndo callback. undo is the inverse Cmd alone.
type triageStartedMsg struct {
	op      triageOp
	n       int
	dest    string
	uids    []mail.UID
	inverse tea.Cmd
}

// toastExpireMsg fires when the undo timer elapses. App ignores it if
// deadline does not match the active toast (stale tick from a prior
// generation).
type toastExpireMsg struct {
	deadline time.Time
}

// undoRequestedMsg is emitted when the user presses `u` while a toast
// is active. App fires the inverse Cmd.
type undoRequestedMsg struct{}

// OpenConfirmEmptyMsg asks App to open the empty-folder confirm modal.
// Source is passed through so it can be handed to emptyFolderCmd later.
type OpenConfirmEmptyMsg struct {
	Folder string // display name shown in modal title and toast
	Total  int    // message count shown in modal body
	Source string // canonical folder name passed to QueueOp
}

// EmptyFolderConfirmedMsg signals the user pressed `y` in the confirm modal.
type EmptyFolderConfirmedMsg struct {
	Folder string
	Source string
}

// ConfirmModalClosedMsg signals the modal was dismissed without confirmation.
type ConfirmModalClosedMsg struct{}

// emptyFolderCmd pages the cache for every UID in src, queues a
// destroy op per UID, then re-reads. Bypasses the undo bar (emitting
// triageStartedMsg with op = "empty" so the toast suppresses the [u]
// hint per ADR-0094).
func emptyFolderCmd(c *cache.Account, displayName, src string) tea.Cmd {
	return func() tea.Msg {
		op := "empty " + strings.ToLower(displayName)
		ctx := context.Background()
		var all []mail.UID
		const page = 1000
		for offset := 0; ; {
			msgs, total, err := c.QueryFolder(src, offset, page)
			if err != nil {
				return ErrorMsg{Op: op, Err: err}
			}
			for _, m := range msgs {
				all = append(all, m.UID)
			}
			offset += len(msgs)
			if len(msgs) == 0 || offset >= total {
				break
			}
		}
		if err := enqueueDestroys(ctx, c, src, all); err != nil {
			return ErrorMsg{Op: op, Err: err}
		}
		return emptyFolderDoneMsg{folder: displayName, source: src, n: len(all)}
	}
}

// emptyFolderDoneMsg reports a successful manual empty.
type emptyFolderDoneMsg struct {
	folder string
	source string
	n      int
}

// destroyCmd queues per-UID destroy ops for the retention sweep.
// Empty input skips the queue.
func destroyCmd(c *cache.Account, folder string, uids []mail.UID) tea.Cmd {
	return func() tea.Msg {
		if len(uids) == 0 {
			return sweepCompletedMsg{folder: folder}
		}
		if err := enqueueDestroys(context.Background(), c, folder, uids); err != nil {
			return ErrorMsg{Op: "purge expired", Err: err}
		}
		return sweepCompletedMsg{folder: folder, uids: uids}
	}
}

// sweepCompletedMsg reports a retention sweep's destroyed UIDs.
type sweepCompletedMsg struct {
	folder string
	uids   []mail.UID
}

// outboxDepthMsg carries the latest cache.OutboxDepth into the App
// for status-bar refresh.
type outboxDepthMsg struct{ depth cache.OutboxDepth }

// outboxSummaryMsg refreshes the open Q overlay.
type outboxSummaryMsg struct {
	groups []cache.OutboxGroup
	err    error
}

// outboxConflictsMsg refreshes the open ! overlay.
type outboxConflictsMsg struct {
	rows []cache.ConflictRow
	err  error
}

// conflictResolvedMsg carries the result of a Retry / Discard call.
type conflictResolvedMsg struct {
	opID int64
	err  error
}

func refreshOutboxDepthCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		d, err := c.OutboxDepth(ctx)
		if err != nil {
			return ErrorMsg{Op: "outbox depth", Err: err}
		}
		return outboxDepthMsg{depth: d}
	}
}

func loadOutboxSummaryCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		gs, err := c.OutboxSummary(ctx)
		return outboxSummaryMsg{groups: gs, err: err}
	}
}

func loadOutboxConflictsCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		rs, err := c.OutboxConflicts(ctx)
		return outboxConflictsMsg{rows: rs, err: err}
	}
}

func retryConflictCmd(c *cache.Account, opID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := c.RetryOp(ctx, opID)
		return conflictResolvedMsg{opID: opID, err: err}
	}
}

func discardConflictCmd(c *cache.Account, opID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := c.DiscardOp(ctx, opID)
		return conflictResolvedMsg{opID: opID, err: err}
	}
}

// attachmentsLoadedMsg carries metadata for the viewer's current UID.
// Stale UIDs are dropped at the AccountTab boundary like bodyLoadedMsg.
type attachmentsLoadedMsg struct {
	uid   mail.UID
	items []mail.Attachment
}

type OpenAttachPickerMsg struct {
	UID   mail.UID
	Items []mail.Attachment
}

type AttachPickerClosedMsg struct{}

type OpenAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}

type SaveAttachmentMsg struct {
	UID mail.UID
	Att mail.Attachment
}

type attachmentSavedMsg struct {
	path string
}

// loadAttachmentsCmd resolves attachment metadata via the cache.
// Errors route through the standard ErrorMsg banner. Stale-UID
// drops happen at the AccountTab boundary.
func loadAttachmentsCmd(c *cache.Account, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		items, err := c.Attachments(context.Background(), uid)
		if err != nil {
			return ErrorMsg{Op: "fetch attachments", Err: err}
		}
		return attachmentsLoadedMsg{uid: uid, items: items}
	}
}

// sanitizeAttachFilename strips path separators and falls back to a
// stable name keyed on partID when the attachment has no filename.
func sanitizeAttachFilename(name, partID string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "attachment-" + partID
	}
	return name
}

// resolveSaveTarget returns the first non-existing path in dir
// derived from base, suffixing -1, -2, ... before the extension.
// Caps at 999 to avoid pathological loops.
func resolveSaveTarget(dir, base string) (string, error) {
	candidate := filepath.Join(dir, base)
	if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
		return candidate, nil
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i <= 999; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("collision suffix exhausted for %q", base)
}

// openAttachmentCmd writes att's bytes to a tempfile and shells out
// to the URLOpener (xdg-open). Fire-and-forget. Errors surface via ErrorMsg.
func openAttachmentCmd(c *cache.Account, opener URLOpener, uid mail.UID, att mail.Attachment) tea.Cmd {
	return func() tea.Msg {
		body, err := c.FetchAttachment(context.Background(), uid, att.PartID)
		if err != nil {
			return ErrorMsg{Op: "open attachment", Err: err}
		}
		name := sanitizeAttachFilename(att.Filename, att.PartID)
		path := filepath.Join(os.TempDir(), fmt.Sprintf("poplar-%s-%s", uid, name))
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return ErrorMsg{Op: "open attachment", Err: err}
		}
		_ = opener(path)
		return nil
	}
}

type composeSeedKind int

const (
	composeSeedReply composeSeedKind = iota
	composeSeedReplyAll
	composeSeedForward
)

// composeSeedCmd fetches the parent body and builds a Draft via the
// matching compose.Seed* function. Result lands as composeSeededMsg.
func composeSeedCmd(acct *cache.Account, parent mail.MessageInfo, self string, kind composeSeedKind) tea.Cmd {
	return func() tea.Msg {
		body, err := acct.FetchBody(parent.UID)
		if err != nil {
			return ErrorMsg{Op: "fetch parent body", Err: err}
		}
		var d compose.Draft
		switch kind {
		case composeSeedReply:
			d = compose.SeedReply(parent, body)
		case composeSeedReplyAll:
			d = compose.SeedReplyAll(parent, body, gomail.Address{Address: self})
		default:
			d = compose.SeedForward(parent, body)
		}
		d.From = gomail.Address{Address: self}
		return composeSeededMsg{Draft: d}
	}
}

// composeSeededMsg carries a pre-filled Draft from r/R/f. App opens
// ComposeTab and calls Seed when this msg arrives.
type composeSeededMsg struct {
	Draft compose.Draft
}

// composeSendCmd runs the tidy seam, assembles MIME, and queues the
// outbox op via cache.Account.QueueOutbound. Returns ErrorMsg on any
// failure, composeSentMsg on success.
func composeSendCmd(acct *cache.Account, sentFolder string, tidy TidyFn, d compose.Draft) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		body, err := tidy(ctx, d.Body)
		if err != nil {
			return ErrorMsg{Op: "tidy body", Err: err}
		}
		d.Body = body
		mime, err := compose.AssembleMIME(d, time.Now())
		if err != nil {
			return ErrorMsg{Op: "assemble MIME", Err: err}
		}
		env := envelopeFromDraft(d)
		if err := acct.QueueOutbound(ctx, sentFolder, env, mime); err != nil {
			return ErrorMsg{Op: "queue outbound", Err: err}
		}
		return composeSentMsg{}
	}
}

func envelopeFromDraft(d compose.Draft) mail.Envelope {
	env := mail.Envelope{From: d.From.Address}
	for _, a := range d.To {
		env.Rcpts = append(env.Rcpts, a.Address)
	}
	for _, a := range d.Cc {
		env.Rcpts = append(env.Rcpts, a.Address)
	}
	for _, a := range d.Bcc {
		env.Rcpts = append(env.Rcpts, a.Address)
	}
	return env
}

// composeSentMsg fires after QueueOutbound returns. App stages a
// non-undoable "Sending…" toast.
type composeSentMsg struct{}

// resolveSentFolder picks the Sent folder for outbound mail from the
// cached folder list. Returns "" if none can be identified.
func resolveSentFolder(acct *cache.Account) string {
	classified, err := acct.ListFolders()
	if err != nil {
		return ""
	}
	for _, cf := range classified {
		if cf.Canonical == "Sent" {
			return cf.Folder.Name
		}
	}
	for _, cf := range classified {
		if strings.EqualFold(cf.Folder.Name, "Sent") {
			return cf.Folder.Name
		}
	}
	return ""
}

// saveAttachmentCmd writes att's bytes to dir with collision-suffix
// resolution and emits attachmentSavedMsg with the final path.
func saveAttachmentCmd(c *cache.Account, dir string, uid mail.UID, att mail.Attachment) tea.Cmd {
	return func() tea.Msg {
		if dir == "" {
			return ErrorMsg{Op: "save attachment", Err: fmt.Errorf("no download dir configured")}
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		body, err := c.FetchAttachment(context.Background(), uid, att.PartID)
		if err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		name := sanitizeAttachFilename(att.Filename, att.PartID)
		target, err := resolveSaveTarget(dir, name)
		if err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			return ErrorMsg{Op: "save attachment", Err: err}
		}
		return attachmentSavedMsg{path: target}
	}
}

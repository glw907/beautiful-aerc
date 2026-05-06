// SPDX-License-Identifier: MIT

package ui

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/mail"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/reader"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// ErrorMsg aliases uicore.ErrorMsg so App-side cmds and the banner
// consumer keep their unqualified spelling. The canonical declaration
// is in uicore. Account-subpackage cmds emit uicore.ErrorMsg directly.
type ErrorMsg = uicore.ErrorMsg

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

// toastExpireMsg fires when the undo timer elapses. App ignores it if
// deadline does not match the active toast (stale tick from a prior
// generation).
type toastExpireMsg struct {
	deadline time.Time
}

// undoRequestedMsg is emitted when the user presses `u` while a toast
// is active. App fires the inverse Cmd.
type undoRequestedMsg struct{}

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

// openDraftMsg is returned by openDraftFromServerUIDCmd when a DraftRow is
// resolved (either from the local cache or freshly reconstructed). Draft is
// the already-decoded value so the App handler is decode-free.
type openDraftMsg struct {
	row   cache.DraftRow
	draft compose.Draft
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

// saveAttachmentCmd writes att's bytes to dir with collision-suffix
// resolution and emits reader.AttachmentSavedMsg with the final path.
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
		return reader.AttachmentSavedMsg{Path: target}
	}
}

// composeSeedCmd fetches the parent body and builds a Draft via the
// matching compose.Seed* function. Result lands as uicompose.SeededMsg.
func composeSeedCmd(acct *cache.Account, parent mail.MessageInfo, self string, kind uicompose.SeedKind) tea.Cmd {
	return func() tea.Msg {
		body, err := acct.FetchBody(parent.UID)
		if err != nil {
			return ErrorMsg{Op: "fetch parent body", Err: err}
		}
		var d compose.Draft
		switch kind {
		case uicompose.SeedReply:
			d = compose.SeedReply(parent, body)
		case uicompose.SeedReplyAll:
			d = compose.SeedReplyAll(parent, body, gomail.Address{Address: self})
		default:
			d = compose.SeedForward(parent, body)
		}
		d.From = gomail.Address{Address: self}
		return uicompose.SeededMsg{Draft: d}
	}
}

// composeSendCmd runs the tidy seam, assembles MIME, and queues the
// outbox op via cache.Account.QueueOutbound. Returns ErrorMsg on any
// failure, uicompose.SentMsg on success.
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
		return uicompose.SentMsg{}
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

// resolveDraftsFolder returns the backend folder name for the Drafts
// canonical, or "" if none can be identified.
func resolveDraftsFolder(acct *cache.Account) string {
	classified, err := acct.ListFolders()
	if err != nil {
		return ""
	}
	for _, cf := range classified {
		if cf.Canonical == "Drafts" {
			return cf.Folder.Name
		}
	}
	for _, cf := range classified {
		if strings.EqualFold(cf.Folder.Name, "Drafts") {
			return cf.Folder.Name
		}
	}
	return ""
}

// enqueuePushDraftCmd forwards a push request from compose to the outbox.
func enqueuePushDraftCmd(acct *cache.Account, draftID, folder string, mime []byte, prevUID mail.UID) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if _, err := acct.QueuePushDraft(ctx, draftID, folder, mime, prevUID); err != nil {
			return ErrorMsg{Op: "queue push draft", Err: err}
		}
		return nil
	}
}

// discardDraftCmd deletes the local draft row and, when prevUID is set,
// queues a server-side Destroy so the stale image is removed.
func discardDraftCmd(acct *cache.Account, draftID, draftsFolder string, prevUID mail.UID) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if prevUID != "" && draftsFolder != "" {
			if _, err := acct.QueueOp(ctx, draftsFolder, prevUID, cache.DestroyArgs{}); err != nil {
				return ErrorMsg{Op: "queue destroy draft", Err: err}
			}
		}
		if err := acct.DeleteDraft(ctx, draftID); err != nil {
			return ErrorMsg{Op: "delete draft", Err: err}
		}
		return nil
	}
}

// upsertAndPushDraftCmd persists draft payload and enqueues a server push.
// Used by the save-on-close path.
func upsertAndPushDraftCmd(acct *cache.Account, draftID, draftsFolder string, d compose.Draft, prevUID mail.UID) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		payload, err := compose.EncodeDraft(d)
		if err != nil {
			return ErrorMsg{Op: "encode draft", Err: err}
		}
		if err := acct.CreateDraft(ctx, draftID, payload); err != nil {
			return ErrorMsg{Op: "save draft", Err: err}
		}
		mime, err := compose.AssembleMIME(d, time.Now())
		if err != nil {
			return ErrorMsg{Op: "assemble draft MIME", Err: err}
		}
		if _, err := acct.QueuePushDraft(ctx, draftID, draftsFolder, mime, prevUID); err != nil {
			return ErrorMsg{Op: "queue push draft", Err: err}
		}
		return nil
	}
}

// openDraftFromServerUIDCmd looks up the local DraftRow for uid. When
// found it emits openDraftMsg with that row. When not found it fetches
// the raw bytes from the cache (body fetch), parses them via
// ParseDraftMIME, and emits openDraftMsg with a freshly allocated row
// so the App can CreateDraft and open compose. A fetch error emits
// uicore.ErrorMsg.
func openDraftFromServerUIDCmd(acct *cache.Account, uid mail.UID, draftsFolder string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		row, err := acct.LookupDraftByServerUID(ctx, uid)
		if err == nil {
			d, derr := compose.DecodeDraft(row.Payload)
			if derr != nil {
				return uicore.ErrorMsg{Op: "decode draft", Err: derr}
			}
			return openDraftMsg{row: row, draft: d}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return uicore.ErrorMsg{Op: "open draft", Err: err}
		}
		// No local row. Reconstruct from the server image.
		raw, err := acct.FetchBody(uid)
		if err != nil {
			return uicore.ErrorMsg{Op: "fetch draft body", Err: err}
		}
		d, err := compose.ParseDraftMIME(raw)
		if err != nil {
			return uicore.ErrorMsg{Op: "parse draft", Err: err}
		}
		payload, err := compose.EncodeDraft(d)
		if err != nil {
			return uicore.ErrorMsg{Op: "encode draft", Err: err}
		}
		newID := uicompose.AllocDraftID()
		if err := acct.CreateDraft(ctx, newID, payload); err != nil {
			return uicore.ErrorMsg{Op: "save draft", Err: err}
		}
		if err := acct.MarkDraftPushed(ctx, newID, uid, draftsFolder); err != nil {
			return uicore.ErrorMsg{Op: "mark draft pushed", Err: err}
		}
		row = cache.DraftRow{
			DraftID:      newID,
			ServerUID:    uid,
			ServerFolder: draftsFolder,
			Payload:      payload,
		}
		return openDraftMsg{row: row, draft: d}
	}
}

// draftLocalID reports whether uid carries the "draft:" prefix and
// returns the local draft ID if so.
func draftLocalID(uid mail.UID) (string, bool) {
	s := string(uid)
	after, ok := strings.CutPrefix(s, "draft:")
	return after, ok
}

// openLocalDraftCmd loads a locally-stored draft by its draftID
// and emits openDraftMsg so the App's handler can mount compose.
func openLocalDraftCmd(acct *cache.Account, draftID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		payload, err := acct.LoadDraft(ctx, draftID)
		if err != nil {
			return uicore.ErrorMsg{Op: "load draft", Err: err}
		}
		d, err := compose.DecodeDraft(payload)
		if err != nil {
			return uicore.ErrorMsg{Op: "decode draft", Err: err}
		}
		return openDraftMsg{
			row:   cache.DraftRow{DraftID: draftID, Payload: payload},
			draft: d,
		}
	}
}

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

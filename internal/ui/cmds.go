package ui

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/cache"
	corecontacts "github.com/glw907/poplar/internal/contacts"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailcompose"
	uicompose "github.com/glw907/poplar/internal/ui/compose"
	"github.com/glw907/poplar/internal/ui/reader"
	"github.com/glw907/poplar/internal/ui/uicore"
	"github.com/google/uuid"
)

// ErrorMsg aliases uicore.ErrorMsg so App-side cmds and the banner
// consumer keep their unqualified spelling. The canonical declaration
// is in uicore; account cmds emit uicore.ErrorMsg directly.
type ErrorMsg = uicore.ErrorMsg

// URLOpener launches a URL in the user's browser. App holds one; tests
// inject a stub via App.WithOpener.
type URLOpener func(string) error

// httpClient is the shared client for one-click unsubscribe POSTs.
// Timeout is intentionally zero; every call sets a context deadline.
var httpClient = &http.Client{}

// launchURLCmd opens url via opener. xdg-open detaches and its exit
// status is unreliable, so errors are intentionally dropped.
func launchURLCmd(opener URLOpener, url string) tea.Cmd {
	return func() tea.Msg {
		_ = opener(url)
		return nil
	}
}

// UnsubscribeDoneMsg fires on a successful (2xx) one-click POST.
// Failures surface as ErrorMsg and never reach this type.
type UnsubscribeDoneMsg struct {
	Host string
}

// unsubscribePostCmd issues an RFC 8058 one-click POST with body
// "List-Unsubscribe=One-Click" and a 10-second context deadline.
// 2xx → UnsubscribeDoneMsg; anything else → ErrorMsg.
func unsubscribePostCmd(rawURL string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(rawURL)
		if err != nil {
			return ErrorMsg{Op: "unsubscribe", Err: err}
		}
		body := strings.NewReader("List-Unsubscribe=One-Click")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
		if err != nil {
			return ErrorMsg{Op: "unsubscribe", Err: err}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := httpClient.Do(req)
		if err != nil {
			return ErrorMsg{Op: "unsubscribe", Err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ErrorMsg{
				Op:  "unsubscribe",
				Err: fmt.Errorf("server returned %s", resp.Status),
			}
		}
		return UnsubscribeDoneMsg{Host: u.Host}
	}
}

// xdgOpenURL is the default URLOpener; it shells out to xdg-open.
func xdgOpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// backendUpdateMsg wraps a single mail.Update in a tea.Msg.
type backendUpdateMsg struct{ update mail.Update }

// pumpUpdatesCmd waits for one mail.Update and returns it as a msg.
// App.Update re-dispatches after each event so the pump stays alive.
func pumpUpdatesCmd(b mail.Backend) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-b.Updates()
		if !ok {
			return backendUpdateMsg{update: mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnOffline}}
		}
		return backendUpdateMsg{update: u}
	}
}

// toastExpireMsg fires when the undo timer elapses. App ignores it
// when deadline does not match the active toast (stale tick from a
// prior generation).
type toastExpireMsg struct {
	deadline time.Time
}

// undoRequestedMsg fires on `u` while a toast is active. App fires the
// inverse Cmd.
type undoRequestedMsg struct{}

// undoCountdownTickMsg is the 1Hz nudge that causes App to re-render
// the send-undo countdown banner.
type undoCountdownTickMsg struct{}

func undoCountdownTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return undoCountdownTickMsg{} })
}

// RestoreFromDraftMsg fires after undo-send has cancelled the outbound.
// App reopens compose seeded from the in-memory Draft.
type RestoreFromDraftMsg struct {
	Draft mailcompose.Draft
}

// undoSendCmd cancels the queued outbox ops and, on success, emits
// RestoreFromDraftMsg so App reopens compose with the original Draft.
func undoSendCmd(acct *cache.Account, opIDs []int64, draft mailcompose.Draft) tea.Cmd {
	return func() tea.Msg {
		if err := acct.CancelOps(context.Background(), opIDs); err != nil {
			if errors.Is(err, cache.ErrNotPending) {
				return ErrorMsg{Op: "undo send", Err: errors.New("already sent")}
			}
			return ErrorMsg{Op: "undo send", Err: err}
		}
		return RestoreFromDraftMsg{Draft: draft}
	}
}

// outboxDepthMsg refreshes the App's status-bar segment.
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

type conflictResolvedMsg struct {
	opID int64
	err  error
}

// openDraftMsg is returned by openDraftFromServerUIDCmd when a DraftRow
// is resolved (cache hit or freshly reconstructed). Draft is already
// decoded so the App handler is decode-free.
type openDraftMsg struct {
	row   cache.DraftRow
	draft mailcompose.Draft
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

// resolveSaveTarget returns the first non-existing path in dir derived
// from base, suffixing -1, -2, ... before the extension. Bounded at 999
// to avoid pathological loops.
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

// openAttachmentCmd writes att to a tempfile and shells out to the
// URLOpener (xdg-open). Fire-and-forget; fetch and write errors surface
// via ErrorMsg, opener errors are dropped.
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
// matching mailcompose.Seed* function, emitting uicompose.SeededMsg.
func composeSeedCmd(acct *cache.Account, parent mail.MessageInfo, self string, kind uicompose.SeedKind) tea.Cmd {
	return func() tea.Msg {
		body, err := acct.FetchBody(parent.UID)
		if err != nil {
			return ErrorMsg{Op: "fetch parent body", Err: err}
		}
		var d mailcompose.Draft
		switch kind {
		case uicompose.SeedReply:
			d = mailcompose.SeedReply(parent, body)
		case uicompose.SeedReplyAll:
			d = mailcompose.SeedReplyAll(parent, body, gomail.Address{Address: self})
		default:
			d = mailcompose.SeedForward(parent, body)
		}
		d.From = gomail.Address{Address: self}
		return uicompose.SeededMsg{Draft: d}
	}
}

// composeSendCmd assembles MIME, persists a drafts row, and queues the
// outbox op via cache.Account.QueueOutbound. Emits uicompose.SentMsg on
// success, ErrorMsg on any failure. userScheduled overrides the undo window
// when non-zero (schedule-send path).
func composeSendCmd(acct *cache.Account, sentFolder string, d mailcompose.Draft, ids []mailcompose.Identity, undoWindow time.Duration, userScheduled time.Time) tea.Cmd {
	return func() tea.Msg {
		mime, err := mailcompose.AssembleMIME(d, ids, time.Now())
		if err != nil {
			return ErrorMsg{Op: "assemble MIME", Err: err}
		}

		draftID := uuid.NewString()
		if err := acct.CreateDraft(context.Background(), draftID, mime); err != nil {
			return ErrorMsg{Op: "persist draft", Err: err}
		}

		var scheduledFor time.Time
		switch {
		case !userScheduled.IsZero():
			scheduledFor = userScheduled
		case undoWindow > 0:
			scheduledFor = time.Now().Add(undoWindow)
		}

		var scheduledNanos int64
		if !scheduledFor.IsZero() {
			scheduledNanos = scheduledFor.UnixNano()
		}

		opIDs, err := acct.QueueOutbound(context.Background(), sentFolder, envelopeFromDraft(d), mime, scheduledNanos, draftID)
		if err != nil {
			_ = acct.DeleteDraft(context.Background(), draftID)
			return ErrorMsg{Op: "queue outbound", Err: err}
		}

		return uicompose.SentMsg{
			OpIDs:        opIDs,
			ScheduledFor: scheduledFor,
			Draft:        d,
		}
	}
}

func envelopeFromDraft(d mailcompose.Draft) mail.Envelope {
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
// canonical, or "" when none can be identified.
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

func enqueuePushDraftCmd(acct *cache.Account, draftID, folder string, mime []byte, prevUID mail.UID) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if _, err := acct.QueuePushDraft(ctx, draftID, folder, mime, prevUID); err != nil {
			return ErrorMsg{Op: "queue push draft", Err: err}
		}
		return nil
	}
}

// discardDraftCmd deletes the local draft row and queues a server-side
// Destroy when prevUID is set so the stale image is cleaned up.
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

// upsertAndPushDraftCmd persists draft payload and enqueues a server
// push, used by the save-on-close path.
func upsertAndPushDraftCmd(acct *cache.Account, draftID, draftsFolder string, d mailcompose.Draft, prevUID mail.UID, ids []mailcompose.Identity) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		payload, err := mailcompose.EncodeDraft(d)
		if err != nil {
			return ErrorMsg{Op: "encode draft", Err: err}
		}
		if err := acct.CreateDraft(ctx, draftID, payload); err != nil {
			return ErrorMsg{Op: "save draft", Err: err}
		}
		mime, err := mailcompose.AssembleMIME(d, ids, time.Now())
		if err != nil {
			return ErrorMsg{Op: "assemble draft MIME", Err: err}
		}
		if _, err := acct.QueuePushDraft(ctx, draftID, draftsFolder, mime, prevUID); err != nil {
			return ErrorMsg{Op: "queue push draft", Err: err}
		}
		return nil
	}
}

// openDraftFromServerUIDCmd resolves a DraftRow for uid. On a local hit
// it emits openDraftMsg with that row; on miss it fetches the raw bytes,
// parses via ParseDraftMIME, allocates a fresh row, and emits
// openDraftMsg. Fetch and parse errors surface via uicore.ErrorMsg.
func openDraftFromServerUIDCmd(acct *cache.Account, uid mail.UID, draftsFolder string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		row, err := acct.LookupDraftByServerUID(ctx, uid)
		if err == nil {
			d, derr := mailcompose.DecodeDraft(row.Payload)
			if derr != nil {
				return uicore.ErrorMsg{Op: "decode draft", Err: derr}
			}
			return openDraftMsg{row: row, draft: d}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return uicore.ErrorMsg{Op: "open draft", Err: err}
		}
		// Reconstruct from the server image when no local row exists.
		raw, err := acct.FetchBody(uid)
		if err != nil {
			return uicore.ErrorMsg{Op: "fetch draft body", Err: err}
		}
		d, err := mailcompose.ParseDraftMIME(raw)
		if err != nil {
			return uicore.ErrorMsg{Op: "parse draft", Err: err}
		}
		payload, err := mailcompose.EncodeDraft(d)
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
// returns the local draft ID when it does.
func draftLocalID(uid mail.UID) (string, bool) {
	s := string(uid)
	after, ok := strings.CutPrefix(s, "draft:")
	return after, ok
}

// openLocalDraftCmd loads a locally-stored draft by draftID and emits
// openDraftMsg so the App handler can mount mailcompose.
func openLocalDraftCmd(acct *cache.Account, draftID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		payload, err := acct.LoadDraft(ctx, draftID)
		if err != nil {
			return uicore.ErrorMsg{Op: "load draft", Err: err}
		}
		d, err := mailcompose.DecodeDraft(payload)
		if err != nil {
			return uicore.ErrorMsg{Op: "decode draft", Err: err}
		}
		return openDraftMsg{
			row:   cache.DraftRow{DraftID: draftID, Payload: payload},
			draft: d,
		}
	}
}

type contactsSyncedMsg struct{}
type contactsTickMsg struct{}

// syncContactsCmd runs one CardDAV sync pass. Errors surface through the
// standard ErrorMsg banner.
func syncContactsCmd(acct *cache.Account, cfg *corecontacts.ClientConfig) tea.Cmd {
	return func() tea.Msg {
		if err := acct.SyncContacts(context.Background(), cfg); err != nil {
			return ErrorMsg{Op: "sync contacts", Err: err}
		}
		return contactsSyncedMsg{}
	}
}

// scheduleSyncCmd fires another sync after d elapses.
func scheduleSyncCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return contactsTickMsg{} })
}

// resolveSentFolder picks the Sent folder for outbound mail from the
// cached folder list, or "" when none can be identified.
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

// outboxScheduledMsg carries the result of OutboxScheduled for the outbox view.
type outboxScheduledMsg struct {
	rows []cache.OutboxRow
	err  error
}

func loadOutboxScheduledCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		rows, err := c.OutboxScheduled(ctx)
		return outboxScheduledMsg{rows: rows, err: err}
	}
}

// outboxCancelledMsg reports the result of CancelOps from the outbox view.
type outboxCancelledMsg struct {
	err error
}

func cancelOutboxOpCmd(c *cache.Account, opID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := c.CancelOps(ctx, []int64{opID})
		return outboxCancelledMsg{err: err}
	}
}

// rescheduleOpMsg reports the result of RescheduleOp.
type rescheduleOpMsg struct {
	err error
}

func rescheduleOpCmd(c *cache.Account, opID int64, when time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := c.RescheduleOp(ctx, opID, when.UnixNano())
		return rescheduleOpMsg{err: err}
	}
}

// editAsDraftCmd cancels the outbox op and opens compose seeded from the
// linked draft. The draft payload must be non-nil; callers gate on that.
func editAsDraftCmd(c *cache.Account, opID int64, draft *cache.DraftRow) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// Cancel the outbound op first. ErrNotPending means it already
		// dispatched; treat as a no-op so the user at least gets compose open
		// with the stale draft text.
		if err := c.CancelOps(ctx, []int64{opID}); err != nil && !errors.Is(err, cache.ErrNotPending) {
			return ErrorMsg{Op: "cancel op", Err: err}
		}
		d, err := mailcompose.DecodeDraft(draft.Payload)
		if err != nil {
			return ErrorMsg{Op: "decode draft", Err: err}
		}
		return RestoreFromDraftMsg{Draft: d}
	}
}

// queueContactPutCmd patches (or builds) the vCard for contact and enqueues
// a CardDAV PUT through the cache outbox. uid=="" means new contact.
// Multi-book selection is post-1.0; uses the default book.
func queueContactPutCmd(c *cache.Account, uid string, contact corecontacts.Contact) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		bookHref, err := c.DefaultBookHref(ctx)
		if err != nil {
			return uicore.ErrorMsg{Op: "save contact", Err: err}
		}
		var (
			vcardBytes []byte
			href       string
			ifMatch    string
		)
		if uid == "" {
			uid = uuid.NewString()
			vcardBytes, err = corecontacts.BuildVCard(contact, uid, time.Now())
			if err != nil {
				return uicore.ErrorMsg{Op: "save contact", Err: err}
			}
		} else {
			stored, lerr := c.LoadStoredVCard(ctx, uid)
			if lerr != nil {
				return uicore.ErrorMsg{Op: "save contact", Err: lerr}
			}
			ifMatch = stored.ETag
			href = stored.Href
			vcardBytes, err = corecontacts.PatchVCard(stored.Raw, contact, time.Now())
			if err != nil {
				return uicore.ErrorMsg{Op: "save contact", Err: err}
			}
		}
		args := cache.ContactPutArgs{BookHref: bookHref, Href: href, IfMatch: ifMatch}
		if err := c.QueueContactPut(ctx, uid, contact, args, vcardBytes); err != nil {
			return uicore.ErrorMsg{Op: "save contact", Err: err}
		}
		return nil
	}
}

// queueContactDeleteCmd queues a CardDAV DELETE for uid.
func queueContactDeleteCmd(c *cache.Account, uid string) tea.Cmd {
	return func() tea.Msg {
		if err := c.QueueContactDelete(context.Background(), uid); err != nil {
			return uicore.ErrorMsg{Op: "delete contact", Err: err}
		}
		return nil
	}
}

// coalesceTimerMsg fires 1s after the first arrival in a coalesce
// window so any pending new-mail toast can render with the accumulated
// count.
type coalesceTimerMsg struct{}

func coalesceNewMailCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return coalesceTimerMsg{} })
}

// noticeExpireMsg fires when a transient notice's window elapses.
// The handler still gates on m.lastNoticeDeadline so a stale tick
// from a superseded notice can't clear a fresh one.
type noticeExpireMsg struct{}

func clearNoticeAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return noticeExpireMsg{} })
}

// BackendReadyMsg fires after Connect succeeds and the account is
// wired. By the time Update sees it, acct.Connected() is true.
type BackendReadyMsg struct{}

// BackendErrMsg fires when Connect or WireBackend fails. The account
// stays unwired; cached reads still work.
type BackendErrMsg struct{ Err error }

// connectBackendCmd runs backend.Connect then WireBackend so Update
// always sees a fully-wired account on BackendReadyMsg.
func connectBackendCmd(ctx context.Context, b mail.Backend, acct *cache.Account) tea.Cmd {
	return func() tea.Msg {
		if err := b.Connect(ctx); err != nil {
			return BackendErrMsg{Err: err}
		}
		ct, _ := b.(mail.ChangeTracker)
		if err := acct.WireBackend(b, ct); err != nil {
			return BackendErrMsg{Err: fmt.Errorf("wire backend: %w", err)}
		}
		return BackendReadyMsg{}
	}
}

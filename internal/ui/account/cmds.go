package account

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/textproto"
	"strings"

	// Register non-UTF8 charset decoders. Without this, MIME parts tagged
	// charset="iso-8859-1" (common from Outlook/Exchange) fail to decode
	// and the body is silently dropped.
	_ "github.com/emersion/go-message/charset"

	tea "charm.land/bubbletea/v2"
	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/cache"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/icalendar"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/search"
	"github.com/glw907/poplar/internal/ui/reader"
	"github.com/glw907/poplar/internal/ui/uicore"
)

const (
	initialWindow   = 500 // UIDs requested on a fresh folder open
	loadMoreTrigger = 20  // rows from the bottom that trigger a load-more
)

func loadFoldersCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		if err := c.SyncFolders(context.Background()); err != nil {
			return uicore.ErrorMsg{Op: "list folders", Err: err}
		}
		cls, err := c.ListFolders()
		if err != nil {
			return uicore.ErrorMsg{Op: "list folders", Err: err}
		}
		return foldersLoadedMsg{classified: cls}
	}
}

// queryFolderCmd reads the first window of cached headers and emits a
// FolderLoadedMsg. When sync is true the backend is nudged to converge
// first (sync errors don't fail the load). Empty name returns nil so
// callers can chain without nil-checks.
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
			return uicore.ErrorMsg{Op: op, Err: err}
		}
		return FolderLoadedMsg{Name: name, Msgs: msgs, Total: total}
	}
}

func openFolderCmd(c *cache.Account, name string) tea.Cmd {
	return queryFolderCmd(c, name, true)
}

// refreshFolderCmd queries without sync so the cache's already-applied
// optimistic flip surfaces.
func refreshFolderCmd(c *cache.Account, name string) tea.Cmd {
	return queryFolderCmd(c, name, false)
}

func loadMoreCmd(c *cache.Account, name string, offset int) tea.Cmd {
	return func() tea.Msg {
		msgs, total, err := c.QueryFolder(name, offset, initialWindow)
		if err != nil {
			return uicore.ErrorMsg{Op: "load more", Err: err}
		}
		return folderAppendedMsg{name: name, msgs: msgs, total: total}
	}
}

// queueOpsCmd enqueues one op per uid then refreshes folder. makeArgs
// lets each uid carry its own args. Uid-independent ops return the same
// args every time.
func queueOpsCmd(c *cache.Account, op, folder string, uids []mail.UID, makeArgs func(mail.UID) cache.OpArgs) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, u := range uids {
			if _, err := c.QueueOp(ctx, folder, u, makeArgs(u)); err != nil {
				return uicore.ErrorMsg{Op: op, Err: err}
			}
		}
		msgs, total, err := c.QueryFolder(folder, 0, initialWindow)
		if err != nil {
			return uicore.ErrorMsg{Op: op + " refresh", Err: err}
		}
		return FolderLoadedMsg{Name: folder, Msgs: msgs, Total: total}
	}
}

// enqueueDestroys queues a destroy op per uid against folder, returning
// on the first error.
func enqueueDestroys(ctx context.Context, c *cache.Account, folder string, uids []mail.UID) error {
	for _, u := range uids {
		if _, err := c.QueueOp(ctx, folder, u, cache.DestroyArgs{}); err != nil {
			return err
		}
	}
	return nil
}

// emptyFolderCmd pages every UID in src, queues a destroy op per UID,
// then re-reads. Bypasses the undo bar; the toast suppresses the [u]
// hint when Op == TriageEmpty (ADR-0094).
func emptyFolderCmd(c *cache.Account, displayName, src string) tea.Cmd {
	return func() tea.Msg {
		op := "empty " + strings.ToLower(displayName)
		ctx := context.Background()
		var all []mail.UID
		const page = 1000
		for offset := 0; ; {
			msgs, total, err := c.QueryFolder(src, offset, page)
			if err != nil {
				return uicore.ErrorMsg{Op: op, Err: err}
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
			return uicore.ErrorMsg{Op: op, Err: err}
		}
		return emptyFolderDoneMsg{folder: displayName, source: src, n: len(all)}
	}
}

// destroyCmd queues per-UID destroy ops for the retention sweep. An
// empty uids slice skips the queue and reports a no-op completion.
func destroyCmd(c *cache.Account, folder string, uids []mail.UID) tea.Cmd {
	return func() tea.Msg {
		if len(uids) == 0 {
			return sweepCompletedMsg{folder: folder}
		}
		if err := enqueueDestroys(context.Background(), c, folder, uids); err != nil {
			return uicore.ErrorMsg{Op: "purge expired", Err: err}
		}
		return sweepCompletedMsg{folder: folder, uids: uids}
	}
}

// runSearchCmd dispatches a cross-folder cache.Search.
func runSearchCmd(c *cache.Account, q string) tea.Cmd {
	return func() tea.Msg {
		hits, err := c.Search(context.Background(), search.Parse(q), cache.SearchScope{}, 200)
		if err != nil {
			return uicore.ErrorMsg{Op: "search", Err: err}
		}
		return searchResultsMsg{Hits: hits}
	}
}

// pumpCacheCmd waits for one CacheEvent. Account's Update re-dispatches
// after each event so the pump stays alive across the program lifetime.
func pumpCacheCmd(c *cache.Account) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-c.Events()
		if !ok {
			return nil
		}
		return CacheEventMsg{Event: ev}
	}
}

// walkBody parses buf as an RFC 5322 message and extracts display text,
// list-unsubscribe data, and the first calendar invite part (if present).
// Non-RFC822 input is returned as-is with a zero Unsubscribe and nil Invite.
func walkBody(buf []byte) (text string, unsub content.Unsubscribe, invite *icalendar.Invite) {
	unsub = parseUnsubscribeFromRaw(buf)
	text, _ = content.ExtractPlainText(buf)
	if !content.IsRFC822Frame(buf) {
		return text, unsub, nil
	}
	mr, err := gomail.CreateReader(bytes.NewReader(buf))
	if err != nil {
		return text, unsub, nil
	}
	for {
		p, perr := mr.NextPart()
		if perr != nil {
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
		if ct == "text/calendar" || ct == "application/ics" {
			if inv, ierr := icalendar.ParseInvite(body); ierr == nil {
				invite = &inv
			}
			break
		}
	}
	mr.Close()
	return text, unsub, invite
}

// loadBodyCmd fetches a message body via the cache and parses it into
// blocks. If ctx is cancelled before FetchBody returns, the cmd returns
// nil and the result is dropped (the backend round-trip still completes).
func loadBodyCmd(ctx context.Context, c *cache.Account, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		resultCh := make(chan tea.Msg, 1)
		go func() {
			buf, err := c.FetchBody(uid)
			if err != nil {
				resultCh <- uicore.ErrorMsg{Op: "fetch body", Err: err}
				return
			}
			text, unsub, invite := walkBody(buf)
			resultCh <- reader.BodyLoadedMsg{UID: uid, Blocks: content.ParseBlocks(text), Unsub: unsub, Invite: invite}
		}()
		select {
		case <-ctx.Done():
			return nil
		case msg := <-resultCh:
			return msg
		}
	}
}

// parseUnsubscribeFromRaw reads the RFC 5322 header block from buf
// and returns parsed List-Unsubscribe data. Non-RFC822 input returns
// a zero Unsubscribe.
func parseUnsubscribeFromRaw(buf []byte) content.Unsubscribe {
	if !content.IsRFC822Frame(buf) {
		return content.Unsubscribe{}
	}
	r := textproto.NewReader(bufio.NewReader(bytes.NewReader(buf)))
	h, err := r.ReadMIMEHeader()
	if err != nil {
		return content.Unsubscribe{}
	}
	return content.ParseListUnsubscribe(h)
}

func markReadCmd(c *cache.Account, folder string, uid mail.UID) tea.Cmd {
	return queueOpsCmd(c, "mark read", folder, []mail.UID{uid}, func(_ mail.UID) cache.OpArgs {
		return cache.FlagArgs{Flag: mail.FlagSeen, Set: true}
	})
}

// loadAttachmentsCmd resolves attachment metadata via the cache. Stale-UID
// drops happen at the Model boundary.
func loadAttachmentsCmd(c *cache.Account, uid mail.UID) tea.Cmd {
	return func() tea.Msg {
		items, err := c.Attachments(context.Background(), uid)
		if err != nil {
			return uicore.ErrorMsg{Op: "fetch attachments", Err: err}
		}
		return reader.AttachmentsLoadedMsg{UID: uid, Items: items}
	}
}

// startTriageCmd batches the forward queueOpsCmd with a triage-toast
// emitter so the chrome row appears in the same Update tick as the
// cache flip.
func startTriageCmd(op uicore.TriageOp, dest string, uids []mail.UID, fwd, rev tea.Cmd) tea.Cmd {
	start := func() tea.Msg {
		return TriageStartedMsg{Op: op, N: len(uids), UIDs: uids, Dest: dest, Inverse: rev}
	}
	return tea.Batch(start, fwd)
}

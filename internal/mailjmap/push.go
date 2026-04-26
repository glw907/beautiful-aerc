package mailjmap

import (
	"context"
	"fmt"
	"os"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"git.sr.ht/~rockorager/go-jmap/core/push"

	"github.com/glw907/poplar/internal/mail"
)

const (
	pushBackoffInitial = 1 * time.Second
	pushBackoffMax     = 30 * time.Second
)

// pushLoop runs until ctx is cancelled. On each runEventSourceFunc
// return it backs off exponentially and emits ConnReconnecting.
func (b *Backend) pushLoop(ctx context.Context) {
	defer close(b.pushDone)
	backoff := pushBackoffInitial
	for {
		if ctx.Err() != nil {
			return
		}
		err := b.runEventSourceFunc(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnReconnecting})
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > pushBackoffMax {
				backoff = pushBackoffMax
			}
			continue
		}
		backoff = pushBackoffInitial
	}
}

// runEventSource opens a JMAP EventSource stream and blocks until the
// stream closes or ctx is cancelled. On stream open it emits
// ConnConnected; each StateChange dispatches handleStateChange.
func (b *Backend) runEventSource(ctx context.Context) error {
	b.mu.Lock()
	cli := b.pushClient
	b.mu.Unlock()
	if cli == nil {
		return fmt.Errorf("run event source: no push client")
	}

	// Wire the context cancellation to closing the EventSource.
	// The push package does not accept a context directly; we close
	// the response body from a separate goroutine when ctx is done.
	es := &push.EventSource{
		Client: cli,
		Handler: func(sc *jmap.StateChange) {
			if sc == nil {
				return
			}
			b.mu.Lock()
			accountID := b.session.PrimaryAccounts[jmapmail.URI]
			b.mu.Unlock()

			ts, ok := sc.Changed[accountID]
			if !ok {
				return
			}
			for typ, newState := range ts {
				b.handleStateChange(typ, newState)
			}
		},
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			es.Close()
		case <-done:
		}
	}()

	b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnConnected})
	err := es.Listen()
	close(done)

	if ctx.Err() != nil {
		return nil
	}
	return err
}

// handleStateChange compares newState to the previously known state for
// typ. If unchanged, returns immediately. Otherwise dispatches the
// appropriate change fetch and, on success, updates b.states[typ].
func (b *Backend) handleStateChange(typ string, newState string) {
	b.mu.Lock()
	old := b.states[typ]
	b.mu.Unlock()

	if old == newState {
		return
	}

	var dispatchErr error
	switch typ {
	case "Email":
		dispatchErr = b.dispatchEmailChanges(old)
	case "Mailbox":
		dispatchErr = b.dispatchMailboxChanges(old)
	}

	if dispatchErr != nil {
		fmt.Fprintf(os.Stderr, "mailjmap: handleStateChange %s: %v\n", typ, dispatchErr)
		return
	}

	b.mu.Lock()
	b.states[typ] = newState
	b.mu.Unlock()
}

// dispatchEmailChanges issues Email/changes since prevState and emits
// UpdateNewMail, UpdateFlagsChanged, and UpdateExpunge as appropriate.
func (b *Backend) dispatchEmailChanges(prevState string) error {
	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Changes{
		Account:    accountID,
		SinceState: prevState,
	})
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("email/changes: %w", err)
	}

	var cr *email.ChangesResponse
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*email.ChangesResponse); ok {
			cr = r
			break
		}
	}
	if cr == nil {
		return fmt.Errorf("email/changes: no response")
	}

	if len(cr.Created) > 0 {
		uids := idsToUIDs(cr.Created)
		b.emit(mail.Update{Type: mail.UpdateNewMail, UIDs: uids})
	}
	if len(cr.Updated) > 0 {
		uids := idsToUIDs(cr.Updated)
		b.emit(mail.Update{Type: mail.UpdateFlagsChanged, UIDs: uids})
		// Refresh blobIDs for updated messages in the background.
		go b.refreshBlobIDs(accountID, cr.Updated)
	}
	if len(cr.Destroyed) > 0 {
		uids := idsToUIDs(cr.Destroyed)
		b.emit(mail.Update{Type: mail.UpdateExpunge, UIDs: uids})
	}
	return nil
}

// refreshBlobIDs issues Email/get for ids and updates b.blobIDs cache.
func (b *Backend) refreshBlobIDs(accountID jmap.ID, ids []jmap.ID) {
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Get{
		Account:    accountID,
		IDs:        ids,
		Properties: []string{"id", "blobId"},
	})
	resp, err := b.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mailjmap: refreshBlobIDs: %v\n", err)
		return
	}
	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*email.GetResponse)
		if !ok {
			continue
		}
		b.mu.Lock()
		for _, e := range gr.List {
			b.blobIDs[mail.UID(e.ID)] = string(e.BlobID)
		}
		b.mu.Unlock()
		return
	}
}

// dispatchMailboxChanges issues Mailbox/changes since prevState and
// emits UpdateFolderInfo for each affected mailbox.
func (b *Backend) dispatchMailboxChanges(prevState string) error {
	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&mailbox.Changes{
		Account:    accountID,
		SinceState: prevState,
	})
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("mailbox/changes: %w", err)
	}

	var cr *mailbox.ChangesResponse
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.ChangesResponse); ok {
			cr = r
			break
		}
	}
	if cr == nil {
		return fmt.Errorf("mailbox/changes: no response")
	}

	// Emit UpdateFolderInfo for each changed mailbox ID.
	affected := make([]jmap.ID, 0, len(cr.Created)+len(cr.Updated)+len(cr.Destroyed))
	affected = append(affected, cr.Created...)
	affected = append(affected, cr.Updated...)
	affected = append(affected, cr.Destroyed...)

	b.mu.Lock()
	// Build reverse map: jmap id → folder display name.
	idToName := make(map[string]string, len(b.folders))
	for name, e := range b.folders {
		idToName[e.id] = name
	}
	b.mu.Unlock()

	for _, id := range affected {
		name := idToName[string(id)]
		b.emit(mail.Update{Type: mail.UpdateFolderInfo, Folder: name})
	}
	return nil
}

// emit sends u to the updates channel. If the channel buffer is full
// the update is dropped with a stderr log.
func (b *Backend) emit(u mail.Update) {
	b.mu.Lock()
	ch := b.updates
	b.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- u:
	default:
		fmt.Fprintln(os.Stderr, "mailjmap: dropped update, buffer full")
	}
}

// idsToUIDs converts a []jmap.ID to []mail.UID.
func idsToUIDs(ids []jmap.ID) []mail.UID {
	out := make([]mail.UID, len(ids))
	for i, id := range ids {
		out[i] = mail.UID(id)
	}
	return out
}

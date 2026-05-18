package mailjmap

import (
	"context"
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/core/push"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/glw907/poplar/internal/mail"
)

const (
	pushBackoffInitial = 1 * time.Second
	pushBackoffMax     = 30 * time.Second
)

// expBackoff returns initial * 2^(attempts-1) capped at max.
// attempts <= 1 returns initial.
func expBackoff(attempts int, initial, max time.Duration) time.Duration {
	if attempts <= 1 {
		return initial
	}
	d := initial << (attempts - 1)
	if d > max {
		return max
	}
	return d
}

// pushLoop runs until ctx is cancelled, backing off exponentially and
// emitting ConnReconnecting between runEventSourceFunc returns.
func (b *Backend) pushLoop(ctx context.Context) {
	defer close(b.pushDone)
	attempts := 0
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
			attempts++
			select {
			case <-ctx.Done():
				return
			case <-time.After(expBackoff(attempts, pushBackoffInitial, pushBackoffMax)):
			}
			continue
		}
		attempts = 0
	}
}

// runEventSource opens a JMAP EventSource stream and blocks until the
// stream closes or ctx is cancelled. ConnConnected is emitted on the
// first received event, the earliest signal that the SSE socket is
// genuinely open. The push package does not accept a context, so a
// goroutine closes the response body when ctx is done.
func (b *Backend) runEventSource(ctx context.Context) error {
	b.mu.Lock()
	cli := b.pushClient
	b.mu.Unlock()
	if cli == nil {
		return fmt.Errorf("run event source: no push client")
	}

	b.log.Debug("jmap eventsource connect")
	var connectedOnce sync.Once
	es := &push.EventSource{
		Client: cli,
		Handler: func(sc *jmap.StateChange) {
			connectedOnce.Do(func() {
				b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnConnected})
			})
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
				b.log.Debug("jmap state change", "type", typ)
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

	err := es.Listen()
	close(done)

	if ctx.Err() != nil {
		return nil
	}
	return err
}

// handleStateChange dispatches a change fetch when newState differs
// from b.states[typ] and updates b.states[typ] on success.
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
		b.log.Error("handleStateChange failed", "type", typ, "err", dispatchErr)
		return
	}

	b.mu.Lock()
	b.states[typ] = newState
	b.mu.Unlock()
}

// dispatchEmailChanges runs Email/changes and emits UpdateNewMail,
// UpdateFlagsChanged, or UpdateExpunge as appropriate.
func (b *Backend) dispatchEmailChanges(prevState string) error {
	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Changes{
		Account:    accountID,
		SinceState: prevState,
	})
	resp, err := b.do(req)
	if err != nil {
		return fmt.Errorf("push: email changes: %v", err)
	}

	var cr *email.ChangesResponse
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*email.ChangesResponse); ok {
			cr = r
			break
		}
	}
	if cr == nil {
		return fmt.Errorf("push: email changes returned no response")
	}

	if len(cr.Created) > 0 {
		uids := idsToUIDs(cr.Created)
		b.emit(mail.Update{Type: mail.UpdateNewMail, UIDs: uids})
	}
	if len(cr.Updated) > 0 {
		uids := idsToUIDs(cr.Updated)
		b.emit(mail.Update{Type: mail.UpdateFlagsChanged, UIDs: uids})
		go b.refreshBlobIDs(accountID, cr.Updated)
	}
	if len(cr.Destroyed) > 0 {
		uids := idsToUIDs(cr.Destroyed)
		b.emit(mail.Update{Type: mail.UpdateExpunge, UIDs: uids})
	}
	return nil
}

func (b *Backend) refreshBlobIDs(accountID jmap.ID, ids []jmap.ID) {
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Get{
		Account:    accountID,
		IDs:        ids,
		Properties: []string{"id", "blobId"},
	})
	resp, err := b.do(req)
	if err != nil {
		b.log.Error("refreshBlobIDs failed", "err", err)
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

// dispatchMailboxChanges runs Mailbox/changes and emits
// UpdateFolderInfo for each affected mailbox.
func (b *Backend) dispatchMailboxChanges(prevState string) error {
	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&mailbox.Changes{
		Account:    accountID,
		SinceState: prevState,
	})
	resp, err := b.do(req)
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

	affected := make([]jmap.ID, 0, len(cr.Created)+len(cr.Updated)+len(cr.Destroyed))
	affected = append(affected, cr.Created...)
	affected = append(affected, cr.Updated...)
	affected = append(affected, cr.Destroyed...)

	b.mu.Lock()
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

// emit drops u when the buffer is full so a slow consumer cannot
// stall the push loop.
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
		b.log.Warn("dropped update, channel full")
	}
}

func idsToUIDs(ids []jmap.ID) []mail.UID {
	out := make([]mail.UID, len(ids))
	for i, id := range ids {
		out[i] = mail.UID(id)
	}
	return out
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/jmap"
	"github.com/glw907/poplar/internal/keyring"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/store"
	syncengine "github.com/glw907/poplar/internal/sync"
	"github.com/glw907/poplar/internal/uerr"
)

// fastmailSessionURL is Fastmail's JMAP session discovery endpoint
// (~/.claude/instructions/fastmail-api.md).
const fastmailSessionURL = "https://api.fastmail.com/jmap/session"

// dispatchInterval is how often run's dispatch loop calls
// Dispatcher.DispatchOnce. ADR-0006 names no cadence; five seconds is
// proportionate to SY-2's 30s p95 connection-recovery bound, close
// enough to interactive that a queued intent does not sit long once
// the account is reachable, without polling an idle queue needlessly
// often.
const dispatchInterval = 5 * time.Second

// backendConnector resolves the backend.Backend run drives its sync
// worker and outbox dispatcher against, and key, the identifier
// ensureAccount uses to find or create that account's store row: the
// live JMAP dial in production (connectLiveJMAP), a scripted
// backendtest.Fake in every test that proves the engine wiring
// without a network call.
type backendConnector func(ctx context.Context) (be backend.Backend, key string, err error)

// connectLiveJMAP is run's production backendConnector: it resolves
// the Fastmail bearer token (an account config value, once pass 2
// adds one, or $FASTMAIL_API_TOKEN) and dials the live JMAP session.
// A missing token fails before any network reach, naming both places
// poplar looked, rather than leaving poplar to start with nothing to
// sync.
func connectLiveJMAP(ctx context.Context) (backend.Backend, string, error) {
	token, err := keyring.Token("")
	if err != nil {
		return nil, "", uerr.New("main.connect", nil, uerr.ClassAuth, err)
	}

	creds := jmap.NewStaticCredentials(token)
	session, err := jmap.Dial(ctx, fastmailSessionURL, creds)
	if err != nil {
		return nil, "", err
	}
	return &jmapBackend{session: session, creds: creds}, session.Capabilities().AccountIDs["mail"], nil
}

// jmapBackend adapts jmap.Session to backend.Backend. Session composes
// only a mail source today; Calendar, Contacts, and Push all report
// none, matching a backend that declares no such source.
type jmapBackend struct {
	session *jmap.Session
	creds   backend.Credentials
}

func (b *jmapBackend) Mail() backend.Mail                 { return b.session.Mail() }
func (b *jmapBackend) Calendar() backend.Calendar         { return nil }
func (b *jmapBackend) Contacts() backend.Contacts         { return nil }
func (b *jmapBackend) Push() backend.Push                 { return nil }
func (b *jmapBackend) Capabilities() backend.Capabilities { return b.session.Capabilities() }
func (b *jmapBackend) Credentials() backend.Credentials   { return b.creds }

var _ backend.Backend = (*jmapBackend)(nil)

// ensureAccount finds or creates the store's account row identified by
// key, the backend's own account id rather than a config-supplied one,
// and returns that row's store-local id, the identifier every store
// table scopes its rows by.
func ensureAccount(ctx context.Context, writer *store.Writer, key string) (int64, error) {
	var id int64
	err := writer.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRow(`SELECT id FROM account WHERE slug = ?`, key).Scan(&id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		res, err := tx.Exec(`INSERT INTO account (slug, backend_kind, address) VALUES (?, 'jmap', ?)`, key, key)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, uerr.New("main.account", nil, uerr.ClassStoreLocal, err)
	}
	return id, nil
}

// startEngines starts accountID's sync worker (RunPush's push loop,
// falling back to polling per ADR-0005 for a backend with no push
// transport) and outbox dispatch loop against be, both driven by ctx
// and both stopped by its cancellation. The returned WaitGroup is done
// once both have actually returned; run waits on it before closing the
// writer they still hold.
func startEngines(ctx context.Context, accountID int64, be backend.Backend, writer *store.Writer) *sync.WaitGroup {
	worker := syncengine.NewWorker(accountID, be, writer, syncengine.DefaultConfig())
	dispatcher := outbox.NewDispatcher(accountID, be, writer)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMailbox, backend.ObjectKindMessage})
	}()
	go func() {
		defer wg.Done()
		runDispatchLoop(ctx, dispatcher)
	}()
	return &wg
}

// runDispatchLoop calls DispatchOnce on dispatchInterval's cadence
// until ctx is done.
func runDispatchLoop(ctx context.Context, d *outbox.Dispatcher) {
	ticker := time.NewTicker(dispatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dispatchOnce(ctx, d)
		case <-ctx.Done():
			return
		}
	}
}

// dispatchOnce runs one DispatchOnce pass, surfacing a failure through
// uerr unless it is ctx ending mid-pass: run's own shutdown stopping
// the loop is not a server or store problem to report as one. Each
// delivered or failed intent within a successful pass is already
// logged by the outbox package itself.
func dispatchOnce(ctx context.Context, d *outbox.Dispatcher) {
	if _, err := d.DispatchOnce(ctx, time.Now()); err != nil && ctx.Err() == nil {
		_ = uerr.New("main.dispatch", nil, uerr.ClassStoreLocal, err)
	}
}

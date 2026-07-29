package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
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
// Dispatcher.DispatchOnce between ticks, on top of the immediate call
// runDispatchLoop makes at start (what actually clears a startup
// backlog without delay). ADR-0006 names no dispatch cadence; five
// seconds keeps this loop's own contribution to end-to-end latency a
// minority of outbox.UndoWindow's ten-second hold, so a triage
// action's wait is dominated by the undo window an operator can
// already see, not by this loop's polling granularity.
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
// poplar looked; isFatalConnect is what keeps that failure fatal to
// run while a dial failure past this point (the network being
// unreachable, or a rejected credential) is not.
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

// isFatalConnect reports whether err is connect's one unretried
// failure: a rejected or missing credential, classified
// uerr.ClassAuth. Retrying either with the same token cannot succeed
// on its own, unlike a dial failure (the network being unreachable, a
// timeout, a 5xx), which SY-3's no-network resilience requires run to
// tolerate rather than exit on.
func isFatalConnect(err error) bool {
	ue, ok := errors.AsType[uerr.Error](err)
	return ok && ue.Class == uerr.ClassAuth
}

// dialBackoffMin and dialBackoffMax bound retryConnect's delay: the
// same range RunPush's own reconnect uses for a dropped push
// connection (sync.DefaultConfig), so a network outage at startup
// backs off on the same schedule as one after.
var (
	dialBackoffMin = syncengine.DefaultConfig().BackoffMin
	dialBackoffMax = syncengine.DefaultConfig().BackoffMax
)

// dialBackoffSleep sleeps a jittered exponential delay for attempt (0
// for the first retry), bounded by dialBackoffMin/Max, and reports
// whether it finished; false means ctx ended first.
func dialBackoffSleep(ctx context.Context, attempt int) bool {
	bound := min(dialBackoffMin, dialBackoffMax)
	for range attempt {
		bound = min(bound*2, dialBackoffMax)
	}
	if bound <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(time.Duration(rand.Int64N(int64(bound)))) //nolint:gosec // G404: jitter timing, not a security decision
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// retryConnect calls connect on dialBackoffSleep's jittered schedule
// until it succeeds or ctx ends. ok is false when ctx ended first or
// connect ever reports a fatal (isFatalConnect) failure: a credential
// the operator removed or that the server started rejecting mid-run
// does not become valid by waiting, so retrying it forever would just
// mask the failure run's synchronous first attempt already surfaces
// for the common case.
func retryConnect(ctx context.Context, connect backendConnector) (be backend.Backend, key string, ok bool) {
	for attempt := 0; ; attempt++ {
		be, key, err := connect(ctx)
		if err == nil {
			return be, key, true
		}
		if isFatalConnect(err) {
			return nil, "", false
		}
		if !dialBackoffSleep(ctx, attempt) {
			return nil, "", false
		}
	}
}

// startEnginesRetrying is run's SY-3 fallback for a connect call that
// failed with anything but a fatal credential problem: it retries in
// the background, via retryConnect, so run reports itself running and
// stays usable with no network rather than exiting, and starts the
// engines once a connection succeeds. An ensureAccount failure once
// connected is not retried in turn (a local store problem is not
// SY-3's concern, and the store was already open and working before
// any connect call ran); it is logged through the uerr call
// ensureAccount already makes and the process simply never starts its
// engines.
func startEnginesRetrying(ctx context.Context, writer *store.Writer, connect backendConnector) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Go(func() {
		be, key, ok := retryConnect(ctx, connect)
		if !ok {
			return
		}
		accountID, err := ensureAccount(ctx, writer, key)
		if err != nil {
			return
		}
		startEngines(ctx, accountID, be, writer).Wait()
	})
	return &wg
}

// jmapBackend adapts jmap.Session to backend.Backend. Session composes
// only a mail source today; Calendar, Contacts, and Push all report
// none, matching a backend that declares no such source. Capabilities
// forces PushTransport to PushTransportNone to match Push()'s nil:
// Session's own Capabilities reports PushTransportEventSource
// whenever the live server advertises one, but no Listen
// implementation exists in this package yet (task 6's cutover adds
// it), and a capability that promises what Push() cannot deliver
// would misrender the first consumer that reads it (SY-5's status
// line).
type jmapBackend struct {
	session *jmap.Session
	creds   backend.Credentials
}

func (b *jmapBackend) Mail() backend.Mail         { return b.session.Mail() }
func (b *jmapBackend) Calendar() backend.Calendar { return nil }
func (b *jmapBackend) Contacts() backend.Contacts { return nil }
func (b *jmapBackend) Push() backend.Push         { return nil }
func (b *jmapBackend) Credentials() backend.Credentials {
	return b.creds
}

func (b *jmapBackend) Capabilities() backend.Capabilities {
	caps := b.session.Capabilities()
	caps.PushTransport = backend.PushTransportNone
	return caps
}

// ensureAccount finds or creates the store's account row identified by
// key, the backend's own account id rather than a config-supplied one,
// and returns that row's store-local id, the identifier every store
// table scopes its rows by. address is left empty rather than seeded
// with key: pass 1b has no onboarding flow to collect the account's
// real email, and the account id is not one, so leaving the column
// empty keeps the gap visible instead of putting a plausible-looking
// wrong value in a column every other writer in the tree treats as an
// address.
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
		res, err := tx.Exec(`INSERT INTO account (slug, backend_kind, address) VALUES (?, 'jmap', '')`, key)
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
	wg.Go(func() {
		worker.RunPush(ctx, []backend.ObjectKind{backend.ObjectKindMailbox, backend.ObjectKindMessage})
	})
	wg.Go(func() {
		runDispatchLoop(ctx, dispatcher)
	})
	return &wg
}

// runDispatchLoop calls DispatchOnce once immediately, so a queued
// intent does not wait out a full idle tick before its first attempt,
// then again on dispatchInterval's cadence until ctx is done.
func runDispatchLoop(ctx context.Context, d *outbox.Dispatcher) {
	dispatchOnce(ctx, d)

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

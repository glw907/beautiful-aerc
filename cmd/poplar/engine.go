package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
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
	class, _ := classifyConnect(err)
	return class == uerr.ClassAuth
}

// classifyConnect reports the uerr.Class and root cause a connect
// failure carries, without having logged it. jmap.Dial's own dial-path
// failures come back as a jmap.DialError (a rejected credential, a
// 404, a 5xx, a dead connection): fetchSession classifies without
// calling uerr.New, since retryConnect's own backoff loop, not
// fetchSession, owns the surfacing decision (ADR-0013 revision 2). A
// keyring.Token failure, the one connect error still built through
// uerr.New (connectLiveJMAP fails before any network reach, so there
// is no retry loop to defer to), keeps its own class and cause. One
// jmap.Dial never recognized at all falls back to uerr.ClassConnection,
// the same default sync's own classifyErr uses for an unclassified
// push failure: fetchSession returns a raw error for a JSON decode
// failure, a truncated body, or any status classifyStatusClass does
// not map, and a captive portal's HTTP 200 login page is exactly that
// case. Every connect failure classifies to something, which is what
// keeps retryConnect's own uerr.New call from depending on a lower
// layer having recognized the failure first.
func classifyConnect(err error) (uerr.Class, error) {
	if de, ok := errors.AsType[jmap.DialError](err); ok {
		return de.Class, de.Cause
	}
	if ue, ok := errors.AsType[uerr.Error](err); ok {
		return ue.Class, ue.Cause
	}
	return uerr.ClassConnection, err
}

// dialBackoffMin and dialBackoffMax bound retryConnect's delay: the
// same range RunPush's own reconnect uses for a dropped push
// connection (sync.DefaultConfig), so a network outage at startup
// backs off on the same schedule as one after.
var (
	dialBackoffMin = syncengine.DefaultConfig().BackoffMin
	dialBackoffMax = syncengine.DefaultConfig().BackoffMax
)

// retryConnect calls connect on syncengine.SleepBackoff's jittered
// schedule until it succeeds or ctx ends, in the same shape
// internal/sync's own reconnect uses for a dropped push connection
// (ADR-0013 revision 2): a failure surfaces through uerr.New once, on
// the first failure or a class change, never once per attempt, and a
// later success after a run of failures logs recovery. firstErr is
// the failure run's own synchronous attempt already produced; it
// seeds that first surfacing so an unclassified error still logs
// exactly once rather than depending on connect's caller having
// recognized it, and retryConnect sleeps before its own first retry
// rather than dialing again within microseconds of firstErr's own
// failure. ok is false when ctx ends first or connect ever reports a
// fatal (isFatalConnect) failure: a credential the operator removed
// or that the server started rejecting mid-run does not become valid
// by waiting, and that terminal failure surfaces once on its way out.
func retryConnect(ctx context.Context, connect backendConnector, firstErr error) (be backend.Backend, key string, ok bool) {
	failClass, cause := classifyConnect(firstErr)
	_ = uerr.New("main.connect", nil, failClass, cause)

	for attempt := 0; ; attempt++ {
		if !syncengine.SleepBackoff(ctx, attempt, dialBackoffMin, dialBackoffMax) {
			return nil, "", false
		}
		be, key, err := connect(ctx)
		if err == nil {
			slog.Info("main: connect reconnected", "attempts", attempt+1)
			return be, key, true
		}
		if isFatalConnect(err) {
			// Giving up is its own surfacing event, and the last one
			// this path has: run stays alive with no sync worker and no
			// dispatcher behind it, so an unlogged exit here leaves mail
			// quietly not arriving as the operator's only evidence.
			class, cause := classifyConnect(err)
			_ = uerr.New("main.connect", nil, class, cause)
			return nil, "", false
		}
		if class, cause := classifyConnect(err); class != failClass {
			_ = uerr.New("main.connect", nil, class, cause)
			failClass = class
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
func startEnginesRetrying(ctx context.Context, writer *store.Writer, connect backendConnector, firstErr error) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Go(func() {
		be, key, ok := retryConnect(ctx, connect, firstErr)
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

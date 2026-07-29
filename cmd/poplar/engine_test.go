package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/backend/jmap"
	"github.com/glw907/poplar/internal/keyring"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// TestConnectLiveJMAPReportsBothTokenSources proves a missing token
// fails before any network reach and names both places poplar looked:
// an account config value pass 2 wires up, and $FASTMAIL_API_TOKEN.
func TestConnectLiveJMAPReportsBothTokenSources(t *testing.T) {
	t.Setenv(keyring.EnvFastmailToken, "")

	_, _, err := connectLiveJMAP(context.Background())
	if err == nil {
		t.Fatal("connectLiveJMAP with no token succeeded, want a typed startup error")
	}

	var uerrErr uerr.Error
	if !errors.As(err, &uerrErr) {
		t.Fatalf("err = %v, not a uerr.Error", err)
	}
	if uerrErr.Class != uerr.ClassAuth {
		t.Errorf("Class = %v, want %v", uerrErr.Class, uerr.ClassAuth)
	}
	if !strings.Contains(uerrErr.Cause.Error(), keyring.EnvFastmailToken) {
		t.Errorf("cause = %q, want it naming %s", uerrErr.Cause, keyring.EnvFastmailToken)
	}
}

// TestClassifyConnect asserts classifyConnect's three paths: a
// jmap.DialError (fetchSession's own classified-but-unlogged dial
// failure) yields its own class and cause; an error never classified
// at all (fetchSession's raw JSON-decode failure against a captive
// portal's HTTP 200 HTML body, most notably) defaults to
// ClassConnection; and a uerr.Error (connectLiveJMAP's own
// keyring.Token failure, the one connect error still built through
// uerr.New) yields its own class and its original root cause rather
// than the fixed per-class sentence uerr.Error.Error() returns.
func TestClassifyConnect(t *testing.T) {
	dialCause := errors.New("dial tcp: connection refused")
	dialErr := jmap.DialError{Class: uerr.ClassConnection, Cause: dialCause}
	if class, cause := classifyConnect(dialErr); class != uerr.ClassConnection || cause != dialCause {
		t.Fatalf("classifyConnect(dialErr) = (%v, %v), want (ClassConnection, dialCause)", class, cause)
	}

	plain := errors.New("invalid character '<' looking for beginning of value")
	if class, cause := classifyConnect(plain); class != uerr.ClassConnection || cause != plain {
		t.Fatalf("classifyConnect(plain) = (%v, %v), want (ClassConnection, plain)", class, cause)
	}

	root := errors.New("no token")
	wrapped := uerr.New("test.op", nil, uerr.ClassAuth, root)
	if class, cause := classifyConnect(wrapped); class != uerr.ClassAuth || cause != root {
		t.Fatalf("classifyConnect(wrapped) = (%v, %v), want (ClassAuth, root)", class, cause)
	}
}

// TestIsFatalConnectRecognizesADialErrorAuthClass proves
// isFatalConnect's fatal case still fires for a rejected credential
// that now arrives as a jmap.DialError rather than a uerr.Error, the
// regression this round's MAJOR B fix (fetchSession classifying
// without logging) risked: isFatalConnect previously checked only
// errors.AsType[uerr.Error].
func TestIsFatalConnectRecognizesADialErrorAuthClass(t *testing.T) {
	rejected := jmap.DialError{Class: uerr.ClassAuth, Cause: errors.New("session 401")}
	if !isFatalConnect(rejected) {
		t.Error("isFatalConnect(a ClassAuth DialError) = false, want true")
	}

	refused := jmap.DialError{Class: uerr.ClassConnection, Cause: errors.New("connection refused")}
	if isFatalConnect(refused) {
		t.Error("isFatalConnect(a ClassConnection DialError) = true, want false")
	}
}

// TestRetryConnectSurfacesEachClassOnceAndLogsRecovery pins
// retryConnect's surfacing calls along the path that ends in a
// success, the regression guard TestClassifyConnect cannot provide
// since it only pins the pure classifier, which passes whether or not
// anything ever calls it (TestRetryConnectSurfacesTheCredentialItGivesUpOn
// pins the fourth call, on the path that ends in a fatal). The
// script walks two classes so no one call covers for another: firstErr
// and the identically-classed retry behind it must produce exactly one
// connection line (the seed plus the dedup), the differently-classed
// failure after that must produce a second line (the class-change
// call), and the success after that must log recovery (the slog.Info
// call). A single-class script leaves the class-change call
// unexercised, which is how deleting it stayed green.
func TestRetryConnectSurfacesEachClassOnceAndLogsRecovery(t *testing.T) {
	buf := uerrtest.Capture(t)
	log := uerrtest.CaptureDefault(t)

	var attempts atomic.Int64
	refused := errors.New("dial tcp: connection refused")
	connect := func(context.Context) (backend.Backend, string, error) {
		switch attempts.Add(1) {
		case 1:
			return nil, "", refused
		case 2:
			return nil, "", jmap.DialError{Class: uerr.ClassServer, Cause: errors.New("session: unexpected status 503")}
		default:
			return &backendtest.Fake{}, "test-account", nil
		}
	}

	be, key, ok := retryConnect(context.Background(), connect, refused)
	if !ok {
		t.Fatal("retryConnect did not succeed despite the connector eventually returning nil")
	}
	if be == nil || key != "test-account" {
		t.Fatalf("retryConnect returned (%v, %q), want the fake backend and %q", be, key, "test-account")
	}

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("got %d main.connect log line(s), want exactly 2 (the seed, deduped against the identically-classed retry, then the class change): %v", len(lines), lines)
	}
	wantClasses := []string{"connection", "server"}
	for i, line := range lines {
		if line["msg"] != "main.connect" {
			t.Errorf("line %d msg = %v, want %q", i, line["msg"], "main.connect")
		}
		if line["class"] != wantClasses[i] {
			t.Errorf("line %d class = %v, want %q", i, line["class"], wantClasses[i])
		}
	}

	if !strings.Contains(log.String(), "connect reconnected") {
		t.Errorf("log = %q, want a recovery line once the dial succeeds", log.String())
	}
}

// TestRetryConnectSurfacesTheCredentialItGivesUpOn proves retryConnect
// does not abandon its loop in silence. A credential the server starts
// rejecting partway through an outage ends the retries for good, and
// the process stays up with no sync worker and no dispatcher behind
// it; without a line naming that, the operator's only evidence is mail
// that quietly stops arriving.
func TestRetryConnectSurfacesTheCredentialItGivesUpOn(t *testing.T) {
	buf := uerrtest.Capture(t)

	var attempts atomic.Int64
	refused := errors.New("dial tcp: connection refused")
	connect := func(context.Context) (backend.Backend, string, error) {
		if attempts.Add(1) == 1 {
			return nil, "", refused
		}
		return nil, "", fmt.Errorf("jmap: dial: %w", jmap.DialError{
			Class: uerr.ClassAuth,
			Cause: errors.New("session: unexpected status 401"),
		})
	}

	if _, _, ok := retryConnect(context.Background(), connect, refused); ok {
		t.Fatal("retryConnect reported success despite the server rejecting the credential")
	}

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("got %d main.connect log line(s), want 2 (firstErr's seed, then the rejected credential it gave up on): %v", len(lines), lines)
	}
	if lines[1]["class"] != "auth" {
		t.Errorf("the giving-up line's class = %v, want %q", lines[1]["class"], "auth")
	}
}

// TestStartEnginesAppliesAPushedChange proves startEngines wires the
// sync worker's push loop end to end: a notification on the fake
// backend's push transport lands a mailbox row in the store, with no
// test-driven SyncKind call in between.
func TestStartEnginesAppliesAPushedChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := storetest.Insert(t, w,
			`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "a", "jmap", "a@example.com")

		var be backendtest.Fake
		notify := make(chan backend.Notification)
		be.PushSource = &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
			return notify, nil
		}}
		be.MailSource.ChangesFunc = func(_ context.Context, kind backend.ObjectKind, token string, _ int) (backend.ChangeSet, error) {
			if kind != backend.ObjectKindMailbox || token != "" {
				return backend.ChangeSet{NewToken: token}, nil
			}
			return backend.ChangeSet{
				Created:  []backend.Record{{ID: "mb1", Fields: map[string]any{"name": "Inbox"}}},
				NewToken: "tok-1",
			}, nil
		}

		ctx, cancel := context.WithCancel(context.Background())
		wg := startEngines(ctx, accountID, &be, w)

		notify <- backend.Notification{}
		// The bulk lane's InteractiveQuiet subordination (ADR-0003
		// revision 2) defers this apply behind the account row's own
		// interactive-lane insert above; DefaultConfig's 1s window plus
		// the 200ms coalesce window is what a bare synctest.Wait would
		// otherwise report as durably blocked without ever completing.
		time.Sleep(2 * time.Second)
		synctest.Wait()

		cancel()
		wg.Wait()

		name := storetest.ScanValue[string](t, w,
			`SELECT name FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, "mb1")
		if name != "Inbox" {
			t.Fatalf("mailbox name = %q, want %q", name, "Inbox")
		}
	})
}

// TestStartEnginesDispatchesAnEnqueuedIntent proves startEngines wires
// the outbox dispatch loop end to end: an intent enqueued directly
// against the store the sync worker and dispatcher share reaches the
// fake backend within one dispatchInterval tick, with no test-driven
// DispatchOnce call in between.
func TestStartEnginesDispatchesAnEnqueuedIntent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := storetest.Insert(t, w,
			`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "a", "jmap", "a@example.com")
		mailboxID := storetest.Insert(t, w,
			`INSERT INTO mailbox (account_id, name, server_id) VALUES (?, ?, ?)`, accountID, "Old Name", "mb1")

		var be backendtest.Fake
		renamed := make(chan string, 1)
		be.MailSource.RenameMailboxFunc = func(_ context.Context, id, name string) error {
			renamed <- name
			return nil
		}

		ctx, cancel := context.WithCancel(context.Background())
		wg := startEngines(ctx, accountID, &be, w)

		if _, _, err := outbox.EnqueueRenameMailbox(ctx, w, accountID, mailboxID, "New Name", time.Now()); err != nil {
			t.Fatalf("EnqueueRenameMailbox: %v", err)
		}

		time.Sleep(dispatchInterval)
		synctest.Wait()

		select {
		case name := <-renamed:
			if name != "New Name" {
				t.Fatalf("RenameMailbox name = %q, want %q", name, "New Name")
			}
		default:
			t.Fatal("the dispatch loop never called RenameMailbox within one dispatchInterval")
		}

		cancel()
		wg.Wait()
	})
}

// TestRunDispatchLoopCallsDispatchOnceImmediately proves
// runDispatchLoop dispatches a queued intent on entry rather than
// waiting out its first dispatchInterval tick: it never advances
// synctest's fake clock past t=0, so a pass here is only possible
// through the loop's immediate call, the same call that delayed every
// triage action by up to a full tick when a prior fix round dropped
// it.
func TestRunDispatchLoopCallsDispatchOnceImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := storetest.Insert(t, w,
			`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "a", "jmap", "a@example.com")
		mailboxID := storetest.Insert(t, w,
			`INSERT INTO mailbox (account_id, name, server_id) VALUES (?, ?, ?)`, accountID, "Old Name", "mb1")

		var be backendtest.Fake
		renamed := make(chan string, 1)
		be.MailSource.RenameMailboxFunc = func(_ context.Context, id, name string) error {
			renamed <- name
			return nil
		}

		if _, _, err := outbox.EnqueueRenameMailbox(context.Background(), w, accountID, mailboxID, "New Name", time.Now()); err != nil {
			t.Fatalf("EnqueueRenameMailbox: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		dispatcher := outbox.NewDispatcher(accountID, &be, w)
		go runDispatchLoop(ctx, dispatcher)

		synctest.Wait()

		select {
		case name := <-renamed:
			if name != "New Name" {
				t.Fatalf("RenameMailbox name = %q, want %q", name, "New Name")
			}
		default:
			t.Fatal("runDispatchLoop never called RenameMailbox before its first tick, want an immediate DispatchOnce on entry")
		}

		cancel()
	})
}

package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/keyring"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
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

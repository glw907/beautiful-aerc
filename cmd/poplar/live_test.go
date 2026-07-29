//go:build live

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/keyring"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestRecordingMailCapturesAServerFailure proves recordingMail's
// capture actually distinguishes a failed call from a successful one:
// the property TestLiveRunnerFillsStoreAndDispatchesATriageIntent's
// dispatchAndVerify* helpers depend on to catch exactly the failure
// mode this file's own review found (a dispatched-and-failed intent
// looks identical to a dispatched-and-delivered one from the outbox
// row's disappearance alone). It needs no network reach, so it always
// runs under the live tag rather than only when a token is present.
func TestRecordingMailCapturesAServerFailure(t *testing.T) {
	var fake backendtest.Fake
	fake.MailSource.CreateMailboxFunc = func(context.Context, string, string) (string, error) {
		return "", errors.New("simulated: not found")
	}
	rec := newRecordingMail(&fake.MailSource)

	if _, err := rec.CreateMailbox(context.Background(), "leaf", ""); err == nil {
		t.Fatal("CreateMailbox succeeded despite the scripted failure")
	}

	c, ok := rec.lastCreate()
	if !ok {
		t.Fatal("lastCreate reported nothing recorded")
	}
	if c.err == nil {
		t.Fatal("lastCreate recorded no error despite the scripted failure, want dispatchAndVerifyCreate's failure check to have something to catch")
	}
}

// TestLiveRunnerFillsStoreAndDispatchesATriageIntent runs the
// runner's real composition (connectLiveJMAP, ensureAccount,
// startEngines) against Geoff's live Fastmail account
// (FASTMAIL_API_TOKEN), gated the same way jmap's own live suite is:
// never in CI or make check, skipped where the token was never
// sourced. It proves mailboxes and messages land in the local store
// through the sync worker's poll path, then drives a mailbox
// create/rename/delete round trip through the outbox dispatcher and
// asserts against what the live server actually did, not merely that
// the outbox row disappeared: Dispatcher.report
// (internal/outbox/dispatch.go) deletes a row on an unretriable
// failure exactly as it does on delivery, so a gone row alone cannot
// tell success from failure. A mailbox round trip touches none of
// Geoff's real mail, unlike a message move: fileIntoMailbox
// (internal/outbox/dispatch.go) sets mailbox_ids to exactly one
// destination, so a message that lives in more than one mailbox would
// be permanently dropped from every mailbox but the one named.
func TestLiveRunnerFillsStoreAndDispatchesATriageIntent(t *testing.T) {
	if _, err := keyring.Token(""); err != nil {
		t.Skipf("no fastmail token: %v", err)
	}

	w := storetest.OpenWriter(t, store.DefaultWriterConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	dialed, key, err := connectLiveJMAP(ctx)
	if err != nil {
		t.Fatalf("connectLiveJMAP: %v", err)
	}
	rec := newRecordingMail(dialed.Mail())
	be := &recordingBackend{Backend: dialed, mail: rec}

	accountID, err := ensureAccount(ctx, w, key)
	if err != nil {
		t.Fatalf("ensureAccount: %v", err)
	}

	wg := startEngines(ctx, accountID, be, w)
	stop := func() {
		cancel()
		wg.Wait()
	}
	defer stop()

	// Half one of the criterion: messages and mailboxes from the real
	// account land in the store through the sync worker's poll path.
	waitForAMessage(t, w, accountID, 2*time.Minute)

	// Half two: a triage intent enqueued locally reaches the server,
	// proven against the server's own response, not the outbox row's
	// disappearance.
	name := fmt.Sprintf("poplar-live-test-%d", time.Now().UnixNano())
	serverID := dispatchAndVerifyCreate(t, ctx, w, accountID, rec, name)

	// resolveClaim needs a local mailbox row to resolve rename/delete
	// against (internal/outbox/dispatch.go): this stands in for the
	// optimistic local write ADR-0006 gives to whatever enqueues the
	// intent, which is ordinarily a UI action pass 2 has not built yet.
	mailboxID := storetest.Insert(t, w,
		`INSERT INTO mailbox (account_id, name, server_id) VALUES (?, ?, ?)`, accountID, name, serverID)

	renamed := name + "-renamed"
	dispatchAndVerifyRename(t, ctx, w, accountID, rec, mailboxID, serverID, renamed)
	verifyServerMailboxName(t, ctx, be, serverID, renamed)

	dispatchAndVerifyDelete(t, ctx, w, accountID, rec, mailboxID, serverID)
	verifyServerMailboxGone(t, ctx, be, serverID)
}

// dispatchAndVerifyCreate enqueues a KindCreateMailbox intent for
// name, waits for it to dispatch, and asserts rec actually recorded a
// successful CreateMailbox call for it, returning the server id the
// live server assigned.
func dispatchAndVerifyCreate(t *testing.T, ctx context.Context, w *store.Writer, accountID int64, rec *recordingMail, name string) string {
	t.Helper()

	intentID, _, err := outbox.EnqueueCreateMailbox(ctx, w, accountID, name, 0, 0, time.Now())
	if err != nil {
		t.Fatalf("EnqueueCreateMailbox: %v", err)
	}

	// Best-effort, and registered ahead of waitForDispatch rather than
	// after it: a timeout there ends the test through runtime.Goexit,
	// so a cleanup registered below it is never registered at all, and
	// a timeout following a server-side create is exactly the case that
	// strands a mailbox on the real account. The closure reads
	// allCreates lazily, and it iterates every recorded create rather
	// than the last, since a dispatcher retry across that timeout can
	// leave two.
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, c := range rec.allCreates() {
			if c.serverID != "" {
				_ = rec.Mail.DeleteMailbox(cctx, c.serverID)
			}
		}
	})

	waitForDispatch(t, w, intentID, 30*time.Second)

	c, ok := rec.lastCreate()
	if !ok {
		t.Fatal("the outbox row disappeared but CreateMailbox was never called on the server")
	}
	if c.err != nil {
		t.Fatalf("CreateMailbox(%q) failed on the server: %v", name, c.err)
	}
	if c.name != name || c.serverID == "" {
		t.Fatalf("CreateMailbox recorded name=%q serverID=%q, want name=%q and a non-empty server id", c.name, c.serverID, name)
	}

	return c.serverID
}

// dispatchAndVerifyRename enqueues a KindRenameMailbox intent for
// mailboxID, waits for it to dispatch, and asserts rec actually
// recorded a successful RenameMailbox call against serverID.
func dispatchAndVerifyRename(t *testing.T, ctx context.Context, w *store.Writer, accountID int64, rec *recordingMail, mailboxID int64, serverID, name string) {
	t.Helper()

	intentID, _, err := outbox.EnqueueRenameMailbox(ctx, w, accountID, mailboxID, name, time.Now())
	if err != nil {
		t.Fatalf("EnqueueRenameMailbox: %v", err)
	}
	waitForDispatch(t, w, intentID, 30*time.Second)

	r, ok := rec.lastRename()
	if !ok {
		t.Fatal("the outbox row disappeared but RenameMailbox was never called on the server")
	}
	if r.err != nil {
		t.Fatalf("RenameMailbox(%s, %q) failed on the server: %v", serverID, name, r.err)
	}
	if r.id != serverID || r.name != name {
		t.Fatalf("RenameMailbox recorded id=%s name=%q, want id=%s name=%q", r.id, r.name, serverID, name)
	}
}

// dispatchAndVerifyDelete enqueues a KindDeleteMailbox intent for
// mailboxID, waits for it to dispatch, and asserts rec actually
// recorded a successful DeleteMailbox call against serverID.
func dispatchAndVerifyDelete(t *testing.T, ctx context.Context, w *store.Writer, accountID int64, rec *recordingMail, mailboxID int64, serverID string) {
	t.Helper()

	intentID, _, err := outbox.EnqueueDeleteMailbox(ctx, w, accountID, mailboxID, time.Now())
	if err != nil {
		t.Fatalf("EnqueueDeleteMailbox: %v", err)
	}
	waitForDispatch(t, w, intentID, 30*time.Second)

	d, ok := rec.lastDelete()
	if !ok {
		t.Fatal("the outbox row disappeared but DeleteMailbox was never called on the server")
	}
	if d.err != nil {
		t.Fatalf("DeleteMailbox(%s) failed on the server: %v", serverID, d.err)
	}
	if d.id != serverID {
		t.Fatalf("DeleteMailbox recorded id=%s, want %s", d.id, serverID)
	}
}

// verifyServerMailboxName confirms serverID's mailbox actually carries
// name on the server right now, independent of recordingMail: a fresh
// baseline Mailbox Changes read.
func verifyServerMailboxName(t *testing.T, ctx context.Context, be backend.Backend, serverID, name string) {
	t.Helper()

	rec, ok := findServerMailbox(t, ctx, be, serverID)
	if !ok {
		t.Fatalf("server mailbox %s not found in a fresh Changes(Mailbox) read", serverID)
	}
	if got, _ := rec.Fields["name"].(string); got != name {
		t.Fatalf("server mailbox %s name = %q, want %q", serverID, got, name)
	}
}

// verifyServerMailboxGone confirms serverID no longer appears in a
// fresh baseline Mailbox Changes read.
func verifyServerMailboxGone(t *testing.T, ctx context.Context, be backend.Backend, serverID string) {
	t.Helper()

	if _, ok := findServerMailbox(t, ctx, be, serverID); ok {
		t.Fatalf("server mailbox %s still present after DeleteMailbox reported success", serverID)
	}
}

// findServerMailbox reads a baseline Mailbox Changes page (token ""
// always returns HasMore false per baselineMailboxes) looking for
// serverID.
func findServerMailbox(t *testing.T, ctx context.Context, be backend.Backend, serverID string) (backend.Record, bool) {
	t.Helper()

	cs, err := be.Mail().Changes(ctx, backend.ObjectKindMailbox, "", 0)
	if err != nil {
		t.Fatalf("Changes(Mailbox): %v", err)
	}
	for _, rec := range cs.Created {
		if rec.ID == serverID {
			return rec, true
		}
	}
	for _, rec := range cs.Updated {
		if rec.ID == serverID {
			return rec, true
		}
	}
	return backend.Record{}, false
}

// waitForAMessage polls the store for accountID's first synced
// message, failing the test if none lands within timeout. The message
// row exists only alongside a message_mailbox row naming a synced
// mailbox, so finding one also confirms the mailbox half of the
// criterion.
func waitForAMessage(t *testing.T, w *store.Writer, accountID int64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var messageID int64
		err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
			return tx.QueryRow(
				`SELECT mm.message_id FROM message_mailbox mm
				 JOIN message m ON m.id = mm.message_id
				 WHERE m.account_id = ? LIMIT 1`, accountID).Scan(&messageID)
		})
		if err == nil {
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("query a synced message: %v", err)
		}
		// A wide poll interval on the writer's interactive lane, not a
		// tight one: SyncKind's baseline pages ride the bulk lane, which
		// ADR-0003's InteractiveQuiet subordination defers behind any
		// recent interactive activity, this poll included.
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("no message landed in the store within %s", timeout)
}

// waitForDispatch polls the outbox table for intentID's row until it
// is gone (DispatchOnce delivered or gave up on it) or timeout
// elapses. Gone is not itself proof of delivery (see
// dispatchAndVerify*'s recordingMail assertions for that); it is only
// proof that DispatchOnce finished with this row one way or the other.
func waitForDispatch(t *testing.T, w *store.Writer, intentID int64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var n int
		err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
			return tx.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = ?`, intentID).Scan(&n)
		})
		if err != nil {
			t.Fatalf("check outbox row %d: %v", intentID, err)
		}
		if n == 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("outbox intent %d did not dispatch within %s", intentID, timeout)
}

// recordedCreate is one CreateMailbox call recordingMail captured.
type recordedCreate struct {
	name, parentID, serverID string
	err                      error
}

// recordedRename is one RenameMailbox call recordingMail captured.
type recordedRename struct {
	id, name string
	err      error
}

// recordedDelete is one DeleteMailbox call recordingMail captured.
type recordedDelete struct {
	id  string
	err error
}

// recordingMail wraps a live backend.Mail, capturing the outcome of
// every CreateMailbox, RenameMailbox, and DeleteMailbox call it
// forwards, so a test can assert against what the server actually
// returned rather than inferring success from the outbox row's
// disappearance.
type recordingMail struct {
	backend.Mail

	mu      sync.Mutex
	created []recordedCreate
	renamed []recordedRename
	deleted []recordedDelete
}

func newRecordingMail(m backend.Mail) *recordingMail {
	return &recordingMail{Mail: m}
}

func (r *recordingMail) CreateMailbox(ctx context.Context, name, parentID string) (string, error) {
	id, err := r.Mail.CreateMailbox(ctx, name, parentID)
	r.mu.Lock()
	r.created = append(r.created, recordedCreate{name: name, parentID: parentID, serverID: id, err: err})
	r.mu.Unlock()
	return id, err
}

func (r *recordingMail) RenameMailbox(ctx context.Context, id, name string) error {
	err := r.Mail.RenameMailbox(ctx, id, name)
	r.mu.Lock()
	r.renamed = append(r.renamed, recordedRename{id: id, name: name, err: err})
	r.mu.Unlock()
	return err
}

func (r *recordingMail) DeleteMailbox(ctx context.Context, id string) error {
	err := r.Mail.DeleteMailbox(ctx, id)
	r.mu.Lock()
	r.deleted = append(r.deleted, recordedDelete{id: id, err: err})
	r.mu.Unlock()
	return err
}

func (r *recordingMail) lastCreate() (recordedCreate, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.created) == 0 {
		return recordedCreate{}, false
	}
	return r.created[len(r.created)-1], true
}

// allCreates returns every CreateMailbox call r has recorded so far,
// not only the last: a dispatcher retry can call CreateMailbox twice
// for the same intent (the server-side create landed but the outbox
// row's own state never resolved before a retry fired), leaving two
// live mailboxes to clean up, not one.
func (r *recordingMail) allCreates() []recordedCreate {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedCreate(nil), r.created...)
}

func (r *recordingMail) lastRename() (recordedRename, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.renamed) == 0 {
		return recordedRename{}, false
	}
	return r.renamed[len(r.renamed)-1], true
}

func (r *recordingMail) lastDelete() (recordedDelete, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.deleted) == 0 {
		return recordedDelete{}, false
	}
	return r.deleted[len(r.deleted)-1], true
}

// recordingBackend wraps a live backend.Backend, substituting mail
// for Mail() so every mailbox lifecycle call the dispatcher makes
// through it is recorded.
type recordingBackend struct {
	backend.Backend
	mail *recordingMail
}

func (r *recordingBackend) Mail() backend.Mail { return r.mail }

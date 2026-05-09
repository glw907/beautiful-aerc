package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// jmapFakeBackend wraps fakeBackend with IsJMAP returning true.
type jmapFakeBackend struct{ fakeBackend }

func (j *jmapFakeBackend) IsJMAP() bool { return true }

// openTestAccountWith opens an account backed by the given backend.
func openTestAccountWith(t *testing.T, be mail.Backend) *Account {
	t.Helper()
	ct := &fakeChangeTracker{}
	a, err := Open("Test Account", be, ct, t.TempDir(), Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	return a
}

// outboxKinds returns the kind strings for all outbox rows, ordered by id.
func outboxKinds(t *testing.T, a *Account) []string {
	t.Helper()
	rows, err := a.db.Query(`SELECT kind FROM outbox ORDER BY id`)
	if err != nil {
		t.Fatalf("query outbox: %v", err)
	}
	defer rows.Close()
	var kinds []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan kind: %v", err)
		}
		kinds = append(kinds, k)
	}
	return kinds
}

func TestQueueOutbound_JMAP_OneOp(t *testing.T) {
	be := &jmapFakeBackend{fakeBackend{
		folders: []mail.Folder{{Name: "Sent", Role: "sent"}},
	}}
	a := openTestAccountWith(t, be)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"x@y.com"}}
	if _, err := a.QueueOutbound(context.Background(), "Sent", env, []byte("MIME bytes"), 0, ""); err != nil {
		t.Fatalf("QueueOutbound: %v", err)
	}
	kinds := outboxKinds(t, a)
	if len(kinds) != 1 {
		t.Fatalf("want 1 outbox row, got %d", len(kinds))
	}
	if kinds[0] != string(KindSend) {
		t.Fatalf("want kind=%q, got %q", KindSend, kinds[0])
	}
}

func TestQueueOutbound_IMAP_TwoOps(t *testing.T) {
	be := &fakeBackend{
		folders: []mail.Folder{{Name: "Sent", Role: "sent"}},
	}
	a := openTestAccountWith(t, be)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"x@y.com"}}
	if _, err := a.QueueOutbound(context.Background(), "Sent", env, []byte("MIME bytes"), 0, ""); err != nil {
		t.Fatalf("QueueOutbound: %v", err)
	}
	kinds := outboxKinds(t, a)
	if len(kinds) != 2 {
		t.Fatalf("want 2 outbox rows, got %d", len(kinds))
	}
	if kinds[0] != string(KindSend) || kinds[1] != string(KindAppend) {
		t.Fatalf("want [send, append], got %v", kinds)
	}
}

func TestQueueSendRoundTrip(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	env := mail.Envelope{
		From:  "geoff@907.life",
		Rcpts: []string{"a@example.com", "b@example.com"},
	}
	mime := []byte("From: geoff@907.life\r\n\r\nhello\r\n")

	opID, err := a.QueueSend(context.Background(), "Inbox", env, mime, 0, "")
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}
	if opID == 0 {
		t.Fatal("expected nonzero op id")
	}

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.Kind != string(KindSend) {
		t.Errorf("kind = %q, want %q", row.Kind, KindSend)
	}
	if string(row.Payload) != string(mime) {
		t.Errorf("payload mismatch: got %q want %q", row.Payload, mime)
	}
	args, err := decodeArgs(row.Kind, row.ArgsJSON)
	if err != nil {
		t.Fatalf("decodeArgs: %v", err)
	}
	sa, ok := args.(SendArgs)
	if !ok {
		t.Fatalf("args type = %T, want SendArgs", args)
	}
	if sa.Envelope.From != env.From {
		t.Errorf("From = %q, want %q", sa.Envelope.From, env.From)
	}
	if len(sa.Envelope.Rcpts) != 2 {
		t.Errorf("Rcpts len = %d, want 2", len(sa.Envelope.Rcpts))
	}
}

func TestQueueAppendRoundTrip(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	mime := []byte("From: geoff@907.life\r\nSubject: test\r\n\r\nbody\r\n")
	opID, err := a.QueueAppend(context.Background(), "Inbox", mail.FlagSeen, mime, 0, "")
	if err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}
	if opID == 0 {
		t.Fatal("expected nonzero op id")
	}

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.Kind != string(KindAppend) {
		t.Errorf("kind = %q, want %q", row.Kind, KindAppend)
	}
	if row.FolderName != "Inbox" {
		t.Errorf("folder = %q, want Inbox", row.FolderName)
	}
	if string(row.Payload) != string(mime) {
		t.Errorf("payload mismatch")
	}
	args, err := decodeArgs(row.Kind, row.ArgsJSON)
	if err != nil {
		t.Fatalf("decodeArgs: %v", err)
	}
	aa, ok := args.(AppendArgs)
	if !ok {
		t.Fatalf("args type = %T, want AppendArgs", args)
	}
	if aa.Flag != mail.FlagSeen {
		t.Errorf("Flag = %v, want FlagSeen", aa.Flag)
	}
}

func TestDispatchSend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}}
	mime := []byte("hi\r\n")
	if _, err := a.QueueSend(context.Background(), "Inbox", env, mime, 0, ""); err != nil {
		t.Fatalf("QueueSend: %v", err)
	}

	a.drainOnce(context.Background(), defaultDrainerConfig())

	if len(fb.sends) != 1 {
		t.Fatalf("backend Send calls = %d, want 1", len(fb.sends))
	}
	got := fb.sends[0]
	if got.Env.From != env.From || len(got.Env.Rcpts) != 1 {
		t.Errorf("envelope mismatch: %+v", got.Env)
	}
	if string(got.MIME) != string(mime) {
		t.Errorf("mime mismatch")
	}
}

func TestDispatchAppend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	mime := []byte("body\r\n")
	if _, err := a.QueueAppend(context.Background(), "Inbox", mail.FlagSeen, mime, 0, ""); err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}

	a.drainOnce(context.Background(), defaultDrainerConfig())

	if len(fb.appends) != 1 {
		t.Fatalf("backend Append calls = %d, want 1", len(fb.appends))
	}
	got := fb.appends[0]
	if got.Folder != "Inbox" {
		t.Errorf("folder = %q, want Inbox", got.Folder)
	}
	if got.Flag != mail.FlagSeen {
		t.Errorf("flag = %v, want FlagSeen", got.Flag)
	}
	if string(got.MIME) != string(mime) {
		t.Errorf("mime mismatch")
	}
}

func TestDrainerSkipsRowsBeforeScheduledFor(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	future := time.Now().Add(500 * time.Millisecond).UnixNano()
	if _, err := a.db.Exec(
		`INSERT INTO outbox(folder, kind, args, payload, enqueued_at, status, attempts, scheduled_for) VALUES (NULL, ?, ?, x'', 0, ?, 0, ?)`,
		KindSend, `{}`, OpPending, future,
	); err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}

	row, err := a.nextOutboxRow(time.Now())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("before scheduled_for: got row=%v err=%v, want ErrNoRows", row, err)
	}

	row, err = a.nextOutboxRow(time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("after scheduled_for: %v", err)
	}
	if row == nil {
		t.Fatal("after scheduled_for: expected non-nil row")
	}
}

func TestDrainerDeletesLinkedDraftOnSuccess(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	if err := a.CreateDraft(ctx, "draft-abc", []byte("payload")); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	mustExec(t, a.db,
		`INSERT INTO outbox(folder, kind, args, payload, enqueued_at, status, attempts, draft_id) VALUES (NULL, ?, ?, x'00', 0, ?, 0, ?)`,
		KindSend, `{}`, OpExecuting, "draft-abc")
	var opID int64
	if err := a.db.QueryRow(`SELECT id FROM outbox WHERE draft_id = ?`, "draft-abc").Scan(&opID); err != nil {
		t.Fatalf("scan opID: %v", err)
	}

	if err := a.finishOpDoneAndDeleteDraft(ctx, opID, "draft-abc"); err != nil {
		t.Fatalf("finishOpDoneAndDeleteDraft: %v", err)
	}

	if _, err := a.LoadDraft(ctx, "draft-abc"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("draft not deleted; got err=%v", err)
	}

	var status OpStatus
	if err := a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != OpDone {
		t.Errorf("status = %v, want %v", status, OpDone)
	}
}

func TestDrainerDeletesLinkedDraft_EmptyDraftID(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	mustExec(t, a.db,
		`INSERT INTO outbox(folder, kind, args, payload, enqueued_at, status, attempts) VALUES (NULL, ?, ?, x'00', 0, ?, 0)`,
		KindSend, `{}`, OpExecuting)
	var opID int64
	if err := a.db.QueryRow(`SELECT id FROM outbox ORDER BY id DESC LIMIT 1`).Scan(&opID); err != nil {
		t.Fatalf("scan opID: %v", err)
	}

	if err := a.finishOpDoneAndDeleteDraft(ctx, opID, ""); err != nil {
		t.Fatalf("finishOpDoneAndDeleteDraft: %v", err)
	}

	var status OpStatus
	if err := a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != OpDone {
		t.Errorf("status = %v, want %v", status, OpDone)
	}
}

func TestDrainerDeletesLinkedDraft_DrainPath(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	fb := a.Backend.(*fakeBackend)

	if err := a.CreateDraft(ctx, "draft-xyz", []byte("draft-payload")); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	// Insert a Send row referencing the draft. The Inbox folder exists after SyncFolders.
	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}}
	mime := []byte("hi\r\n")
	opID, err := a.QueueSend(ctx, "Inbox", env, mime, 0, "")
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}
	// Back-fill draft_id on the row (simulates Task 5 wiring).
	mustExec(t, a.db, `UPDATE outbox SET draft_id = ? WHERE id = ?`, "draft-xyz", opID)

	a.drainOnce(ctx, defaultDrainerConfig())

	if len(fb.sends) != 1 {
		t.Fatalf("backend Send calls = %d, want 1", len(fb.sends))
	}

	if _, err := a.LoadDraft(ctx, "draft-xyz"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("draft not deleted after drain; got err=%v", err)
	}

	var status OpStatus
	if err := a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != OpDone {
		t.Errorf("status = %v, want %v", status, OpDone)
	}
}

func TestSendSucceedsAppendConflicts(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}}
	mime := []byte("hello\r\n")

	sendID, err := a.QueueSend(context.Background(), "Inbox", env, mime, 0, "")
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}
	appendID, err := a.QueueAppend(context.Background(), "Inbox", mail.FlagSeen, mime, 0, "")
	if err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}

	// Append fails permanently with auth error → conflict on first attempt.
	fb.appErr = mail.ErrAuth

	a.drainOnce(context.Background(), defaultDrainerConfig())

	// Send must be done. Append must be conflict.
	var sendStatus, appendStatus string
	if err := a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, sendID).Scan(&sendStatus); err != nil {
		t.Fatalf("read send: %v", err)
	}
	if OpStatus(sendStatus) != OpDone {
		t.Errorf("send status = %q, want %q", sendStatus, OpDone)
	}
	if err := a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, appendID).Scan(&appendStatus); err != nil {
		t.Fatalf("read append: %v", err)
	}
	if OpStatus(appendStatus) != OpConflict {
		t.Errorf("append status = %q, want %q", appendStatus, OpConflict)
	}
}

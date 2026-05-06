# Outbox Send/Append Dispatch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `cache.Account` to enqueue and dispatch `KindSend` and `KindAppend` outbox ops through `mail.Backend.Send`/`Append`, with assembled MIME bytes carried in a new `outbox.payload` BLOB column (schema v6).

**Architecture:** SendArgs and AppendArgs become envelope-only typed structs (no payload in JSON). A new `outbox.payload BLOB NULL` column stores assembled MIME bytes verbatim. Two new entry points — `(*Account).QueueSend` and `(*Account).QueueAppend` — share an internal payload-bearing helper. The drainer's `dispatch` switch grows two cases that read `row.Payload` and call the backend. The discard path's `revertOptimisticTx` is relaxed to no-op for Send/Append so conflicted rows can be cleared.

**Tech Stack:** Go 1.26, `modernc.org/sqlite`, existing `internal/cache/` and `internal/mail/` packages.

---

## File map

- Modify: `internal/cache/schema.go` — add `migrateV6`, bump `schemaVersion` to 6.
- Modify: `internal/cache/ops.go` — new `SendArgs`/`AppendArgs` shapes; add `outboxRow.Payload`; new `QueueSend`/`QueueAppend`; relax `revertOptimisticTx`.
- Modify: `internal/cache/drainer.go` — `dispatch` takes `*outboxRow`; new Send/Append cases; `decodeArgs` cases.
- Modify: `internal/cache/cache_test.go` — extend `fakeBackend` to capture Send/Append calls.
- Create: `internal/cache/send_test.go` — round-trip + dispatch + partial-failure tests.
- Modify: `internal/cache/conflicts_test.go` — DiscardOp on conflicted Send/Append.
- Modify: `internal/cache/schema_test.go` (or wherever migrations are tested) — v5→v6 upgrade.

---

### Task 1: Schema v6 migration — add `outbox.payload`

**Files:**
- Modify: `internal/cache/schema.go`

- [ ] **Step 1: Bump schemaVersion**

In `internal/cache/schema.go`:

```go
const schemaVersion = 6
```

- [ ] **Step 2: Append migrateV6 to the migrations slice**

```go
var migrations = []migration{
	migrateV1,
	migrateV2,
	migrateV3,
	migrateV4,
	migrateV5,
	migrateV6, // v5 → v6: outbox.payload BLOB for Send/Append MIME bytes
}
```

- [ ] **Step 3: Implement migrateV6**

After `migrateV5`:

```go
// migrateV6 adds outbox.payload to carry assembled MIME bytes for
// Send/Append ops. NULL for Move/Flag/Destroy.
func migrateV6(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE outbox ADD COLUMN payload BLOB`); err != nil {
		return fmt.Errorf("add outbox.payload: %v", err)
	}
	return nil
}
```

- [ ] **Step 4: Run vet + tests**

Run: `cd /home/glw907/Projects/poplar && go vet ./internal/cache/... && go test ./internal/cache/... -run TestSchema -v`
Expected: PASS (existing schema tests still green; v6 migration runs in fresh DB setup).

- [ ] **Step 5: Commit**

```bash
git add internal/cache/schema.go
git commit -m "Pass 9g: schema v6 — outbox.payload BLOB"
```

---

### Task 2: Replace SendArgs/AppendArgs placeholders with typed shapes

**Files:**
- Modify: `internal/cache/ops.go`

- [ ] **Step 1: Replace placeholder structs**

In `internal/cache/ops.go`, replace the existing block:

```go
type (
	MoveArgs struct{ Dest string }
	FlagArgs struct {
		Flag mail.Flag
		Set  bool
	}
	DestroyArgs struct{}
	SendArgs    struct{}
	AppendArgs  struct{}
)
```

with:

```go
type (
	MoveArgs struct{ Dest string }
	FlagArgs struct {
		Flag mail.Flag
		Set  bool
	}
	DestroyArgs struct{}
	// SendArgs carries the SMTP-level envelope. The MIME body
	// lives in outbox.payload. The destination Sent folder is the
	// outbox row's folder (informational on JMAP, target on IMAP).
	SendArgs struct{ Envelope mail.Envelope }
	// AppendArgs carries the IMAP APPEND flags. The destination
	// folder is the outbox row's folder. The MIME body lives in
	// outbox.payload.
	AppendArgs struct{ Flag mail.Flag }
)
```

- [ ] **Step 2: Update the doc comment on OpArgs**

Replace the comment block above `type OpArgs interface{ opKind() OpKind }` with:

```go
// OpArgs is the sealed sum of queueable operations. Each
// implementation is JSON-serializable for on-disk storage in
// outbox.args. Send/Append carry only metadata; their MIME
// payload lives in outbox.payload.
```

- [ ] **Step 3: Build**

Run: `cd /home/glw907/Projects/poplar && go build ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cache/ops.go
git commit -m "Pass 9g: typed SendArgs/AppendArgs shapes"
```

---

### Task 3: Add `outboxRow.Payload` and update column reads

**Files:**
- Modify: `internal/cache/ops.go`

- [ ] **Step 1: Add Payload field**

In the `outboxRow` struct (around line 109), append:

```go
type outboxRow struct {
	ID         int64
	FolderID   int64
	FolderName string
	MessageID  sql.NullInt64
	ProtocolID sql.NullString
	Kind       string
	ArgsJSON   string
	Attempts   int
	Payload    []byte
}
```

- [ ] **Step 2: Update nextOutboxRow query and Scan**

Replace `nextOutboxRow`'s query and Scan call:

```go
func (a *Account) nextOutboxRow(now time.Time) (*outboxRow, error) {
	const q = `
        SELECT o.id, o.folder, f.name, o.message,
               COALESCE((SELECT m.protocol_id FROM messages m WHERE m.id = o.message), ''),
               o.kind, o.args, o.attempts, o.payload
        FROM outbox o
        JOIN folders f ON f.id = o.folder
        WHERE o.status = ?
           OR (o.status = ? AND (o.next_eligible_at IS NULL OR o.next_eligible_at <= ?))
        ORDER BY o.id LIMIT 1`
	var row outboxRow
	var payload sql.RawBytes
	err := a.db.QueryRow(q, OpPending, OpFailed, now.UnixNano()).Scan(
		&row.ID, &row.FolderID, &row.FolderName, &row.MessageID,
		&row.ProtocolID, &row.Kind, &row.ArgsJSON, &row.Attempts, &payload)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		row.Payload = append([]byte(nil), payload...)
	}
	return &row, nil
}
```

(`sql.RawBytes` + copy is necessary because the underlying buffer is reused across the next Scan.)

- [ ] **Step 3: Build + run existing cache tests**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -count=1`
Expected: PASS (Move/Flag/Destroy paths unchanged; payload reads NULL → nil).

- [ ] **Step 4: Commit**

```bash
git add internal/cache/ops.go
git commit -m "Pass 9g: outboxRow.Payload + payload column read"
```

---

### Task 4: Internal helper `insertFolderOp` for payload-bearing inserts

**Files:**
- Modify: `internal/cache/ops.go`

- [ ] **Step 1: Add the helper**

After `QueueOp` in `internal/cache/ops.go`, add:

```go
// insertFolderOp inserts a folder-scoped outbox row carrying a MIME
// payload. Shared by QueueSend and QueueAppend. No optimistic UI
// flip — these ops have no message-row state to mirror.
func (a *Account) insertFolderOp(ctx context.Context, folder string, args OpArgs, payload []byte) (int64, error) {
	if args == nil {
		return 0, fmt.Errorf("queue: nil args")
	}
	if len(payload) == 0 {
		return 0, fmt.Errorf("queue: empty payload for %s", args.opKind())
	}
	body, err := json.Marshal(args)
	if err != nil {
		return 0, fmt.Errorf("encode args: %w", err)
	}
	folderID, err := a.folderID(folder)
	if err != nil {
		return 0, err
	}
	var opID int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.Exec(`
            INSERT INTO outbox (folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
            VALUES (?, NULL, ?, ?, ?, ?, ?, 0, NULL)`,
			folderID, string(args.opKind()), string(body), payload,
			time.Now().UnixNano(), OpPending)
		if err != nil {
			return fmt.Errorf("insert outbox: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		opID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	a.signalDrainer()
	return opID, nil
}
```

- [ ] **Step 2: Build**

Run: `cd /home/glw907/Projects/poplar && go build ./internal/cache/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/cache/ops.go
git commit -m "Pass 9g: insertFolderOp helper for payload-bearing ops"
```

---

### Task 5: TDD `QueueSend` — failing test, then implement

**Files:**
- Create: `internal/cache/send_test.go`
- Modify: `internal/cache/ops.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cache/send_test.go`:

```go
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func TestQueueSendRoundTrip(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	env := mail.Envelope{
		From:  "geoff@907.life",
		Rcpts: []string{"a@example.com", "b@example.com"},
	}
	mime := []byte("From: geoff@907.life\r\n\r\nhello\r\n")

	opID, err := a.QueueSend(context.Background(), "INBOX", env, mime)
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

```

(Add `"time"` to the import block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run TestQueueSendRoundTrip -v`
Expected: FAIL — `a.QueueSend undefined` AND `decodeArgs` returns error for `KindSend`.

- [ ] **Step 3: Implement QueueSend**

In `internal/cache/ops.go`, after `insertFolderOp`:

```go
// QueueSend enqueues a Send op carrying mime as the assembled
// payload. sentFolder names the canonical Sent folder for the
// account (informational on JMAP, the IMAP target Append will
// reuse on follow-up). Drainer dispatch calls Backend.Send.
func (a *Account) QueueSend(ctx context.Context, sentFolder string, env mail.Envelope, mime []byte) (int64, error) {
	return a.insertFolderOp(ctx, sentFolder, SendArgs{Envelope: env}, mime)
}
```

- [ ] **Step 4: Add SendArgs case to decodeArgs**

In `internal/cache/drainer.go`, extend `decodeArgs`:

```go
func decodeArgs(kind string, payload string) (OpArgs, error) {
	switch OpKind(kind) {
	case KindMove:
		var v MoveArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindFlag:
		var v FlagArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindDestroy:
		return DestroyArgs{}, nil
	case KindSend:
		var v SendArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindAppend:
		var v AppendArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, fmt.Errorf("unknown op kind %q", kind)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run TestQueueSendRoundTrip -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cache/ops.go internal/cache/drainer.go internal/cache/send_test.go
git commit -m "Pass 9g: QueueSend + SendArgs/AppendArgs decode"
```

---

### Task 6: TDD `QueueAppend`

**Files:**
- Modify: `internal/cache/send_test.go`
- Modify: `internal/cache/ops.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/send_test.go`:

```go
func TestQueueAppendRoundTrip(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	mime := []byte("From: geoff@907.life\r\nSubject: test\r\n\r\nbody\r\n")
	opID, err := a.QueueAppend(context.Background(), "INBOX", mail.FlagSeen, mime)
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
	if row.FolderName != "INBOX" {
		t.Errorf("folder = %q, want INBOX", row.FolderName)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run TestQueueAppendRoundTrip -v`
Expected: FAIL — `a.QueueAppend undefined`.

- [ ] **Step 3: Implement QueueAppend**

In `internal/cache/ops.go` after `QueueSend`:

```go
// QueueAppend enqueues an Append op writing mime to folder with
// flag. Drainer dispatch calls Backend.Append.
func (a *Account) QueueAppend(ctx context.Context, folder string, flag mail.Flag, mime []byte) (int64, error) {
	return a.insertFolderOp(ctx, folder, AppendArgs{Flag: flag}, mime)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run TestQueueAppendRoundTrip -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/ops.go internal/cache/send_test.go
git commit -m "Pass 9g: QueueAppend"
```

---

### Task 7: Wire dispatch — drainer routes Send/Append to backend

**Files:**
- Modify: `internal/cache/drainer.go`
- Modify: `internal/cache/cache_test.go` (extend fakeBackend capture)
- Modify: `internal/cache/send_test.go`

- [ ] **Step 1: Extend fakeBackend to capture Send/Append**

In `internal/cache/cache_test.go`, replace the `fakeBackend` struct and Send/Append methods:

```go
type fakeBackend struct {
	moves    []mail.UID
	flags    []mail.UID
	destroys []mail.UID
	sends    []sendCall
	appends  []appendCall
	sendErr  error
	appErr   error
	err      error
	folders  []mail.Folder
	headers  []mail.MessageInfo
	updates  chan mail.Update
}

type sendCall struct {
	Env  mail.Envelope
	MIME []byte
}

type appendCall struct {
	Folder string
	MIME   []byte
	Flag   mail.Flag
}
```

Replace the existing Send/Append stubs:

```go
func (f *fakeBackend) Send(env mail.Envelope, mime []byte) error {
	f.sends = append(f.sends, sendCall{Env: env, MIME: append([]byte(nil), mime...)})
	return f.sendErr
}
func (f *fakeBackend) Append(folder string, mime []byte, flag mail.Flag) error {
	f.appends = append(f.appends, appendCall{Folder: folder, MIME: append([]byte(nil), mime...), Flag: flag})
	return f.appErr
}
```

- [ ] **Step 2: Write failing dispatch test**

Append to `internal/cache/send_test.go`:

```go
func TestDispatchSend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}}
	mime := []byte("hi\r\n")
	if _, err := a.QueueSend(context.Background(), "INBOX", env, mime); err != nil {
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
	if _, err := a.QueueAppend(context.Background(), "INBOX", mail.FlagSeen, mime); err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}

	a.drainOnce(context.Background(), defaultDrainerConfig())

	if len(fb.appends) != 1 {
		t.Fatalf("backend Append calls = %d, want 1", len(fb.appends))
	}
	got := fb.appends[0]
	if got.Folder != "INBOX" {
		t.Errorf("folder = %q, want INBOX", got.Folder)
	}
	if got.Flag != mail.FlagSeen {
		t.Errorf("flag = %v, want FlagSeen", got.Flag)
	}
	if string(got.MIME) != string(mime) {
		t.Errorf("mime mismatch")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run "TestDispatch(Send|Append)" -v`
Expected: FAIL — `dispatch: unknown args cache.SendArgs` (and AppendArgs).

- [ ] **Step 4: Update dispatch signature and add cases**

In `internal/cache/drainer.go`, replace `dispatch` and its caller in `executeOne`:

```go
// dispatch routes one decoded op to the backend.
func (a *Account) dispatch(args OpArgs, row *outboxRow) error {
	uids := []mail.UID{}
	if row.ProtocolID.Valid && row.ProtocolID.String != "" {
		uids = append(uids, mail.UID(row.ProtocolID.String))
	}
	switch v := args.(type) {
	case MoveArgs:
		return a.Backend.Move(uids, v.Dest)
	case FlagArgs:
		return a.Backend.Flag(uids, v.Flag, v.Set)
	case DestroyArgs:
		return a.Backend.Destroy(uids)
	case SendArgs:
		return a.Backend.Send(v.Envelope, row.Payload)
	case AppendArgs:
		return a.Backend.Append(row.FolderName, row.Payload, v.Flag)
	}
	return fmt.Errorf("dispatch: unknown args %T", args)
}
```

In `executeOne`, replace the `uids` block + `dispatch` call:

```go
	dispatchErr := a.dispatch(args, row)
```

(Delete the local `uids` slice construction in `executeOne` — it now lives in `dispatch`.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run "TestDispatch(Send|Append)" -v`
Expected: PASS.

- [ ] **Step 6: Run full cache suite to verify no regression**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -count=1`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cache/drainer.go internal/cache/cache_test.go internal/cache/send_test.go
git commit -m "Pass 9g: dispatch Send/Append through Backend"
```

---

### Task 8: Partial-failure test — Send done, Append fails → conflict

**Files:**
- Modify: `internal/cache/send_test.go`

- [ ] **Step 1: Write the test**

Append to `internal/cache/send_test.go`:

```go
func TestSendSucceedsAppendConflicts(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}}
	mime := []byte("hello\r\n")

	sendID, err := a.QueueSend(context.Background(), "INBOX", env, mime)
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}
	appendID, err := a.QueueAppend(context.Background(), "INBOX", mail.FlagSeen, mime)
	if err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}

	// Append fails permanently with auth error → conflict on first attempt.
	fb.appErr = mail.ErrAuth

	a.drainOnce(context.Background(), defaultDrainerConfig())

	// Send must be done; append must be conflict.
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
```

- [ ] **Step 2: Run test**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run TestSendSucceedsAppendConflicts -v`
Expected: PASS (the existing auth-failure conflict path covers this with no further changes).

- [ ] **Step 3: Commit**

```bash
git add internal/cache/send_test.go
git commit -m "Pass 9g: partial-failure test — Send done, Append conflict"
```

---

### Task 9: Relax `revertOptimisticTx` for Send/Append + DiscardOp regression test

**Files:**
- Modify: `internal/cache/ops.go`
- Modify: `internal/cache/conflicts_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/conflicts_test.go`:

```go
func TestDiscardConflictedSend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	fb.sendErr = mail.ErrAuth
	opID, err := a.QueueSend(context.Background(), "INBOX",
		mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}},
		[]byte("body\r\n"))
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}

	a.drainOnce(context.Background(), defaultDrainerConfig())

	// Sanity: row is in conflict.
	var status string
	if err := a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if OpStatus(status) != OpConflict {
		t.Fatalf("status = %q, want conflict", status)
	}

	if err := a.DiscardOp(context.Background(), opID); err != nil {
		t.Fatalf("DiscardOp: %v", err)
	}

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = ?`, opID).Scan(&n); err != nil {
		t.Fatalf("count row: %v", err)
	}
	if n != 0 {
		t.Errorf("row count = %d, want 0", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run TestDiscardConflictedSend -v`
Expected: FAIL — `discard: revert: revertOptimisticTx: cache.SendArgs not supported`.

- [ ] **Step 3: Relax revertOptimisticTx**

In `internal/cache/ops.go`, replace the `revertOptimisticTx` function:

```go
// revertOptimisticTx mirrors applyOptimisticTx: it undoes the UI flip
// applied at QueueOp time so a discard leaves the cache reflecting
// what the server actually has. Send and Append have no message-row
// state to mirror — the discard simply deletes the outbox row.
func revertOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error {
	switch v := args.(type) {
	case MoveArgs, DestroyArgs:
		_, err := tx.Exec(`UPDATE messages SET ui_hide = 0 WHERE id = ?`, msgID)
		return err
	case FlagArgs:
		bit := uint32(v.Flag)
		stmt := `UPDATE messages SET ui_flags = ui_flags & ~? WHERE id = ?`
		if !v.Set {
			stmt = `UPDATE messages SET ui_flags = ui_flags | ? WHERE id = ?`
		}
		_, err := tx.Exec(stmt, bit, msgID)
		return err
	case SendArgs, AppendArgs:
		return nil
	}
	return fmt.Errorf("revertOptimisticTx: unknown args %T", args)
}
```

Also: `DiscardOp` at line ~255 currently runs `revertOptimisticTx` only when `msgID.Valid`. For Send/Append the message column is NULL, so the call is skipped anyway. But the Send/Append case must still not error if some future caller passes them through with a valid msgID — leaving the explicit no-op case is the conservative choice.

Update the `revertOptimisticTx` doc comment near line 173 (the surrounding paragraph that says "SendArgs and AppendArgs are placeholder op kinds with no optimistic UI state, so they error out") — drop the "so they error out" wording. Replacement text already shown above.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/glw907/Projects/poplar && go test ./internal/cache/... -run TestDiscardConflictedSend -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/ops.go internal/cache/conflicts_test.go
git commit -m "Pass 9g: revertOptimisticTx no-ops Send/Append; DiscardOp test"
```

---

### Task 10: Update cache invariants doc

**Files:**
- Modify: `.claude/rules/cache-invariants.md`

- [ ] **Step 1: Update the schema fact**

Find the bullet starting `Schema is versioned in `schema_version`` and change `v5 adds the attachments table` paragraph to end with:

```
... (UNIQUE
  `(message, part_id)`; index on `message`). v6 adds
  `outbox.payload BLOB NULL` carrying the assembled MIME bytes
  for `KindSend`/`KindAppend` (NULL for Move/Flag/Destroy).
```

- [ ] **Step 2: Update the QueueOp fact**

Find the bullet starting `(*Account).QueueOp(ctx, folder, msgUID, args)` and replace `(reserved `SendArgs`, `AppendArgs`)` with the typed shapes — and add the QueueSend/QueueAppend entry points:

```
- `(*Account).QueueOp(ctx, folder, msgUID, args)` is the single
  forward write entry for Move/Flag/Destroy. `OpArgs` is a sealed
  sum (`MoveArgs`, `FlagArgs`, `DestroyArgs`,
  `SendArgs{Envelope}`, `AppendArgs{Flag}`).
  `(*Account).QueueSend(ctx, sentFolder, env, mime)` and
  `(*Account).QueueAppend(ctx, folder, flag, mime)` are the
  payload-bearing entry points for outbound mail; both insert a
  folder-scoped row with the assembled MIME bytes in
  `outbox.payload` and skip optimistic UI (no message-row state
  to mirror). [...rest of the existing bullet about RetryOp/DiscardOp
  unchanged...]
```

(Preserve the existing trailing text about RetryOp / DiscardOp / ErrNotConflict; this bullet is long — edit in place rather than rewrite.)

- [ ] **Step 3: Update the dispatch fact**

Find the bullet describing the drainer conflict matrix (`Drainer is per-account, single goroutine.`). After the existing text, leave it as-is — the matrix is unchanged. No edit needed.

- [ ] **Step 4: Commit**

```bash
git add .claude/rules/cache-invariants.md
git commit -m "Pass 9g: cache-invariants — payload column + QueueSend/QueueAppend"
```

---

### Task 11: Run `make check` and full test sweep

**Files:** none.

- [ ] **Step 1: Run make check**

Run: `cd /home/glw907/Projects/poplar && make check`
Expected: PASS (fmt-check + vet + voice-check + test all green).

- [ ] **Step 2: If anything fails**

Fix the issue and re-run. Do not skip. Common issues:
- `gofmt -l .` finds an unformatted file → `gofmt -w` it.
- `go vet` complains → fix the call site.
- voice-check trips a tell → rewrite the offending comment per the catalogue.

- [ ] **Step 3: Confirm clean tree**

Run: `git status`
Expected: working tree clean.

(No extra commit — `make check` does not modify tracked files in passing runs.)

---

## Notes for the executor

- The pass-end consolidation ritual (ADR-0158, invariants update for `docs/poplar/invariants.md`, STATUS.md update, plan archival, push, install) is run via the `poplar-pass` skill after this plan's tasks are done. **Do not perform those steps in this plan.**
- The Backend interface assertion in `TestDispatchSend` etc. — `a.Backend.(*fakeBackend)` — is already valid because `openTestAccount` constructs the backend with `&fakeBackend{...}` and assigns it; verify by reading `cache_test.go` if the assertion fails at runtime.
- If `mail.FlagSeen` doesn't exist with that exact name, grep `internal/mail/` for the flag constants and use the matching one (e.g. `mail.FlagSeen`, `mail.SeenFlag`).
- Each task's tests should pass without touching prior tasks. If an earlier test breaks while later tasks land, that's a regression — stop and investigate.

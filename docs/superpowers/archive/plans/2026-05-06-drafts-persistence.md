# Drafts Persistence (Pass 9h.5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist compose drafts in a local SQLite `drafts` table with 1s autosave and 5-minute server push to the JMAP Drafts mailbox, so a draft survives Esc / quit and reappears across devices via Fastmail/JMAP.

**Architecture:** Cache schema v7 adds a `drafts` table; the existing outbox machinery gains a `KindPushDraft` op that calls a new `Backend.PushDraft(folder, mime, prevUID) (newUID, error)` method. JMAP impl batches `Email/import` + `Email/set destroy` in one round-trip; IMAP impl is a stub returning `mail.ErrUnsupported` (filled in Pass 9h.6). The App gates draft persistence on `Backend.IsJMAP()` so IMAP accounts retain today's in-memory-only compose flow.

**Tech Stack:** Go 1.26, modernc.org/sqlite, encoding/gob, bubbletea, rockorager/go-jmap, emersion/go-message.

**Reference docs:**
- Spec: `docs/superpowers/specs/2026-05-06-drafts-persistence-design.md`
- Cache invariants: `.claude/rules/cache-invariants.md`
- Compose foundation ADR: 0156, 0157, 0158, 0159, 0160
- Mandatory skills before any Go file: `go-conventions`. Before any `internal/ui/` file: `elm-conventions`.

**Pre-flight (run once at start of pass):**

- [ ] Read `docs/superpowers/specs/2026-05-06-drafts-persistence-design.md` end-to-end.
- [ ] Confirm `make check` is green on `master` before starting:

```bash
make check
```

Expected: PASS.

---

### Task 1: Schema v7 — drafts table

**Files:**
- Modify: `internal/cache/schema.go`
- Test: `internal/cache/cache_test.go` (extend `TestSchemaMigration`)

- [ ] **Step 1: Write the failing test**

In `internal/cache/cache_test.go`, after the existing `TestSchemaMigration` body, add a new sub-test or extend the existing one to assert v7 and the drafts table shape. Add this test:

```go
func TestSchemaMigration_V7Drafts(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()

	var version int
	if err := a.db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != 7 {
		t.Errorf("schema version = %d, want 7", version)
	}

	rows, err := a.db.Query(`PRAGMA table_info(drafts)`)
	if err != nil {
		t.Fatalf("table_info(drafts): %v", err)
	}
	defer rows.Close()

	want := map[string]string{
		"draft_id":       "TEXT",
		"server_uid":     "TEXT",
		"server_folder":  "TEXT",
		"payload":        "BLOB",
		"dirty":          "INTEGER",
		"created_at":     "INTEGER",
		"updated_at":     "INTEGER",
		"last_pushed_at": "INTEGER",
	}
	got := map[string]string{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = typ
	}
	for col, typ := range want {
		if got[col] != typ {
			t.Errorf("drafts.%s type = %q, want %q", col, got[col], typ)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cache/ -run TestSchemaMigration_V7Drafts -v
```

Expected: FAIL — schema version is 6, no `drafts` table.

- [ ] **Step 3: Implement `migrateV7` and bump `schemaVersion`**

In `internal/cache/schema.go`:

1. Change `const schemaVersion = 6` to `const schemaVersion = 7`.
2. Append `migrateV7` to the `migrations` slice:

```go
var migrations = []migration{
	migrateV1,
	migrateV2,
	migrateV3,
	migrateV4,
	migrateV5,
	migrateV6, // v5 → v6: outbox.payload BLOB for Send/Append MIME bytes
	migrateV7, // v6 → v7: drafts table (local-buffer + server_uid pointer)
}
```

3. Add the migration function at the bottom of the file:

```go
// migrateV7 adds the drafts table. The local row is the high-frequency
// edit buffer for compose; server_uid points at the JMAP/IMAP image
// once a PushDraftOp has succeeded. Drafts with server_uid == NULL are
// local-only (never pushed yet, e.g., offline-created).
func migrateV7(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE drafts (
            draft_id       TEXT PRIMARY KEY,
            server_uid     TEXT,
            server_folder  TEXT,
            payload        BLOB    NOT NULL,
            dirty          INTEGER NOT NULL DEFAULT 1,
            created_at     INTEGER NOT NULL,
            updated_at     INTEGER NOT NULL,
            last_pushed_at INTEGER
        )`,
		`CREATE INDEX drafts_by_server_uid
            ON drafts(server_uid) WHERE server_uid IS NOT NULL`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("add drafts table: %v", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/cache/ -run TestSchemaMigration -v
```

Expected: PASS for both `TestSchemaMigration` (now version=7) and `TestSchemaMigration_V7Drafts`.

- [ ] **Step 5: Update cache-invariants and STATUS notes**

Edit `.claude/rules/cache-invariants.md`. Find the line listing schema versions (currently ending at v6) and append:

```
v7 adds the `drafts` table (`draft_id` PK, `server_uid` nullable
pointer at the server-side image, `server_folder`, `payload` BLOB
holding the gob-encoded `compose.Draft`, `dirty`, `created_at`,
`updated_at`, `last_pushed_at`) plus a partial index
`drafts_by_server_uid` on non-null `server_uid`.
```

- [ ] **Step 6: Commit**

```bash
git add internal/cache/schema.go internal/cache/cache_test.go .claude/rules/cache-invariants.md
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 1: cache schema v7 — drafts table

Adds the local edit-buffer table for compose drafts. server_uid is
nullable; null means "local-only, not yet pushed" (offline draft).
The partial index on non-null server_uid keys the
JMAP-id → draft-id lookup the App uses when restoring a Drafts-row.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: cache.Account drafts API + gob codec

**Files:**
- Create: `internal/cache/drafts.go`
- Test: `internal/cache/drafts_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cache/drafts_test.go`:

```go
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func TestUpsertLoadDraft(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	if err := a.UpsertDraft(ctx, "d1", []byte("payload-v1")); err != nil {
		t.Fatalf("UpsertDraft v1: %v", err)
	}
	got, err := a.LoadDraft(ctx, "d1")
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if string(got) != "payload-v1" {
		t.Errorf("LoadDraft = %q, want %q", got, "payload-v1")
	}

	if err := a.UpsertDraft(ctx, "d1", []byte("payload-v2")); err != nil {
		t.Fatalf("UpsertDraft v2: %v", err)
	}
	got, err = a.LoadDraft(ctx, "d1")
	if err != nil {
		t.Fatalf("LoadDraft after update: %v", err)
	}
	if string(got) != "payload-v2" {
		t.Errorf("LoadDraft after update = %q, want %q", got, "payload-v2")
	}
}

func TestLoadDraft_NotFound(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	_, err := a.LoadDraft(context.Background(), "nope")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("LoadDraft missing = %v, want sql.ErrNoRows", err)
	}
}

func TestListDrafts(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	if err := a.UpsertDraft(ctx, "d1", []byte("a")); err != nil {
		t.Fatalf("UpsertDraft d1: %v", err)
	}
	if err := a.UpsertDraft(ctx, "d2", []byte("b")); err != nil {
		t.Fatalf("UpsertDraft d2: %v", err)
	}
	if err := a.MarkDraftPushed(ctx, "d2", mail.UID("server-id-99"), "Drafts"); err != nil {
		t.Fatalf("MarkDraftPushed: %v", err)
	}

	rows, err := a.ListDrafts(ctx)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListDrafts len = %d, want 2", len(rows))
	}
	byID := map[string]DraftRow{rows[0].DraftID: rows[0], rows[1].DraftID: rows[1]}
	if got := byID["d1"]; got.ServerUID != "" || !got.Dirty {
		t.Errorf("d1 = %+v, want ServerUID empty + Dirty", got)
	}
	if got := byID["d2"]; got.ServerUID != "server-id-99" || got.Dirty {
		t.Errorf("d2 = %+v, want ServerUID=server-id-99 + !Dirty", got)
	}
}

func TestDeleteDraft(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	ctx := context.Background()
	if err := a.UpsertDraft(ctx, "d1", []byte("x")); err != nil {
		t.Fatalf("UpsertDraft: %v", err)
	}
	if err := a.DeleteDraft(ctx, "d1"); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}
	_, err := a.LoadDraft(ctx, "d1")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("LoadDraft after delete = %v, want sql.ErrNoRows", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cache/ -run TestUpsert -v
```

Expected: FAIL with "undefined: a.UpsertDraft" etc.

- [ ] **Step 3: Implement the drafts API**

Create `internal/cache/drafts.go`:

```go
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// DraftRow is the in-memory shape of one drafts row. ServerUID is
// empty until the first successful PushDraft op. LastPushedAt is the
// zero time until then.
type DraftRow struct {
	DraftID      string
	ServerUID    mail.UID
	ServerFolder string
	Payload      []byte
	Dirty        bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastPushedAt time.Time
}

// UpsertDraft writes payload for draftID, marking dirty and bumping
// updated_at. Creates the row on first call. Idempotent — last writer
// wins. Caller is the compose autosave timer.
func (a *Account) UpsertDraft(ctx context.Context, draftID string, payload []byte) error {
	if draftID == "" {
		return fmt.Errorf("UpsertDraft: empty draftID")
	}
	now := time.Now().UnixNano()
	return a.tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
            INSERT INTO drafts (draft_id, payload, dirty, created_at, updated_at)
            VALUES (?, ?, 1, ?, ?)
            ON CONFLICT(draft_id) DO UPDATE SET
                payload    = excluded.payload,
                dirty      = 1,
                updated_at = excluded.updated_at`,
			draftID, payload, now, now)
		return err
	})
}

// LoadDraft returns the payload for draftID, or sql.ErrNoRows.
func (a *Account) LoadDraft(ctx context.Context, draftID string) ([]byte, error) {
	var payload []byte
	err := a.db.QueryRowContext(ctx,
		`SELECT payload FROM drafts WHERE draft_id = ?`, draftID).Scan(&payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// ListDrafts returns every drafts row, oldest-first by created_at.
// The App reads these to project local-only drafts (server_uid == "")
// into the Drafts-folder message-list view.
func (a *Account) ListDrafts(ctx context.Context) ([]DraftRow, error) {
	rows, err := a.db.QueryContext(ctx, `
        SELECT draft_id, COALESCE(server_uid, ''), COALESCE(server_folder, ''),
               payload, dirty, created_at, updated_at,
               COALESCE(last_pushed_at, 0)
        FROM drafts
        ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DraftRow
	for rows.Next() {
		var r DraftRow
		var serverUID, serverFolder string
		var dirtyInt int
		var created, updated, pushed int64
		if err := rows.Scan(&r.DraftID, &serverUID, &serverFolder,
			&r.Payload, &dirtyInt, &created, &updated, &pushed); err != nil {
			return nil, err
		}
		r.ServerUID = mail.UID(serverUID)
		r.ServerFolder = serverFolder
		r.Dirty = dirtyInt != 0
		r.CreatedAt = time.Unix(0, created)
		r.UpdatedAt = time.Unix(0, updated)
		if pushed != 0 {
			r.LastPushedAt = time.Unix(0, pushed)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteDraft removes the local row. Caller is responsible for
// queueing a Destroy op against any server image in the same logical
// flow (handled by the App's Discard / Send paths).
func (a *Account) DeleteDraft(ctx context.Context, draftID string) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM drafts WHERE draft_id = ?`, draftID)
	return err
}

// MarkDraftPushed updates the row after a successful PushDraft. The
// drainer calls this in its post-success path.
func (a *Account) MarkDraftPushed(ctx context.Context, draftID string, serverUID mail.UID, serverFolder string) error {
	_, err := a.db.ExecContext(ctx, `
        UPDATE drafts
        SET server_uid     = ?,
            server_folder  = ?,
            dirty          = 0,
            last_pushed_at = ?
        WHERE draft_id = ?`,
		string(serverUID), serverFolder, time.Now().UnixNano(), draftID)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/cache/ -run "TestUpsertLoadDraft|TestLoadDraft_NotFound|TestListDrafts|TestDeleteDraft" -v
```

Expected: PASS for all four.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/drafts.go internal/cache/drafts_test.go
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 2: cache.Account drafts API

UpsertDraft / LoadDraft / ListDrafts / DeleteDraft / MarkDraftPushed.
Payload is opaque bytes at this layer — the gob encoding of
compose.Draft lives in the compose package. UPSERT via ON CONFLICT
keeps the API single-call from the autosave path.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: PushDraftArgs + KindPushDraft + QueuePushDraft

**Files:**
- Modify: `internal/cache/account.go` (add `KindPushDraft`)
- Modify: `internal/cache/ops.go` (add `PushDraftArgs`, `QueuePushDraft`, extend `revertOptimisticTx`)
- Modify: `internal/cache/drainer.go` (extend `decodeArgs`)
- Test: `internal/cache/drafts_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/drafts_test.go`:

```go
func TestQueuePushDraft(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	mustEnsureFolder(t, a, "Drafts")

	if err := a.UpsertDraft(ctx, "d1", []byte("encoded-draft")); err != nil {
		t.Fatalf("UpsertDraft: %v", err)
	}
	opID, err := a.QueuePushDraft(ctx, "d1", "Drafts", []byte("MIME"), mail.UID(""))
	if err != nil {
		t.Fatalf("QueuePushDraft: %v", err)
	}
	if opID == 0 {
		t.Errorf("QueuePushDraft returned zero opID")
	}

	var kind, argsJSON string
	var payload []byte
	err = a.db.QueryRow(
		`SELECT kind, args, payload FROM outbox WHERE id = ?`, opID).
		Scan(&kind, &argsJSON, &payload)
	if err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	if kind != string(KindPushDraft) {
		t.Errorf("kind = %q, want %q", kind, KindPushDraft)
	}
	if string(payload) != "MIME" {
		t.Errorf("payload = %q, want MIME", payload)
	}
	// Decode through the shared helper to make sure dispatch works.
	args, err := decodeArgsTest(kind, argsJSON)
	if err != nil {
		t.Fatalf("decodeArgs: %v", err)
	}
	pd, ok := args.(PushDraftArgs)
	if !ok {
		t.Fatalf("decoded type = %T, want PushDraftArgs", args)
	}
	if pd.DraftID != "d1" {
		t.Errorf("DraftID = %q, want d1", pd.DraftID)
	}
	if pd.PrevServerUID != "" {
		t.Errorf("PrevServerUID = %q, want empty", pd.PrevServerUID)
	}
}
```

If a `mustEnsureFolder` helper doesn't already exist, create one near the top of the test file:

```go
func mustEnsureFolder(t *testing.T, a *Account, name string) {
	t.Helper()
	_, err := a.db.Exec(
		`INSERT OR IGNORE INTO folders (name, protocol_name) VALUES (?, ?)`,
		name, name)
	if err != nil {
		t.Fatalf("ensure folder %q: %v", name, err)
	}
}

// decodeArgsTest exposes the package-private decodeArgs for tests.
func decodeArgsTest(kind, argsJSON string) (OpArgs, error) {
	return decodeArgs(kind, argsJSON)
}
```

(If `mustEnsureFolder`-equivalent already exists in another `_test.go` file, reuse it.)

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cache/ -run TestQueuePushDraft -v
```

Expected: FAIL — `KindPushDraft` undefined, `PushDraftArgs` undefined, `QueuePushDraft` undefined.

- [ ] **Step 3: Add KindPushDraft to account.go**

In `internal/cache/account.go`, find the `OpKind` consts block:

```go
const (
	KindMove    OpKind = "move"
	KindFlag    OpKind = "flag"
	KindDestroy OpKind = "destroy"
	KindSend    OpKind = "send"
	KindAppend  OpKind = "append"
)
```

Add `KindPushDraft`:

```go
const (
	KindMove      OpKind = "move"
	KindFlag      OpKind = "flag"
	KindDestroy   OpKind = "destroy"
	KindSend      OpKind = "send"
	KindAppend    OpKind = "append"
	KindPushDraft OpKind = "push-draft"
)
```

- [ ] **Step 4: Add PushDraftArgs and QueuePushDraft to ops.go**

In `internal/cache/ops.go`, extend the `OpArgs` block:

```go
type (
	MoveArgs    struct{ Dest string }
	FlagArgs    struct {
		Flag mail.Flag
		Set  bool
	}
	DestroyArgs struct{}
	SendArgs    struct{ Envelope mail.Envelope }
	AppendArgs  struct{ Flag mail.Flag }
	// PushDraftArgs queues a server-side draft replace. The current
	// payload lives in outbox.payload (assembled MIME). On success
	// the drainer updates the drafts row's server_uid and (if
	// PrevServerUID was non-empty) the backend has destroyed the
	// prior server image in the same call.
	PushDraftArgs struct {
		DraftID       string
		PrevServerUID mail.UID
	}
)

func (MoveArgs) opKind() OpKind      { return KindMove }
func (FlagArgs) opKind() OpKind      { return KindFlag }
func (DestroyArgs) opKind() OpKind   { return KindDestroy }
func (SendArgs) opKind() OpKind      { return KindSend }
func (AppendArgs) opKind() OpKind    { return KindAppend }
func (PushDraftArgs) opKind() OpKind { return KindPushDraft }
```

Add the queue entry-point near `QueueAppend`:

```go
// QueuePushDraft enqueues a PushDraft op carrying mime as the
// assembled payload. prevUID is empty on first push for a draft
// and the previous server_uid on subsequent pushes; the backend
// destroys the prior image as part of the same op when prevUID
// is non-empty.
func (a *Account) QueuePushDraft(ctx context.Context, draftID, folder string, mime []byte, prevUID mail.UID) (int64, error) {
	return a.insertFolderOp(ctx, folder,
		PushDraftArgs{DraftID: draftID, PrevServerUID: prevUID}, mime)
}
```

Extend `revertOptimisticTx` to no-op on PushDraftArgs (mirroring Send/Append):

```go
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
	case SendArgs, AppendArgs, PushDraftArgs:
		return nil
	}
	return fmt.Errorf("revertOptimisticTx: unknown args %T", args)
}
```

- [ ] **Step 5: Extend decodeArgs in drainer.go**

In `internal/cache/drainer.go`, in the `decodeArgs` switch, add a `KindPushDraft` arm before the default:

```go
case KindAppend:
	var v AppendArgs
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		return nil, err
	}
	return v, nil
case KindPushDraft:
	var v PushDraftArgs
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		return nil, err
	}
	return v, nil
```

- [ ] **Step 6: Run test to verify it passes**

```bash
go test ./internal/cache/ -run TestQueuePushDraft -v
go test ./internal/cache/... -run "." -v
```

Expected: TestQueuePushDraft PASSES; all other cache tests still PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cache/account.go internal/cache/ops.go internal/cache/drainer.go internal/cache/drafts_test.go
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 3: PushDraftArgs op + QueuePushDraft

Joins the OpArgs sealed sum. Drainer dispatch arm comes in task 5
once the backend method exists; this task wires the queue path,
the JSON codec, and revertOptimisticTx (no-op like Send/Append).

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Backend.PushDraft on the interface + IMAP stub

**Files:**
- Modify: `internal/mail/backend.go` (add `PushDraft` to `Backend`, add `ErrUnsupported`)
- Modify: `internal/mailimap/imap.go` (stub `PushDraft` returning `mail.ErrUnsupported`)
- Test: `internal/mailimap/imap_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/mailimap/imap_test.go`, add:

```go
func TestPushDraft_IMAP_Unsupported(t *testing.T) {
	b := newTestBackend(t) // existing helper
	_, err := b.PushDraft("Drafts", []byte("MIME"), mail.UID(""))
	if !errors.Is(err, mail.ErrUnsupported) {
		t.Errorf("PushDraft err = %v, want mail.ErrUnsupported", err)
	}
}
```

If `newTestBackend` helper doesn't exist exactly under that name, locate the equivalent test helper for IMAP (`grep -n "func.*Backend.*test" internal/mailimap/`) and use that name. Add `import "errors"` and `"github.com/glw907/poplar/internal/mail"` if missing.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/mailimap/ -run TestPushDraft_IMAP_Unsupported -v
```

Expected: FAIL — `mail.ErrUnsupported` undefined and/or `b.PushDraft` undefined.

- [ ] **Step 3: Add ErrUnsupported and PushDraft to mail.Backend**

In `internal/mail/backend.go`, near the existing sentinels (search for `ErrAuth` / `ErrNotFound` definitions):

```go
// ErrUnsupported is returned by backend methods whose feature is
// not implemented for that protocol in this poplar release. The
// drainer does NOT route this through the conflict matrix as
// auth/transient — callers (the App) are expected to gate on
// capability before queueing such ops.
var ErrUnsupported = errors.New("mail: operation unsupported by backend")
```

(Add `import "errors"` if not already present.)

In the `Backend` interface, after `Append`, add:

```go
// PushDraft writes a new server image of a draft and (if prevUID
// is non-empty) destroys the prior image in the same operation.
// Atomicity is best-effort by backend:
//
// JMAP: one Email/import + Email/set destroy round-trip.
//       Succeeds-or-fails as a unit at the network layer.
// IMAP: APPEND new + UID STORE \Deleted on prevUID + UID EXPUNGE.
//       Not atomic; orphans possible on partial failure.
//
// Returns the newly assigned server UID.
PushDraft(folder string, mime []byte, prevUID UID) (UID, error)
```

- [ ] **Step 4: Stub IMAP impl**

In `internal/mailimap/imap.go`, add (near the existing `Append` method):

```go
// PushDraft is unsupported on IMAP in this release. Pass 9h.6 will
// implement APPEND \Draft + STORE \Deleted + UID EXPUNGE. The App
// gates on Backend.IsJMAP() so this stub never fires for IMAP
// accounts in normal operation; if it does, the outbox routes the
// error through its conflict matrix as a transient.
func (b *Backend) PushDraft(folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
	return "", mail.ErrUnsupported
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/mailimap/ -run TestPushDraft_IMAP_Unsupported -v
go build ./...
```

Expected: TestPushDraft_IMAP_Unsupported PASSES; the build is green (the JMAP backend will fail to compile until task 5, so this step's `go build` may red-flag a missing JMAP method — that's expected; do not commit yet if so. If the build is red on JMAP only, proceed to task 5 and commit at the end of task 5 for both 4 and 5 together).

If build is fully green (the JMAP backend is structurally compatible), commit task 4 separately:

- [ ] **Step 6: Commit (only if build is fully green)**

```bash
git add internal/mail/backend.go internal/mailimap/imap.go internal/mailimap/imap_test.go
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 4: mail.Backend.PushDraft + ErrUnsupported

Adds the protocol-agnostic draft push primitive. IMAP stub
returns ErrUnsupported pending Pass 9h.6. The App gates on
IsJMAP() so the stub is never reached in production paths.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

If the build was red on JMAP, skip this commit and bundle with task 5.

---

### Task 5: JMAP PushDraft impl + drainer dispatch

**Files:**
- Modify: `internal/mailjmap/jmap.go` (implement `PushDraft`)
- Modify: `internal/cache/drainer.go` (add `KindPushDraft` arm to `dispatch`)
- Test: `internal/mailjmap/jmap_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/mailjmap/jmap_test.go`, add a test using the existing fake-client pattern (search for an existing test like `TestSend_JMAP_*` to model on):

```go
func TestPushDraft_JMAP_FirstPush(t *testing.T) {
	fake := newFakeJMAPClient(t)
	fake.expectFolders(map[string]string{"Drafts": "drafts-mb-1"})
	fake.onImport("k1", "new-email-id-1") // returns Email/import Created["k1"].ID
	fake.expectNoDestroy()

	b := newJMAPBackend(t, fake)
	if err := b.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	uid, err := b.PushDraft("Drafts", []byte("MIME"), mail.UID(""))
	if err != nil {
		t.Fatalf("PushDraft: %v", err)
	}
	if uid != "new-email-id-1" {
		t.Errorf("PushDraft uid = %q, want new-email-id-1", uid)
	}
}

func TestPushDraft_JMAP_ReplacesPrev(t *testing.T) {
	fake := newFakeJMAPClient(t)
	fake.expectFolders(map[string]string{"Drafts": "drafts-mb-1"})
	fake.onImport("k1", "new-email-id-2")
	fake.expectDestroy([]string{"old-email-id-1"})

	b := newJMAPBackend(t, fake)
	if err := b.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	uid, err := b.PushDraft("Drafts", []byte("MIME"), mail.UID("old-email-id-1"))
	if err != nil {
		t.Fatalf("PushDraft: %v", err)
	}
	if uid != "new-email-id-2" {
		t.Errorf("PushDraft uid = %q, want new-email-id-2", uid)
	}
}
```

The exact test-helper names depend on the existing JMAP test scaffold. **Before writing**, run `rg -n "func.*FakeClient|func newJMAP" internal/mailjmap/jmap_test.go | head` and use the existing names. The two pieces of behavior these tests verify are:
1. `Email/import` runs against the Drafts mailbox and the returned ID is propagated.
2. When `prevUID != ""`, an `Email/set destroy` is batched in the same request.

Adapt the test code to call helpers that already exist in this codebase. If the existing fake doesn't expose `onImport`/`expectDestroy`, extend it minimally (one-line additions) — do not rewrite the test scaffold.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/mailjmap/ -run TestPushDraft_JMAP -v
```

Expected: FAIL — `PushDraft` undefined on JMAP backend.

- [ ] **Step 3: Implement JMAP PushDraft**

In `internal/mailjmap/jmap.go`, add (modeled on `Append` and `Destroy`):

```go
// PushDraft uploads mime, imports it into the Drafts mailbox with
// the $draft keyword, and (if prevUID is non-empty) destroys the
// prior image in the same JMAP request. Returns the new server
// id as mail.UID.
func (b *Backend) PushDraft(folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
	b.mu.Lock()
	upload := b.uploadBlob
	entry, ok := b.folders[folder]
	accountID := b.accountIDLocked()
	b.mu.Unlock()
	if upload == nil {
		return "", errors.New("jmap: not connected")
	}
	if !ok {
		return "", fmt.Errorf("push-draft: unknown folder %q", folder)
	}

	blobID, err := upload(mime)
	if err != nil {
		return "", fmt.Errorf("push-draft: upload: %w", classifyErr(err))
	}
	now := time.Now().UTC()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	importCallID := req.Invoke(&email.Import{
		Account: accountID,
		Emails: map[string]*email.EmailImport{
			"k1": {
				BlobID:     jmap.ID(blobID),
				MailboxIDs: map[jmap.ID]bool{jmap.ID(entry.id): true},
				Keywords:   map[string]bool{"$draft": true},
				ReceivedAt: &now,
			},
		},
	})
	var destroyCallID string
	if prevUID != "" {
		destroyCallID = req.Invoke(&email.Set{
			Account: accountID,
			Destroy: []jmap.ID{jmap.ID(prevUID)},
		})
	}

	resp, err := b.do(req)
	if err != nil {
		return "", fmt.Errorf("push-draft: %w", classifyErr(err))
	}
	newID, err := importedID(resp, importCallID)
	if err != nil {
		return "", fmt.Errorf("push-draft: import: %w", err)
	}
	if destroyCallID != "" {
		// notFound on destroy is benign (the prior image was already
		// gone, e.g. another client deleted it). The new image is
		// canonical; surface no error.
		if err := checkEmailSetDestroyed(resp, destroyCallID); err != nil {
			// Last-write-wins: the create succeeded, so we still
			// committed the user's intent. Log only.
			fmt.Fprintf(stderrLog(), "jmap: push-draft destroy of %s: %v (continuing)\n", prevUID, err)
		}
	}
	return mail.UID(newID), nil
}

// importedID extracts the Created["k1"].ID from an Email/import
// response. Mirrors checkImportCreated but returns the id.
func importedID(resp *jmap.Response, callID string) (string, error) {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		ir, ok := inv.Args.(*email.ImportResponse)
		if !ok {
			continue
		}
		if se, bad := ir.NotCreated["k1"]; bad {
			return "", fmt.Errorf("import rejected: %s", se.Type)
		}
		entry, ok := ir.Created["k1"]
		if !ok {
			return "", errors.New("import: no Created entry")
		}
		return string(entry.ID), nil
	}
	return "", errors.New("import: no response")
}
```

If `stderrLog()` is not in scope from `internal/mailjmap`, replace with `os.Stderr` and add `"os"` to imports.

If a `mailjmap` `stderrLog` helper exists (it does in cache), match the existing pattern.

- [ ] **Step 4: Add drainer dispatch arm**

In `internal/cache/drainer.go`, in the `dispatch` switch, add a `PushDraftArgs` arm:

```go
case PushDraftArgs:
	newUID, err := a.Backend.PushDraft(row.FolderName, row.Payload, v.PrevServerUID)
	if err != nil {
		return err
	}
	return a.MarkDraftPushed(context.Background(), v.DraftID, newUID, row.FolderName)
```

(Place it after the `AppendArgs` case, before the default.)

Note: the dispatcher doesn't currently take a `ctx`; the `MarkDraftPushed` call uses `context.Background()` because the row is already committed and the only failure mode here is a SQLite transient. If the existing dispatch path threads ctx, use it; otherwise `context.Background()` is consistent with how `executeOne` calls `finalizeSuccess(ctx, ...)` — pass the same ctx through if the signature allows.

If `dispatch` does not take a ctx but the surrounding `executeOne` does, change the dispatch signature to `dispatch(ctx context.Context, args OpArgs, row *outboxRow) error` and update the one caller.

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/mailjmap/ -run TestPushDraft_JMAP -v
go test ./internal/cache/... -run "." -v
go build ./...
```

Expected: PushDraft tests PASS; all cache tests PASS; build is fully green.

- [ ] **Step 6: Commit**

```bash
git add internal/mailjmap/jmap.go internal/mailjmap/jmap_test.go internal/cache/drainer.go
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 5: JMAP PushDraft impl + drainer dispatch

Email/import + Email/set destroy batched in one request. notFound
on the destroy is benign (prior image already gone). Drainer's
post-success path writes server_uid back to the drafts row via
MarkDraftPushed.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

If task 4's commit was deferred, include those files in this commit too.

---

### Task 6: ParseDraftMIME — symmetric to AssembleMIME

**Files:**
- Create: `internal/compose/parse.go`
- Test: `internal/compose/parse_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/compose/parse_test.go`:

```go
// SPDX-License-Identifier: MIT

package compose

import (
	"testing"
	"time"

	gomail "github.com/emersion/go-message/mail"
)

func TestRoundTrip_AssembleParse(t *testing.T) {
	d := Draft{
		From:    gomail.Address{Name: "Geoff", Address: "geoff@907.life"},
		To:      []gomail.Address{{Address: "alice@example.com"}},
		Cc:      []gomail.Address{{Address: "bob@example.com"}},
		Subject: "test draft",
		Body:    "hello\n\n> quoted\n",
	}
	mime, err := AssembleMIME(d, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("AssembleMIME: %v", err)
	}
	got, err := ParseDraftMIME(mime)
	if err != nil {
		t.Fatalf("ParseDraftMIME: %v", err)
	}
	if got.Subject != d.Subject {
		t.Errorf("Subject = %q, want %q", got.Subject, d.Subject)
	}
	if got.From.Address != d.From.Address {
		t.Errorf("From = %q, want %q", got.From.Address, d.From.Address)
	}
	if len(got.To) != 1 || got.To[0].Address != "alice@example.com" {
		t.Errorf("To = %+v, want alice@example.com", got.To)
	}
	if len(got.Cc) != 1 || got.Cc[0].Address != "bob@example.com" {
		t.Errorf("Cc = %+v, want bob@example.com", got.Cc)
	}
	if got.Body != d.Body {
		t.Errorf("Body = %q, want %q", got.Body, d.Body)
	}
}

func TestParseDraftMIME_HeadersOnly(t *testing.T) {
	mime := []byte("From: a@x\r\nTo: b@y\r\nSubject: hi\r\n\r\nBody text\r\n")
	got, err := ParseDraftMIME(mime)
	if err != nil {
		t.Fatalf("ParseDraftMIME: %v", err)
	}
	if got.Subject != "hi" || got.Body != "Body text\r\n" {
		t.Errorf("got = %+v, want subject=hi body=Body text", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/compose/ -run TestRoundTrip_AssembleParse -v
```

Expected: FAIL — `ParseDraftMIME` undefined.

- [ ] **Step 3: Implement ParseDraftMIME**

Create `internal/compose/parse.go`:

```go
// SPDX-License-Identifier: MIT

package compose

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	gomessage "github.com/emersion/go-message"
	gomail "github.com/emersion/go-message/mail"
)

// ParseDraftMIME reverses AssembleMIME for the fields a Draft carries.
// It walks the message tree to extract the text/plain part as the
// body; HTML siblings are dropped (we re-assemble them from markdown
// on the next push). Attachments come back as filenames only — the
// outbox payload carries the full bytes, so a draft round-trip via
// the local store loses nothing the user can see in compose.
func ParseDraftMIME(raw []byte) (Draft, error) {
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		// Fall back to a plain go-message read for headers-only or
		// non-multipart drafts (legacy / hand-typed content).
		return parsePlain(raw)
	}
	defer mr.Close()

	var d Draft
	hdr := mr.Header
	if from, _ := hdr.AddressList("From"); len(from) > 0 {
		d.From = *from[0]
	}
	if to, _ := hdr.AddressList("To"); len(to) > 0 {
		for _, a := range to {
			d.To = append(d.To, *a)
		}
	}
	if cc, _ := hdr.AddressList("Cc"); len(cc) > 0 {
		for _, a := range cc {
			d.Cc = append(d.Cc, *a)
		}
	}
	if bcc, _ := hdr.AddressList("Bcc"); len(bcc) > 0 {
		for _, a := range bcc {
			d.Bcc = append(d.Bcc, *a)
		}
	}
	d.Subject, _ = hdr.Subject()
	d.InReplyTo, _ = hdr.Text("In-Reply-To")
	if refs, err := hdr.MsgIDList("References"); err == nil {
		d.References = refs
	}

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return d, fmt.Errorf("read part: %w", err)
		}
		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := h.ContentType()
			if d.Body == "" && strings.EqualFold(ct, "text/plain") {
				body, err := io.ReadAll(p.Body)
				if err != nil {
					return d, fmt.Errorf("read body: %w", err)
				}
				d.Body = string(body)
			}
		case *gomail.AttachmentHeader:
			fn, _ := h.Filename()
			if fn != "" {
				d.Attachments = append(d.Attachments, fn)
			}
		}
	}
	return d, nil
}

func parsePlain(raw []byte) (Draft, error) {
	m, err := gomessage.Read(bytes.NewReader(raw))
	if err != nil {
		return Draft{}, fmt.Errorf("parse mime: %w", err)
	}
	var d Draft
	hdr := gomail.Header{Header: m.Header}
	if from, _ := hdr.AddressList("From"); len(from) > 0 {
		d.From = *from[0]
	}
	if to, _ := hdr.AddressList("To"); len(to) > 0 {
		for _, a := range to {
			d.To = append(d.To, *a)
		}
	}
	d.Subject, _ = hdr.Subject()
	body, err := io.ReadAll(m.Body)
	if err != nil {
		return d, fmt.Errorf("read body: %w", err)
	}
	d.Body = string(body)
	return d, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/compose/ -run "TestRoundTrip_AssembleParse|TestParseDraftMIME_HeadersOnly" -v
```

Expected: PASS for both.

- [ ] **Step 5: Commit**

```bash
git add internal/compose/parse.go internal/compose/parse_test.go
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 6: ParseDraftMIME, symmetric to AssembleMIME

Lifts the text/plain part as the body; drops HTML siblings (we
re-assemble them from markdown on next push). Attachments come
back as filenames only — the outbox payload carries full bytes
so a draft round-trip via the local store loses nothing visible
in compose.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: compose.Model lifecycle — draftID, autosave, server-push timer

**Files:**
- Modify: `internal/ui/compose/model.go`
- Modify: `internal/ui/compose/msgs.go`
- Create: `internal/compose/codec.go` (gob encode/decode of `Draft`)
- Test: `internal/ui/compose/model_test.go`

**Pre-step:** Invoke `elm-conventions` skill before editing any `internal/ui/` file. The lifecycle changes here must keep state in the model, mutations only in `Update`, I/O only in `tea.Cmd`.

- [ ] **Step 1: Write the failing test (codec)**

Create `internal/compose/codec_test.go`:

```go
// SPDX-License-Identifier: MIT

package compose

import (
	"reflect"
	"testing"

	gomail "github.com/emersion/go-message/mail"
)

func TestDraftGobRoundTrip(t *testing.T) {
	in := Draft{
		From:        gomail.Address{Name: "G", Address: "g@x"},
		To:          []gomail.Address{{Address: "a@x"}},
		Cc:          []gomail.Address{{Address: "b@x"}},
		Subject:     "s",
		Body:        "hello",
		InReplyTo:   "<id@host>",
		References:  []string{"<r1@host>", "<r2@host>"},
		Attachments: []string{"/tmp/a.txt"},
	}
	b, err := EncodeDraft(in)
	if err != nil {
		t.Fatalf("EncodeDraft: %v", err)
	}
	out, err := DecodeDraft(b)
	if err != nil {
		t.Fatalf("DecodeDraft: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\nin = %+v\nout = %+v", in, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/compose/ -run TestDraftGobRoundTrip -v
```

Expected: FAIL — `EncodeDraft` / `DecodeDraft` undefined.

- [ ] **Step 3: Implement codec**

Create `internal/compose/codec.go`:

```go
// SPDX-License-Identifier: MIT

package compose

import (
	"bytes"
	"encoding/gob"
)

// EncodeDraft serializes d for storage in the cache drafts table.
// gob is used because Draft carries gomail.Address values, which
// JSON cannot round-trip cleanly when Name contains non-ASCII.
func EncodeDraft(d Draft) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeDraft is the inverse of EncodeDraft.
func DecodeDraft(b []byte) (Draft, error) {
	var d Draft
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&d); err != nil {
		return Draft{}, err
	}
	return d, nil
}
```

- [ ] **Step 4: Run codec test**

```bash
go test ./internal/compose/ -run TestDraftGobRoundTrip -v
```

Expected: PASS.

- [ ] **Step 5: Add cross-boundary message types**

Open `internal/ui/compose/msgs.go`. (If the file does not exist, search for where compose's exported Msg types live — they are documented in `invariants.md` as living in `<subpkg>/msgs.go`. If they're currently inline in `model.go`, create `msgs.go` and migrate them in this step.)

Add:

```go
// EnqueuePushDraftMsg signals the App to enqueue a PushDraft op
// against the cache outbox. Compose emits this from the 5-min
// server-push timer and from the close-with-save path.
type EnqueuePushDraftMsg struct {
	DraftID       string
	Folder        string
	MIME          []byte
	PrevServerUID string
}

// DraftPersistedMsg is emitted by compose after a successful local
// UpsertDraft. App ignores it; useful in tests.
type DraftPersistedMsg struct {
	DraftID string
}
```

- [ ] **Step 6: Write the failing model test**

In `internal/ui/compose/model_test.go` (create if missing), add:

```go
func TestModel_AutosaveDebounce(t *testing.T) {
	cache := newFakeCacheAccount(t)
	m := New(NewStyles(testTheme(t)), "geoff@907.life")
	m.SetCache(cache)
	m.SetSize(80, 24)

	// Simulate a keystroke: dirty flag set, autosave tick scheduled.
	m, _ = m.Update(simulateKey('h'))
	if !m.localDirty {
		t.Fatalf("after keystroke: localDirty should be true")
	}

	// Fire the autosave tick.
	m2, _ := m.Update(autosaveTickMsg{})
	if m2.localDirty {
		t.Errorf("after autosaveTick: localDirty should be cleared")
	}
	if got := cache.upsertCalls; got != 1 {
		t.Errorf("UpsertDraft calls = %d, want 1", got)
	}
}
```

The exact `simulateKey`, `testTheme`, and `newFakeCacheAccount` helpers depend on what's already in the compose test scaffold. Run:

```bash
rg -n "func.*test|fakeCache|simulateKey" internal/ui/compose/ | head
```

If a fake cache account doesn't exist for compose tests, add a minimal one in the test file (interface that compose expects: `UpsertDraft(ctx, id, payload) error`). Define a small interface in `model.go` so the model can take either `*cache.Account` or the fake:

```go
// CacheStore is the subset of cache.Account compose needs. Defined
// here so model_test.go can inject a fake without depending on
// internal/cache.
type CacheStore interface {
	UpsertDraft(ctx context.Context, draftID string, payload []byte) error
	LoadDraft(ctx context.Context, draftID string) ([]byte, error)
}
```

This is a real seam (test fake), so it satisfies the go-conventions "single-impl interfaces require a named seam" rule.

- [ ] **Step 7: Run model test**

```bash
go test ./internal/ui/compose/ -run TestModel_AutosaveDebounce -v
```

Expected: FAIL — `m.localDirty`, `autosaveTickMsg`, `m.SetCache`, `CacheStore` not yet defined.

- [ ] **Step 8: Implement compose lifecycle**

In `internal/ui/compose/model.go`:

1. Add fields to the `Model` struct (find the existing struct definition):

```go
type Model struct {
	// ... existing fields ...

	cache       CacheStore
	draftID     string
	localDirty  bool
	pushDirty   bool
	lastEditAt  time.Time
	lastPushAt  time.Time
}
```

2. Add the `CacheStore` interface near the top of the file (under the package doc).

3. Add `SetCache` and a draftID accessor:

```go
// SetCache wires the cache store. Must be called before Init().
func (m *Model) SetCache(c CacheStore) { m.cache = c }

// DraftID returns the UUID for this compose instance. Allocated by
// New() (fresh) or Open() (resumed).
func (m *Model) DraftID() string { return m.draftID }
```

4. Modify `New` to allocate a UUID:

```go
func New(styles Styles, accountEmail string) *Model {
	return &Model{
		// ... existing init ...
		draftID: newDraftID(),
	}
}
```

with helper:

```go
import "github.com/google/uuid"

func newDraftID() string { return uuid.NewString() }
```

If `github.com/google/uuid` is not already a dependency, run `go get github.com/google/uuid` and `go mod tidy` as part of this step. If the project already vendors a UUID source, use that.

5. Add `Open` entry-point:

```go
// Open seeds a Model from a cached draft. Called by the App when
// restoring from a Drafts-folder row. The caller passes the existing
// draft_id so the autosave path keeps writing to the same row.
func Open(styles Styles, accountEmail, draftID string, d compose.Draft) *Model {
	m := New(styles, accountEmail)
	m.draftID = draftID
	m.Seed(d) // existing seed path
	m.localDirty = false
	m.pushDirty = false
	return m
}
```

(Adjust `compose.Draft` import path to match how `internal/ui/compose` aliases the domain package — likely `mailcompose "github.com/glw907/poplar/internal/compose"`.)

6. Add the autosave tick message and tick scheduler:

```go
type autosaveTickMsg struct{}
type serverPushTickMsg struct{}

const (
	autosaveDelay     = 1 * time.Second
	serverPushDelay   = 5 * time.Minute
)

func (m *Model) scheduleAutosaveCmd() tea.Cmd {
	return tea.Tick(autosaveDelay, func(time.Time) tea.Msg { return autosaveTickMsg{} })
}

func (m *Model) scheduleServerPushCmd() tea.Cmd {
	return tea.Tick(serverPushDelay, func(time.Time) tea.Msg { return serverPushTickMsg{} })
}
```

7. In `Init`, schedule both timers and write an empty row:

```go
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.scheduleAutosaveCmd(),
		m.scheduleServerPushCmd(),
	}
	if m.cache != nil {
		cmds = append(cmds, m.upsertDraftCmd())
	}
	return tea.Batch(cmds...)
}

func (m *Model) upsertDraftCmd() tea.Cmd {
	id := m.draftID
	cache := m.cache
	d := m.currentDraft() // existing accessor on the model
	return func() tea.Msg {
		payload, err := mailcompose.EncodeDraft(d)
		if err != nil {
			return uicore.ErrorMsg{Err: fmt.Errorf("encode draft: %w", err)}
		}
		if err := cache.UpsertDraft(context.Background(), id, payload); err != nil {
			return uicore.ErrorMsg{Err: fmt.Errorf("save draft: %w", err)}
		}
		return DraftPersistedMsg{DraftID: id}
	}
}
```

(Note: `currentDraft()` accessor must already exist or be added — it returns the current `compose.Draft` from the model fields. Search `func (m *Model)` in `model.go` for an existing `Draft()` accessor; reuse it.)

8. In `Update`, add dirty-marking on edit and tick handling:

```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case autosaveTickMsg:
		// Only flush if dirty AND idle ≥ debounce window.
		if m.localDirty && time.Since(m.lastEditAt) >= autosaveDelay {
			m.localDirty = false
			return m, tea.Batch(m.upsertDraftCmd(), m.scheduleAutosaveCmd())
		}
		return m, m.scheduleAutosaveCmd()

	case serverPushTickMsg:
		if m.pushDirty {
			m.pushDirty = false
			m.lastPushAt = time.Now()
			d := m.currentDraft()
			payload, err := mailcompose.EncodeDraft(d)
			if err != nil {
				return m, m.scheduleServerPushCmd()
			}
			mime, err := mailcompose.AssembleMIME(d, time.Now())
			if err != nil {
				return m, m.scheduleServerPushCmd()
			}
			_ = payload // upsert already happened on autosave
			return m, tea.Batch(
				func() tea.Msg {
					return EnqueuePushDraftMsg{
						DraftID:       m.draftID,
						Folder:        m.draftsFolder,
						MIME:          mime,
						PrevServerUID: m.prevServerUID,
					}
				},
				m.scheduleServerPushCmd(),
			)
		}
		return m, m.scheduleServerPushCmd()

	// ... existing cases ...
	}

	// After delegating to text inputs: if any keystroke landed, mark dirty.
	if isEditMsg(msg) {
		m.localDirty = true
		m.pushDirty = true
		m.lastEditAt = time.Now()
	}
	return m, nil
}
```

`isEditMsg` is a small helper distinguishing keystrokes that change content from navigation/focus events. Define it as needed for the existing input model.

Add the `draftsFolder` and `prevServerUID` fields to the `Model` struct; the App sets them before `Init`:

```go
// SetDraftTarget configures the Drafts-folder name and the previous
// server UID for this draft. The App calls this before Init() (for
// fresh) or after Open() (for restored).
func (m *Model) SetDraftTarget(folder string, prevUID string) {
	m.draftsFolder = folder
	m.prevServerUID = prevUID
}
```

When the drainer succeeds, the App reads the new `server_uid` from `cache.Events()` and re-calls `SetDraftTarget` so the next push targets the latest UID. (App-side wiring lands in task 8.)

- [ ] **Step 9: Run model test**

```bash
go test ./internal/ui/compose/ -run TestModel -v
go build ./...
```

Expected: PASS; build green.

- [ ] **Step 10: Commit**

```bash
git add internal/compose/codec.go internal/compose/codec_test.go internal/ui/compose/model.go internal/ui/compose/msgs.go internal/ui/compose/model_test.go go.mod go.sum
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 7: compose lifecycle — draftID + autosave + push tick

1s debounce flushes to cache.UpsertDraft via tea.Cmd. 5-min tick
emits EnqueuePushDraftMsg when pushDirty. CacheStore interface is
the test seam. Open() restores a Model from an existing draft_id;
New() allocates a fresh UUID. SetDraftTarget threads the Drafts
folder name and prev server UID for push.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: App routing — Drafts Enter, fresh c, close-confirm, Send/Discard

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/keys.go` (if quit/discard binding semantics change)
- Test: `internal/ui/app_test.go`

**Pre-step:** Re-read `elm-conventions` and the App seam invariants. App owns ConfirmModal already (ADR-0094); reuse it.

- [ ] **Step 1: Write the failing test**

In `internal/ui/app_test.go`, add:

```go
func TestApp_FreshComposeAllocatesDraftID(t *testing.T) {
	app, _ := newTestApp(t, withJMAPBackend())
	app, _ = app.Update(simulateKey('c'))
	if app.compose == nil {
		t.Fatalf("compose nil after 'c'")
	}
	if app.compose.DraftID() == "" {
		t.Errorf("DraftID empty after fresh compose")
	}
}

func TestApp_DraftsRowEnterOpensExistingDraft(t *testing.T) {
	app, fakeCache := newTestApp(t, withJMAPBackend())
	// Pre-seed a draft row with a known server_uid.
	if err := fakeCache.UpsertDraft(context.Background(), "d-known",
		mustEncodeDraft(t, compose.Draft{Subject: "saved"})); err != nil {
		t.Fatalf("seed UpsertDraft: %v", err)
	}
	if err := fakeCache.MarkDraftPushed(context.Background(), "d-known",
		mail.UID("server-77"), "Drafts"); err != nil {
		t.Fatalf("seed MarkDraftPushed: %v", err)
	}

	app = openDraftsFolderInTest(t, app)
	app, _ = app.Update(simulateEnterOnUID(app, "server-77"))

	if app.compose == nil {
		t.Fatalf("compose nil after Enter on Drafts row")
	}
	if app.compose.DraftID() != "d-known" {
		t.Errorf("DraftID = %q, want d-known", app.compose.DraftID())
	}
}

func TestApp_EscOnDirtyComposeOpensConfirm(t *testing.T) {
	app, _ := newTestApp(t, withJMAPBackend())
	app, _ = app.Update(simulateKey('c'))
	app, _ = app.Update(simulateKey('h')) // dirty
	app, _ = app.Update(simulateKey(tea.KeyEsc))
	if !app.confirmModalOpen() {
		t.Errorf("Esc on dirty compose did not open ConfirmModal")
	}
}
```

The test scaffold helpers (`newTestApp`, `withJMAPBackend`, `simulateKey`, `simulateEnterOnUID`, `openDraftsFolderInTest`, `mustEncodeDraft`, `confirmModalOpen`) follow existing patterns in `app_test.go`. Adapt to whatever names exist there; do not rewrite the scaffold.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/ui/ -run TestApp_FreshCompose -v
go test ./internal/ui/ -run TestApp_DraftsRowEnter -v
go test ./internal/ui/ -run TestApp_EscOnDirty -v
```

Expected: all FAIL.

- [ ] **Step 3: Wire compose to cache and gate on IsJMAP**

In `internal/ui/app.go`, in the `c`-key handler (search for `key.Matches(msg, m.keys.Compose)`):

```go
case key.Matches(msg, m.keys.Compose):
	if !m.acct.Backend().IsJMAP() {
		// Pre-9h.6: IMAP accounts retain in-memory-only compose.
		m.compose = uicompose.New(uicompose.NewStyles(m.theme), m.acct.AccountEmail())
		m.compose.SetSize(w, h)
		return m, m.compose.Init()
	}
	m.compose = uicompose.New(uicompose.NewStyles(m.theme), m.acct.AccountEmail())
	m.compose.SetCache(m.acct)                                        // *cache.Account satisfies CacheStore
	m.compose.SetDraftTarget(m.draftsFolderName(), "")                // fresh: no prevUID
	m.compose.SetSize(w, h)
	return m, m.compose.Init()
```

`m.acct` is a `*cache.Account` per invariants. `draftsFolderName()` resolves the canonical Drafts folder from `mail.Classify` results; add it as a small helper if absent:

```go
func (m *App) draftsFolderName() string {
	for _, f := range m.acct.ListFolders() {
		if f.Role == mail.RoleDrafts {
			return f.Name
		}
	}
	return "Drafts"
}
```

(Match the existing `mail.Folder` / `mail.FolderEntry` shape — use whatever role accessor exists.)

- [ ] **Step 4: Wire Drafts-folder Enter routing**

In the Enter-on-message handler (search for the message-list Enter dispatch), add a Drafts-folder branch:

```go
if m.currentFolderRole() == mail.RoleDrafts && m.acct.Backend().IsJMAP() {
	uid := m.messageList.SelectedUID()
	row, err := m.acct.LookupDraftByServerUID(context.Background(), uid)
	if errors.Is(err, sql.ErrNoRows) {
		// Server draft never opened locally. Reconstruct from cached MIME.
		mime, err := m.acct.FetchBody(context.Background(), uid)
		if err == nil {
			d, err := mailcompose.ParseDraftMIME(mime)
			if err == nil {
				newID := uicompose.AllocDraftID()
				payload, _ := mailcompose.EncodeDraft(d)
				_ = m.acct.UpsertDraft(context.Background(), newID, payload)
				_ = m.acct.MarkDraftPushed(context.Background(), newID, uid, m.draftsFolderName())
				m.compose = uicompose.Open(uicompose.NewStyles(m.theme), m.acct.AccountEmail(), newID, d)
				m.compose.SetCache(m.acct)
				m.compose.SetDraftTarget(m.draftsFolderName(), string(uid))
				m.compose.SetSize(w, h)
				return m, m.compose.Init()
			}
		}
		break // fall through to normal message-open if reconstruction fails
	}
	if err != nil {
		return m, func() tea.Msg { return uicore.ErrorMsg{Err: err} }
	}
	d, err := mailcompose.DecodeDraft(row.Payload)
	if err != nil {
		return m, func() tea.Msg { return uicore.ErrorMsg{Err: err} }
	}
	m.compose = uicompose.Open(uicompose.NewStyles(m.theme), m.acct.AccountEmail(), row.DraftID, d)
	m.compose.SetCache(m.acct)
	m.compose.SetDraftTarget(m.draftsFolderName(), string(row.ServerUID))
	m.compose.SetSize(w, h)
	return m, m.compose.Init()
}
```

Add `LookupDraftByServerUID` to `internal/cache/drafts.go`:

```go
// LookupDraftByServerUID returns the drafts row whose server_uid
// matches uid, or sql.ErrNoRows if none. Used by the App when the
// user opens a Drafts-folder row.
func (a *Account) LookupDraftByServerUID(ctx context.Context, uid mail.UID) (DraftRow, error) {
	row := a.db.QueryRowContext(ctx, `
        SELECT draft_id, COALESCE(server_uid, ''), COALESCE(server_folder, ''),
               payload, dirty, created_at, updated_at,
               COALESCE(last_pushed_at, 0)
        FROM drafts WHERE server_uid = ?`, string(uid))
	var r DraftRow
	var serverUID, serverFolder string
	var dirtyInt int
	var created, updated, pushed int64
	err := row.Scan(&r.DraftID, &serverUID, &serverFolder, &r.Payload,
		&dirtyInt, &created, &updated, &pushed)
	if err != nil {
		return DraftRow{}, err
	}
	r.ServerUID = mail.UID(serverUID)
	r.ServerFolder = serverFolder
	r.Dirty = dirtyInt != 0
	r.CreatedAt = time.Unix(0, created)
	r.UpdatedAt = time.Unix(0, updated)
	if pushed != 0 {
		r.LastPushedAt = time.Unix(0, pushed)
	}
	return r, nil
}
```

Add `AllocDraftID` to `internal/ui/compose/model.go` (export the existing `newDraftID`):

```go
// AllocDraftID returns a fresh draft UUID. Used by the App when
// restoring a server-side draft that has no matching local row.
func AllocDraftID() string { return newDraftID() }
```

- [ ] **Step 5: Wire EnqueuePushDraftMsg handling**

In `internal/ui/app.go`, in the `Update` switch, add:

```go
case uicompose.EnqueuePushDraftMsg:
	_, err := m.acct.QueuePushDraft(context.Background(),
		msg.DraftID, msg.Folder, msg.MIME, mail.UID(msg.PrevServerUID))
	if err != nil {
		return m, func() tea.Msg { return uicore.ErrorMsg{Err: err} }
	}
	return m, nil
```

- [ ] **Step 6: Wire close-confirm modal**

The existing close path (search for `pendingComposeDiscard`) currently always discards. Replace with a save/discard branch:

```go
case uicompose.CancelMsg:
	if !m.compose.IsDirty() && !m.compose.HasContent() {
		// Empty: silent close + delete the empty row written at Init.
		id := m.compose.DraftID()
		_ = m.acct.DeleteDraft(context.Background(), id)
		m.compose = nil
		return m, nil
	}
	if !m.compose.IsDirty() {
		// Clean resumed draft: silent close, keep the local row.
		m.compose = nil
		return m, nil
	}
	// Dirty: open the save/discard modal.
	m.confirmModal = uicore.NewConfirmModal(
		"Save draft?",
		"[Y] Save and close   [N] Discard   [Esc] Keep editing",
		uicore.ConfirmTernary, // Y / N / Esc
	)
	m.pendingComposeDecision = true
	return m, nil
```

Add `IsDirty()` and `HasContent()` accessors to `compose.Model`:

```go
func (m *Model) IsDirty() bool    { return m.localDirty || m.pushDirty }
func (m *Model) HasContent() bool {
	d := m.currentDraft()
	return len(d.To) > 0 || len(d.Cc) > 0 || len(d.Bcc) > 0 ||
		d.Subject != "" || d.Body != "" || len(d.Attachments) > 0
}
```

Handle the modal's three terminal messages in App's Update:

```go
case uicore.ConfirmYesMsg:
	if m.pendingComposeDecision {
		m.pendingComposeDecision = false
		// Save and close.
		d := m.compose.Draft()
		mime, err := mailcompose.AssembleMIME(d, time.Now())
		if err != nil {
			m.compose = nil
			return m, func() tea.Msg { return uicore.ErrorMsg{Err: err} }
		}
		payload, _ := mailcompose.EncodeDraft(d)
		_ = m.acct.UpsertDraft(context.Background(), m.compose.DraftID(), payload)
		_, err = m.acct.QueuePushDraft(context.Background(),
			m.compose.DraftID(), m.draftsFolderName(), mime,
			mail.UID(m.compose.PrevServerUID()))
		m.compose = nil
		if err != nil {
			return m, func() tea.Msg { return uicore.ErrorMsg{Err: err} }
		}
		return m, nil
	}
	// ... existing ConfirmYes handlers ...

case uicore.ConfirmNoMsg:
	if m.pendingComposeDecision {
		m.pendingComposeDecision = false
		id := m.compose.DraftID()
		prevUID := m.compose.PrevServerUID()
		// If the server has an image, queue a Destroy.
		if prevUID != "" {
			_, _ = m.acct.QueueOp(context.Background(),
				m.draftsFolderName(), mail.UID(prevUID), cache.DestroyArgs{})
		}
		_ = m.acct.DeleteDraft(context.Background(), id)
		m.compose = nil
		return m, nil
	}
	// ... existing ConfirmNo handlers ...
```

Add `PrevServerUID()` accessor to `compose.Model`:

```go
func (m *Model) PrevServerUID() string { return m.prevServerUID }
```

If `uicore.ConfirmTernary` doesn't exist, extend the existing `ConfirmModal` to support a third "cancel-on-Esc" outcome (it likely already does — Esc currently dismisses). Use whatever message the existing modal emits on Esc and treat it as "keep editing" by leaving compose mounted.

- [ ] **Step 7: Wire Send cleanup**

In the `uicompose.SentMsg` handler, add cleanup:

```go
case uicompose.SentMsg:
	if m.compose != nil {
		id := m.compose.DraftID()
		prevUID := m.compose.PrevServerUID()
		if prevUID != "" {
			_, _ = m.acct.QueueOp(context.Background(),
				m.draftsFolderName(), mail.UID(prevUID), cache.DestroyArgs{})
		}
		_ = m.acct.DeleteDraft(context.Background(), id)
	}
	m.compose = nil
	// ... existing Sent handling ...
```

- [ ] **Step 8: Run tests**

```bash
go test ./internal/ui/ -run TestApp -v
go build ./...
```

Expected: all three new App tests PASS; no regressions.

- [ ] **Step 9: Commit**

```bash
git add internal/ui/app.go internal/ui/keys.go internal/ui/app_test.go internal/ui/compose/model.go internal/cache/drafts.go
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 8: App routing — Drafts Enter, save/discard modal

Fresh c allocates a draftID and wires SetCache/SetDraftTarget. Enter
on a Drafts-folder row looks up by server_uid and Opens the cached
draft (or reconstructs via ParseDraftMIME if no local row exists).
Esc on a dirty compose opens a Save/Discard ConfirmModal; Send
removes the local row and queues Destroy on any server image.
IsJMAP() gate keeps IMAP accounts on today's in-memory flow until
9h.6.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Drafts-folder view projection (local-only drafts)

**Files:**
- Modify: `internal/cache/reads.go` (extend `QueryFolder` for Drafts folder)
- Test: `internal/cache/drafts_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/drafts_test.go`:

```go
func TestQueryFolder_DraftsLocalOnly(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	mustEnsureFolder(t, a, "Drafts")
	// Mark Drafts folder as role=drafts so QueryFolder branches on it.
	if _, err := a.db.Exec(`UPDATE folders SET role = 'drafts' WHERE name = 'Drafts'`); err != nil {
		t.Fatalf("set drafts role: %v", err)
	}

	// One local-only draft.
	if err := a.UpsertDraft(ctx, "d-local", []byte("payload")); err != nil {
		t.Fatalf("UpsertDraft: %v", err)
	}

	uids, total, err := a.QueryFolder("Drafts", 0, 100)
	if err != nil {
		t.Fatalf("QueryFolder: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(uids) != 1 || string(uids[0]) != "draft:d-local" {
		t.Errorf("uids = %v, want [draft:d-local]", uids)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cache/ -run TestQueryFolder_DraftsLocalOnly -v
```

Expected: FAIL — `QueryFolder` returns 0 rows for the Drafts folder.

- [ ] **Step 3: Extend QueryFolder**

In `internal/cache/reads.go`, find `QueryFolder`. After the normal server-message query, add a branch for the drafts role:

```go
func (a *Account) QueryFolder(folderName string, offset, limit int) ([]mail.UID, int, error) {
	// ... existing server-message query ...
	uids, total, err := a.queryServerMessages(folderName, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	role, err := a.folderRole(folderName)
	if err == nil && role == "drafts" {
		drafts, err := a.ListDrafts(context.Background())
		if err == nil {
			for _, d := range drafts {
				if d.ServerUID == "" {
					uids = append(uids, mail.UID("draft:"+d.DraftID))
					total++
				}
			}
		}
	}
	return uids, total, nil
}
```

Add the small helper:

```go
func (a *Account) folderRole(name string) (string, error) {
	var role sql.NullString
	err := a.db.QueryRow(`SELECT role FROM folders WHERE name = ?`, name).Scan(&role)
	if err != nil {
		return "", err
	}
	if !role.Valid {
		return "", nil
	}
	return role.String, nil
}
```

(If the existing function isn't named `queryServerMessages`, extract it inline; keep the change minimal.)

The synthetic `draft:<id>` UID is recognized by the App's Enter handler (task 8 already routes by role to the local-draft path; extend it to also recognize the `draft:` prefix for local-only rows that have no server_uid yet):

In `internal/ui/app.go`, in the Enter handler from task 8, add a branch before the `LookupDraftByServerUID` call:

```go
if strings.HasPrefix(string(uid), "draft:") {
	id := strings.TrimPrefix(string(uid), "draft:")
	payload, err := m.acct.LoadDraft(context.Background(), id)
	if err != nil {
		return m, func() tea.Msg { return uicore.ErrorMsg{Err: err} }
	}
	d, err := mailcompose.DecodeDraft(payload)
	if err != nil {
		return m, func() tea.Msg { return uicore.ErrorMsg{Err: err} }
	}
	m.compose = uicompose.Open(uicompose.NewStyles(m.theme), m.acct.AccountEmail(), id, d)
	m.compose.SetCache(m.acct)
	m.compose.SetDraftTarget(m.draftsFolderName(), "")
	m.compose.SetSize(w, h)
	return m, m.compose.Init()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cache/ -run TestQueryFolder_DraftsLocalOnly -v
go test ./internal/cache/... -v
go test ./internal/ui/ -v
go build ./...
```

Expected: new test PASSES; no regressions.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/reads.go internal/cache/drafts.go internal/cache/drafts_test.go internal/ui/app.go
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 9: Drafts-folder local-only projection

QueryFolder appends draft: synthetic UIDs for drafts rows with
no server_uid (offline-created or pending first push). The App's
Enter handler decodes draft:<id> directly from the local store.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: tmux verification + feature matrix + research note + ADR

**Files:**
- Create: `docs/poplar/research/2026-05-06-drafts-sync-norms.md`
- Modify: `docs/poplar/feature-matrix.md`
- Create: `docs/poplar/decisions/0164-drafts-persistence-jmap.md`

- [ ] **Step 1: tmux smoke test against Fastmail**

Reference: `.claude/docs/tmux-testing.md`. Pre-req: `$FASTMAIL_API_TOKEN` is in `~/.local/secrets`; the user's account is `geoff@907.life`.

```bash
make install
```

Then start poplar in a tmux session at 80×24:

```bash
tmux new-session -d -s poplar-drafts -x 80 -y 24 'poplar'
sleep 2
```

Walk through manually, capturing at each step:

1. Press `c` to compose. Verify the compose surface mounts.
2. Type a few characters into the body. Wait 2 seconds.
3. Verify the cache row exists:

```bash
poplar cache drafts list   # if the CLI exposes this; otherwise:
sqlite3 ~/.cache/poplar/<slug>/mail.db 'SELECT draft_id, length(payload), dirty FROM drafts'
```

Expected: one row, dirty=1.

4. Press Esc. Verify the Save/Discard modal renders. Capture:

```bash
tmux capture-pane -t poplar-drafts -p > /tmp/drafts-modal.txt
```

5. Press `y` to save. Verify the modal closes and compose unmounts.
6. Inspect outbox:

```bash
sqlite3 ~/.cache/poplar/<slug>/mail.db "SELECT id, kind, status FROM outbox WHERE kind = 'push-draft'"
```

Expected: one row, status=`done` (or `executing` briefly).
7. Verify the draft appears in Fastmail web (open https://app.fastmail.com/mail/Drafts/ in a browser). Confirm the body matches.
8. In poplar, navigate to the Drafts folder. Press Enter on the draft. Verify compose reopens with the saved body.
9. Edit the body, press Esc, Y. Wait. Reload Fastmail web — verify the edit landed and the prior draft is gone (replaced).

If any step fails, fix the underlying bug before continuing. Capture both 80×24 and 120×40 if any UI surface looks suspect:

```bash
tmux new-session -d -s poplar-drafts-wide -x 120 -y 40 'poplar'
```

Kill the session at the end:

```bash
tmux kill-session -t poplar-drafts
tmux kill-session -t poplar-drafts-wide 2>/dev/null
```

- [ ] **Step 2: Write the prior-art research note**

Create `docs/poplar/research/2026-05-06-drafts-sync-norms.md`:

```markdown
# Draft sync norms across email clients

Snapshot 2026-05-06. Used during Pass 9h.5 brainstorm to pick a
cadence and storage model for poplar drafts.

## Pattern summary

GUI clients with autosave: server-canonical, periodic upload,
APPEND+destroy-old (because IMAP can't edit-in-place; JMAP Email
is immutable too — the destroy-old must be batched with the
create-new). Cadence varies (10s → 5min). Last-write-wins is
universal — no client surveyed does merge-on-conflict for drafts.

Terminal clients (mutt, aerc, alpine) are an outlier: postpone-
style, user-driven, no autosave at all.

## Per-client table

| Client            | Storage              | Cadence       | Push model                                 | Conflict       |
|-------------------|----------------------|---------------|--------------------------------------------|----------------|
| Thunderbird       | Server Drafts        | 5 min default | APPEND new + STORE \Deleted on old         | Last-write-wins |
| Apple Mail        | Server Drafts        | ~30s          | APPEND + delete-old                        | Last-write-wins |
| Outlook (IMAP)    | Server Drafts        | ~3 min        | APPEND + delete-old                        | Last-write-wins |
| Geary             | Server Drafts        | ~10s          | APPEND + delete-old                        | Last-write-wins |
| Evolution         | Local; server opt    | ~60s          | APPEND when server-configured              | Last-write-wins |
| K-9 Mail          | Local then server    | On close only | APPEND + delete-old                        | n/a             |
| mutt              | $postponed mbox/IMAP | Manual        | None (^X postpone)                         | n/a             |
| aerc              | [postpone] folder    | Manual        | None                                       | n/a             |
| alpine            | postponed-msgs       | Manual (^O)   | None                                       | n/a             |
| Fastmail web      | Server Drafts        | ~real-time    | JMAP Email/import + destroy via creation-ref | Last-write-wins |
| Gmail web         | Server Drafts        | ~1–3s         | Gmail API patch (server mutable)           | Last-write-wins |

## Poplar's choice

- Local SQLite as 1s-debounce edit buffer.
- Server push on close + 5 min idle timer (Thunderbird's cadence).
- APPEND+destroy-old is the universal pattern; JMAP gives us atomicity
  via creation-ref, IMAP (9h.6) lands without atomicity (orphans
  possible on partial failure).
- Last-write-wins; no merge.

Reference: ADR-0164.
```

- [ ] **Step 3: Update feature matrix**

In `docs/poplar/feature-matrix.md`, find the row "Drafts (saved & resumable)". Below it, insert a new row:

```
| Drafts cross-device sync           |  —   |  —   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9h.5|
```

(Match the column count and `:` alignment of the surrounding rows. Update poplar's column on the existing "Drafts (saved & resumable)" row from `⏳9h.5` to `✓` once 9h.5 ships — that update belongs to the pass-end consolidation step, not this task.)

- [ ] **Step 4: Write ADR-0164**

Create `docs/poplar/decisions/0164-drafts-persistence-jmap.md`:

```markdown
---
title: Drafts persistence (JMAP, end-to-end)
status: accepted
date: 2026-05-06
---

## Context

Closing compose without sending lost the draft. Cross-device draft
visibility is table-stakes for users who flip between poplar and
Fastmail web on phone. Pass 9h.5 lands persistence end-to-end for
JMAP-backed accounts; Pass 9h.6 layers IMAP on the same pipeline.

The dominant prior-art model (Thunderbird, Apple Mail, Geary,
Outlook, Fastmail web, Gmail web) is server-canonical with
APPEND+destroy-old upload, last-write-wins conflict, cadences
ranging 10s–5min. Terminal clients (mutt, aerc, alpine) have no
autosave at all — poplar is functionally a full-featured client,
so the GUI pattern applies.

## Decision

- Cache schema v7 adds a `drafts` table keyed by App-internal UUID.
  `server_uid` is nullable (null = local-only, not yet pushed).
- `compose.Model` autosaves to the local cache on a 1s debounce
  and emits an `EnqueuePushDraftMsg` every 5 min while dirty
  (Thunderbird's cadence) and on close-with-save.
- A new outbox op `KindPushDraft` carries the assembled MIME in
  `outbox.payload` and routes through the existing drainer
  conflict matrix.
- `mail.Backend.PushDraft(folder, mime, prevUID) (newUID, error)`
  is the protocol primitive. JMAP impl batches `Email/import`
  with `$draft` keyword and `Email/set destroy` on prevUID in a
  single round-trip.
- The IMAP impl is a stub returning `mail.ErrUnsupported`. The
  App gates draft persistence on `Backend.IsJMAP()` so IMAP
  accounts retain today's in-memory-only compose flow until
  Pass 9h.6 fills in the impl.
- Esc on a dirty non-empty compose opens a Save/Discard
  ConfirmModal. Empty compose closes silently. Send removes the
  local row and queues a Destroy against any server image.
- Conflict policy is last-write-wins; no UI signal in 9h.5
  (banner lands in 9h.6).
- The Drafts folder's message-list view projects local-only
  drafts (server_uid IS NULL) as synthetic `draft:<id>` UIDs
  appended to the normal server-synced rows.

## Consequences

- Cross-device draft visibility within ~5 min of stopping
  typing (the push-tick cadence) on JMAP accounts. Users who
  close compose see updates immediately on the next device open.
- IMAP users see no behavior change in 9h.5. 9h.6 lifts the gate.
- Schema v7 is a pure additive migration; the partial index on
  non-null `server_uid` keys the App's Drafts-Enter lookup.
- The `CacheStore` interface in `internal/ui/compose/` is a real
  test seam (per go-conventions: single-impl interfaces require
  a named seam) — `*cache.Account` is the production impl;
  fakes power compose's unit tests.
```

- [ ] **Step 5: make check**

```bash
make check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add docs/poplar/research/2026-05-06-drafts-sync-norms.md \
        docs/poplar/feature-matrix.md \
        docs/poplar/decisions/0164-drafts-persistence-jmap.md
git commit -m "$(cat <<'EOF'
Pass 9h.5 task 10: docs — research note + feature matrix + ADR-0164

Captures the prior-art table that grounded the cadence choice
(Thunderbird's 5-min idle, push-on-close). ADR-0164 records the
JMAP-only end-to-end shape; ADR-0165 in pass 9h.6 will codify
IMAP semantics + conflict banner.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Pass-end consolidation

After all 10 tasks land green:

- [ ] Run `/simplify` on the diff. Apply genuine wins, then proceed.
- [ ] Idiomatic-bubbletea checklist on the compose lifecycle changes (skill `poplar-pass` step 1b).
- [ ] Update `docs/poplar/invariants.md` — add a binding fact for drafts persistence under "Compose" or a new "Drafts" subsection. Update the decision index table to include 0164.
- [ ] Update `docs/poplar/STATUS.md` — mark 9h.5 done; replace the starter prompt with 9h.6 (IMAP push + conflict banner). Move the spec from `docs/superpowers/specs/` to `docs/superpowers/archive/specs/` and the plan from `docs/superpowers/plans/` to `docs/superpowers/archive/plans/` via `git mv`.
- [ ] `make check` final.
- [ ] Commit consolidation: `git add -A && git commit -m "Pass 9h.5 consolidation: ADR-0164, invariants, STATUS, archive"`
- [ ] `git push && make install`.

---

## Self-review

**Spec coverage:**
- Schema v7 → task 1 ✓
- cache.Account drafts API → task 2 ✓
- PushDraftOp + drainer dispatch → tasks 3, 5 ✓
- mail.Backend.PushDraft + IMAP stub → task 4 ✓
- JMAP PushDraft impl → task 5 ✓
- ParseDraftMIME → task 6 ✓
- compose.Model lifecycle (autosave + push tick + draftID + Open/SetCache/SetDraftTarget) → task 7 ✓
- App routing (fresh c, Drafts Enter, EnqueuePushDraftMsg, close-confirm, Send cleanup) → task 8 ✓
- Drafts-folder local-only projection → task 9 ✓
- Feature matrix + research note + ADR + tmux → task 10 ✓

**Type consistency:**
- `mail.UID` (string) used consistently for `server_uid` in Go, `TEXT` in SQLite.
- `PushDraftArgs{DraftID, PrevServerUID}` shape unchanged across tasks 3, 5, 8.
- `EnqueuePushDraftMsg{DraftID, Folder, MIME, PrevServerUID}` shape unchanged across tasks 7, 8.
- `compose.Draft` value type unchanged; new methods (`SetCache`, `SetDraftTarget`, `IsDirty`, `HasContent`, `DraftID`, `PrevServerUID`, `AllocDraftID`, `Open`) all use consistent names across tasks 7, 8, 9.
- `CacheStore` interface in compose pkg defined once in task 7, used by tests there and by App seam (App's `*cache.Account` satisfies it implicitly in task 8).

**Placeholder scan:** None.

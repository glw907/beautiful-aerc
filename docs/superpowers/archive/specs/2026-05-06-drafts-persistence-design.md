# Drafts persistence — design

**Pass:** 9h.5 (JMAP end-to-end). 9h.6 layers IMAP onto the same
pipeline.
**Issue:** #33
**Date:** 2026-05-06

## Goal

Persist compose drafts so closing the compose surface preserves
the draft and reopening restores it. The local SQLite cache holds
high-frequency edit state (1s debounce); the server's Drafts
folder is the canonical store across devices, written on close
and on a 5-minute idle timer (Thunderbird's cadence).

A user typing in poplar can close compose, switch to Fastmail
web on their phone, and find the draft within ~5 minutes.

## Scope

**In (9h.5).**

- Cache schema v7: `drafts` table.
- `cache.Account` API: `UpsertDraft`, `LoadDraft`, `ListDrafts`,
  `DeleteDraft`.
- New outbox op: `PushDraftArgs{Folder, PrevServerUID}` carrying
  assembled MIME in `outbox.payload`.
- `mail.Backend.PushDraft(folder, mime, prevUID) (newUID, error)`.
  JMAP impl (atomic `Email/import` + `destroy` via creation-ref).
  IMAP impl is a stub returning `mail.ErrUnsupported`; the App
  gates on `Backend.IsJMAP()` so IMAP accounts retain today's
  in-memory-only compose behavior.
- `compose.Model`: `draftID` field, 1s autosave debounce, 5min
  server-push timer, dirty tracking.
- App lifecycle: Drafts-folder Enter routes to compose-with-id;
  fresh `c` allocates a new draft-id; close-confirm modal;
  Send/Discard cleanup.
- Last-write-wins on conflict, silent (no banner). 9h.6 adds the
  banner alongside IMAP support.
- Feature-matrix update + research note capturing prior-art.

**Out (deferred to 9h.6 or later).**

- IMAP push impl (APPEND `\Draft` + Destroy old via UIDPLUS).
- Gmail X-GM-EXT routing for drafts.
- Conflict status-bar banner.
- Real-time per-keystroke server push (TB cadence is the v1
  answer; configurability is post-1.0).
- Draft merge / three-way reconciliation. v1 is last-write-wins.
- Multi-draft pool. ADR-0159's one-open-compose-at-a-time rule
  holds; `draftID` is per-instance.
- Signatures (9.4), address autocomplete (9.1), schedule send
  (9.2), attachments-rich compose UI (9.5).

## Architecture

Local SQLite is the high-frequency store; the server's Drafts
folder is the low-frequency canonical store. Two timers drive
the lifecycle:

```
keystroke ─[1s debounce]─▶ cache.UpsertDraft ──┐
                                               │
                          ┌────────────────────┘
                          │
                          ▼
            ┌─[5min idle while dirty]──▶ enqueue PushDraftOp ─┐
            │                                                 │
       close-with-save ─────────────────────────────────────▶ │
                                                              ▼
                                                     drainer dispatches
                                                     Backend.PushDraft
                                                              │
                                                              ▼
                                                     update server_uid
                                                     in drafts row
```

The drainer reuses the existing outbox machinery — same
conflict matrix (`ErrAuth`, `ErrNotFound`, transient backoff),
same `RetryOp`/`DiscardOp` resolution. PushDraft is folder-
scoped (no `messages` row), like Send/Append.

## Data model — schema v7

```sql
CREATE TABLE drafts (
    draft_id        TEXT PRIMARY KEY,    -- UUID, App-internal
    server_uid      TEXT,                -- mail.UID of current server image, NULL until first push
    server_folder   TEXT,                -- canonical Drafts folder name
    payload         BLOB NOT NULL,       -- gob-encoded compose.Draft
    dirty           INTEGER NOT NULL,    -- 1 if local edits not yet pushed
    created_at      INTEGER NOT NULL,    -- unix nanos
    updated_at      INTEGER NOT NULL,
    last_pushed_at  INTEGER              -- nullable; set on successful push
);

CREATE INDEX drafts_by_server_uid
    ON drafts(server_uid) WHERE server_uid IS NOT NULL;
```

`payload` is `encoding/gob` — `compose.Draft` is the only struct
encoded, and gob handles the address-list slices cleanly without
the JSON gotchas around `time.Time` and embedded interfaces. The
encoder/decoder lives in `internal/cache/drafts.go`.

`server_uid` is `TEXT` to match `mail.UID`'s underlying type;
JMAP IDs are arbitrary strings and IMAP UIDs are decimal-string
already in poplar.

## cache.Account API

```go
// UpsertDraft writes the draft payload, marks dirty, bumps
// updated_at. Creates the row on first call for a given draftID.
// Idempotent under concurrent writes; the last writer wins.
func (a *Account) UpsertDraft(ctx context.Context, draftID string, payload []byte) error

// LoadDraft returns the payload for draftID, or sql.ErrNoRows.
func (a *Account) LoadDraft(ctx context.Context, draftID string) ([]byte, error)

// ListDrafts returns all draft rows for projection into the
// Drafts-folder view (App reads these and merges with the cache's
// server-message rows under the same folder).
func (a *Account) ListDrafts(ctx context.Context) ([]DraftRow, error)

// DeleteDraft removes the local row. Caller is responsible for
// queueing a destroy of any server image (via the same tx in the
// Discard / Send paths).
func (a *Account) DeleteDraft(ctx context.Context, draftID string) error

type DraftRow struct {
    DraftID      string
    ServerUID    mail.UID  // empty if not yet pushed
    ServerFolder string
    Payload      []byte
    Dirty        bool
    UpdatedAt    time.Time
    LastPushedAt time.Time // zero if never pushed
}
```

## Outbox PushDraftOp

```go
// PushDraftArgs queues a server-side draft replace. The current
// payload lives in outbox.payload (assembled MIME). On success the
// drainer writes the new server_uid back to the drafts row and,
// if PrevServerUID was non-empty, destroys the prior server image
// in the same Backend call.
type PushDraftArgs struct {
    DraftID       string
    PrevServerUID mail.UID // empty on first push
}

func (PushDraftArgs) opKind() OpKind { return KindPushDraft }
```

`KindPushDraft` joins the `OpKind` enum.
`revertOptimisticTx` no-ops on `PushDraftArgs` (no message-row
state to mirror — drafts aren't in `messages`).

`(*Account).QueuePushDraft(ctx, draftID, folder, mime, prevUID)`
inserts a folder-scoped row carrying `mime` as `outbox.payload`.

The drainer's dispatch switch gains a `KindPushDraft` arm:

```go
case KindPushDraft:
    var a PushDraftArgs
    if err := json.Unmarshal([]byte(row.ArgsJSON), &a); err != nil { ... }
    newUID, err := backend.PushDraft(row.FolderName, row.Payload, a.PrevServerUID)
    if err != nil {
        return err  // routed through existing conflict matrix
    }
    return account.markDraftPushed(a.DraftID, newUID)
```

`markDraftPushed` updates the `drafts` row:
`server_uid = newUID`, `dirty = 0`, `last_pushed_at = now`.

Crash recovery: PushDraft, like Send/Append, transitions an
`OpExecuting` row to `OpConflict crashed-mid-execute` on
restart. The user-visible effect is a queued push that needs
`/cache outbox retry`; the local draft survives (it's in the
`drafts` table, not the outbox).

## Backend.PushDraft

```go
// PushDraft writes a new server image of a draft and (if
// prevUID is non-empty) destroys the prior image in the same
// operation. Atomicity is best-effort by backend:
//
// JMAP: one Email/set call with both create + destroy via
//       creation-ref (#k1). Succeeds-or-fails as a unit.
// IMAP: APPEND new + UID STORE \Deleted on prevUID + UID EXPUNGE.
//       Not atomic; orphans possible on partial failure.
//
// 9h.5 ships JMAP impl only. IMAP returns ErrUnsupported until 9h.6.
PushDraft(folder string, mime []byte, prevUID UID) (newUID UID, err error)
```

`mail.ErrUnsupported` joins `ErrAuth` / `ErrNotFound` as a
typed sentinel. The App layer gates draft persistence on
`Backend.IsJMAP()`; an IMAP user sees today's behavior
(in-memory compose, no autosave, no Drafts-folder routing) and
no spurious conflict rows. 9h.6 lifts the gate.

### JMAP impl

```go
func (j *jmapBackend) PushDraft(folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
    // Resolve the Drafts mailbox id (cached from session)
    // Build an Email/import call with the assembled MIME
    // Same Email/set request issues destroy on prevUID via #k1
    // Return the new server-assigned id
}
```

Reuses the same `cli.Upload` + `Email/import` path as `Send`'s
Sent-copy placement. The Drafts mailbox id is cached on the
backend at first call (mirrors the existing `identityID` cache).

The `$draft` keyword is set by `Email/import` automatically
when importing into a role:drafts mailbox; we don't set it
manually.

## compose.Model lifecycle

New fields:

```go
type Model struct {
    // ... existing ...
    draftID        string         // UUID, allocated at Open
    cache          *cache.Account // injected by App
    dirty          bool           // any field edited since last UpsertDraft
    lastDirtyAt    time.Time      // for the 1s debounce
    lastPushAt     time.Time      // for the 5min server-push timer
}
```

New messages (compose-internal, unexported):

- `autosaveTickMsg` — fires 1s after `lastDirtyAt`. If still
  dirty, runs `cache.UpsertDraft(ctx, draftID, encode(draft))`,
  clears `dirty` for the local-store sense.
- `serverPushTickMsg` — fires 5min after `lastPushAt`. If the
  draft has been pushed-dirty since `lastPushAt`, the model
  emits `enqueuePushDraftMsg` (cross-boundary, exported in
  `compose/msgs.go`) which the App handles by calling
  `cache.QueuePushDraft`.

The split between "local dirty" and "push dirty" is two flags
in the model: `localDirty` (set on edit, cleared on
`UpsertDraft`) and `pushDirty` (set on edit, cleared on
successful `markDraftPushed` event from `cache.Events()`).

`Init()` returns the autosave tick. `Update` reschedules ticks
on edit. `View()` is unchanged.

`compose.Open(draftID, draft)` is a new entry-point alongside
`compose.New`. App calls `Open` when restoring a Drafts row;
`New` allocates a fresh UUID for `c`-from-anywhere.

## App routing

- **`c` from any tab.** App generates a new UUID, calls
  `compose.New(...)` with it. Compose's `Init` writes an empty
  row via `UpsertDraft` (so the row exists for autosave to
  update; cleaned up by Discard / Send).
- **Drafts folder Enter on a row.** App reads the row's UID,
  looks up `cache.LoadDraft` keyed by `(server_uid → draft_id)`
  via the index, and calls `compose.Open(draftID, draft)`. If
  no local draft is found (e.g., a draft created in another
  client and synced server-side), App allocates a new
  `draft_id`, decodes the MIME from the cached message body,
  reconstructs a `compose.Draft`, and writes it via
  `UpsertDraft` before opening compose.

The MIME-to-Draft reconstruction lives in
`internal/compose/parse.go` as `ParseDraftMIME([]byte) (Draft,
error)` — symmetric to `AssembleMIME`. Reuses the same
`go-message/mail` reader the seeders already use.

## Drafts-folder view

The Drafts folder's message-list view is the union of:

1. **Server-synced messages** in the Drafts folder (current
   behavior — the cache's `messages` table populated by sync).
2. **Local-only drafts** — rows in the new `drafts` table
   where `server_uid IS NULL` (offline-created, not yet
   pushed). These render with the existing outbox `Q` overlay
   to mark them as pending-upload.

The projection is implemented in `cache.Account.QueryFolder`:
when the folder is the canonical Drafts folder, append local-
only drafts as synthetic `MessageInfo` rows after the server
rows, using `draft_id` as the protocol_id, the draft's Subject
as Subject, the draft's From as From, `updated_at` as Date,
and a synthetic `mail.UID = "draft:" + draftID`. The UI
recognizes the `draft:` prefix and routes Enter to
`compose.Open` directly without going through the message
read path.

This avoids special-casing the message-list View; the UID
prefix is the discriminator.

## Close-confirm modal

Wires into the existing `ConfirmModal` shell (ADR-0094 pattern,
already used for the empty-folder confirm).

```
┌────────────────────────────┐
│ Save draft?                │
│                            │
│ [Y] Save and close         │
│ [N] Discard                │
│ [Esc] Keep editing         │
└────────────────────────────┘
```

Triggered on:

- Esc with `localDirty || pushDirty || hasContent`.
- `q` (quit) with the same predicate.

Skipped on:

- Empty compose (no To, Cc, Bcc, Subject, Body, attachments) —
  silent close, no row written. (If the model already wrote a
  row at `Init`, the close path runs `DeleteDraft` to clean
  it up.)
- Clean compose (resumed draft, no edits) — silent close. The
  existing local row stays as-is.
- `Ctrl+C` — explicit discard, skips modal. Runs the same
  cleanup as the modal's "N" path.
- Send dispatch — no modal. On `QueueOutbound` success, the
  Send tx also runs `DeleteDraft` and (if `server_uid != ""`)
  enqueues a `Destroy` op against the server image. Both happen
  in one cache tx alongside the Send op insert.

The "Save and close" path:

1. `cache.UpsertDraft(ctx, draftID, encode(draft))` — flush
   any pending local changes synchronously.
2. `cache.QueuePushDraft(ctx, draftID, draftsFolder, mime,
   prevServerUID)` — enqueue server push.
3. Close compose.

The "Discard" path:

1. If `server_uid != ""`, queue a `Destroy` op against it.
2. `cache.DeleteDraft(ctx, draftID)`.
3. Close compose.

## Conflict (last-write-wins)

If `PushDraft` succeeds but `prevServerUID` was already gone
(another client edited and pushed in the meantime), the JMAP
`Email/set` returns `notFound` for the destroy and the create
still succeeds. Backend reports success; `markDraftPushed`
overwrites with the new UID. The other client's edits are
silently lost.

9h.5 logs this case at INFO and emits no UI signal. 9h.6 adds
a status-bar banner: `draft conflict — your version saved,
other edits lost`.

## Testing

- **Cache layer.** `internal/cache/drafts_test.go`:
  schema migration round-trip; `UpsertDraft` idempotency;
  `ListDrafts` returns local-only rows; `DeleteDraft`
  removes; PushDraftOp insert + drainer dispatch with a
  fake JMAP backend.
- **Compose model.** `internal/ui/compose/model_test.go`:
  autosave-tick-debounce semantics (typing within 1s
  reschedules; idle past 1s flushes); 5min push-tick;
  Init writes empty row; Open seeds from cache.
- **App routing.** `internal/ui/app_test.go`:
  Drafts-folder Enter on a draft UID opens
  compose-with-id; `c` allocates a fresh id; close-confirm
  modal renders and routes Y/N/Esc correctly.
- **Round-trip integration.** New `internal/cache/drafts_integration_test.go`:
  open compose → edit → close-with-save → drainer pushes
  via fake backend → reopen via Drafts row → payload
  matches.
- **tmux verify.** Live test against the Fastmail account
  with `$FASTMAIL_API_TOKEN`: compose, type, close,
  reopen, edit, send. Capture at 80×24 and 120×40 per
  ADR-0097.

## Risks

- **Reconstruction fidelity.** `ParseDraftMIME` round-trips
  must be lossless for the fields `compose.Draft` carries
  (To/Cc/Bcc/Subject/Body/From/InReplyTo/References, attachment
  paths). Body is currently raw markdown going to multipart/
  alternative — the reverse lifts the text/plain part as the
  body. Inline attachments (when 9.5 lands) will need richer
  parsing; punted for now.
- **Schema migration on a populated v6 db.** `migrateV7` is a
  pure additive — `CREATE TABLE drafts` + `CREATE INDEX`. No
  data movement, no risk to existing rows.
- **JMAP Drafts mailbox id discovery.** Some accounts have no
  role:drafts mailbox until something is appended. The push
  path resolves the id at first call and caches; if the
  mailbox is absent, the backend returns
  `fmt.Errorf("no Drafts folder")` which routes through the
  conflict matrix as a transient — the user retries after
  configuring a Drafts folder server-side.

## Forward — Pass 9h.6 scope

- IMAP `PushDraft`: APPEND with `\Draft` flag + UID STORE
  `\Deleted` on `prevUID` + UID EXPUNGE, scoped by UIDPLUS.
- Gmail (`GmailQuirks`): route through `[Gmail]/Drafts`.
- Conflict status-bar banner.
- Lift the `IsJMAP()` gate in App and compose.

ADR-0165 will codify the IMAP semantics and the banner UX.

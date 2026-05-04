# Attachments I — backend design (Pass 8.6)

**Date:** 2026-05-03
**Scope:** `internal/mail/`, `internal/cache/`, `internal/mailjmap/`,
`internal/mailimap/`. No UI changes — viewer + indicators land in Pass 8.7,
compose-side attach in Pass 9.5.

## Goal

End-to-end backend support for attachment metadata + lazy byte download
through the cache, mirroring the Cache II body lazy-population pattern
(ADR-0122/0123/0124).

## `mail.Attachment` type and Backend methods

New type in `internal/mail/types.go`:

```go
type Disposition uint8

const (
    DispAttachment Disposition = iota
    DispInline
)

// Attachment carries metadata for a non-body MIME part. PartID is the
// protocol-native identifier — JMAP partId, IMAP section number
// ("2", "2.1") — and is opaque to consumers.
type Attachment struct {
    PartID      string
    Filename    string      // best-effort; "" when neither name nor filename param present
    MIMEType    string      // "type/subtype" lowercased, params dropped
    Size        uint32      // backend-reported decoded size
    ContentID   string      // unwrapped (no "<>"); "" when absent
    Disposition Disposition
}
```

Two new methods on `mail.Backend`:

```go
// Attachments returns metadata for non-body parts of uid. Implementations
// may issue a roundtrip; callers should expect this to block.
Attachments(uid UID) ([]Attachment, error)

// FetchAttachment returns decoded bytes for partID on uid. partID must
// come from a prior Attachments call on the same backend instance.
FetchAttachment(uid UID, partID string) ([]byte, error)
```

### Classification rule (inline vs attachment)

Source of truth is the `Content-Disposition` header (JMAP `disposition`
field, IMAP body-structure dispositional extension). When disposition is
empty or missing:

- `ContentID != ""` → `DispInline`
- otherwise → `DispAttachment`

Top-level `text/plain` and `text/html` parts that compose the displayable
message body are **not** surfaced as attachments — those continue to
flow through `FetchBody`. Attachments enumerated here are strictly the
non-body parts.

### MIME normalization

Stored as `type/subtype`, lowercased, parameters dropped. `name` is
captured in `Filename`; `charset` is irrelevant for binary parts.
Backend-reported `application/octet-stream` is preserved verbatim — no
sniffing in v1.

## Cache schema (v5) + API

New table, added by `migrateV5`:

```sql
CREATE TABLE attachments (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    message      INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    part_id      TEXT    NOT NULL,
    filename     TEXT    NOT NULL DEFAULT '',
    mime_type    TEXT    NOT NULL,
    size         INTEGER NOT NULL,
    content_id   TEXT    NOT NULL DEFAULT '',
    disposition  TEXT    NOT NULL,           -- 'attachment' | 'inline'
    bytes        BLOB,                       -- NULL until first FetchAttachment
    fetched_at   INTEGER,                    -- NULL until first FetchAttachment
    UNIQUE (message, part_id)
);
CREATE INDEX attachments_message ON attachments(message);
```

New API on `*cache.Account`:

```go
// Attachments returns metadata for uid. Cache miss → backend.Attachments
// → store rows with bytes=NULL → return.
func (a *Account) Attachments(ctx context.Context, uid mail.UID) ([]mail.Attachment, error)

// FetchAttachment returns bytes for partID on uid. Cache miss →
// backend.FetchAttachment → store under size backstop → return.
func (a *Account) FetchAttachment(ctx context.Context, uid mail.UID, partID string) ([]byte, error)
```

### Population pattern

- **Metadata** is populated lazily on first `Attachments(uid)`. No
  pre-population during folder sync — too expensive to issue an extra
  per-message roundtrip for messages the user may never open.
- **Bytes** are populated lazily on first `FetchAttachment`,
  write-through with a separate size backstop.

### Size backstop

New config field `[cache] max-attachment-size` (default 2 GB, identical
to body default). Eviction mirrors `bodies.evictBySize`: oldest by
`messages.sent_at`, scoped to `attachments.bytes IS NOT NULL`. Two
distinct budgets so a flood of attachments cannot push out cached
bodies and vice-versa.

## Backend implementations

### JMAP

New file `internal/mailjmap/attachments.go`.

`Attachments(uid)` calls `Email/get` with properties
`["bodyStructure", "attachments"]`. The JMAP `attachments` property
(RFC 8621 §4.1.4) is the server-precomputed list of non-body parts;
each entry is mapped to `mail.Attachment` and the Q1 classification
rule is applied to set `Disposition` (the rule, not the JMAP list's
inclusion criteria, is the source of truth for inline-vs-attachment).

`FetchAttachment(uid, partID)` resolves `(uid, partID) → blobID` via a
per-uid `partBlobIDs map[mail.UID]map[string]string` cached alongside
the existing `b.blobIDs`. Cache miss on the partID map issues a
fresh `Email/get` for the bodyStructure. Bytes come from
`cli.Download(accountID, blobID)`, identical to the body path.

Tests follow the existing `fake_test.go` pattern.

### IMAP

New file `internal/mailimap/attachments.go`.

`Attachments(uid)` issues `UID FETCH uid (BODYSTRUCTURE)` via
`emersion/go-imap` v2 and walks the returned `imap.BodyStructure` tree.
The walk emits a flat list of leaf parts, filtered by the Q1 rule:
top-level `text/plain` and `text/html` body candidates are dropped;
inline images and binary parts are kept.

`FetchAttachment(uid, partID)` issues `UID FETCH uid BODY[<partID>]`
and decodes per the part's transfer encoding using `go-message`.

Tests use the `fake_test.go` IMAP server pattern.

### No new sync hook

Folder sync is unchanged. The `📎 has attachment` indicator (Pass 8.7
work) needs a separate signal: JMAP exposes `hasAttachment` on the
standard Email properties for free; IMAP does not, so the indicator
will degrade for IMAP accounts until the message body is opened. This
is acceptable for v1 and called out in the Pass 8.7 plan; out of
scope for Pass 8.6.

## Test plan

- `internal/mail/types_test.go` — disposition string round-trip, MIME
  normalization helper.
- `internal/cache/attachments_test.go` — schema migration v4→v5,
  metadata lazy population, byte write-through, size backstop
  eviction (mirror existing `bodies_test.go` cases).
- `internal/mailjmap/attachments_test.go` — `Attachments` parses
  bodyStructure correctly; classification rule respects disposition;
  `FetchAttachment` rides existing Download path.
- `internal/mailimap/attachments_test.go` — BODYSTRUCTURE walk, leaf
  filtering, `BODY[<part>]` fetch + transfer-encoding decode.
- Integration: cache + fake backend exercises full lazy → populate →
  hit cycle for both metadata and bytes.

## Out of scope

- UI: viewer attachment list, save-to-disk action, message-list
  indicator. (Pass 8.7.)
- Compose: attaching files when sending. (Pass 9.5.)
- Inline-image rendering in the viewer. (Tracked separately; not v1.)
- Attachment-bytes preflight or partial-content range fetches.
- Sniffing octet-stream MIME types via `http.DetectContentType`.

## Migration notes

- Schema bump v4→v5 is additive (new table, no column changes on
  existing tables). Forward migration only; pre-beta posture
  (ADR-0105) — no rollback path.
- `Backend` interface gains two methods. The mock backend in
  `internal/mail/mock.go` returns `nil, nil` and `nil, nil`
  respectively until Pass 8.7 needs richer fixtures.

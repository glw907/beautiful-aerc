# Attachments I (Pass 8.6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Backend support for parsing and downloading attachments through the cache, mirroring Cache II body lazy-population. Pass 8.7 (viewer) consumes this; no UI changes here.

**Architecture:** New `mail.Attachment` type + two backend methods (`Attachments`, `FetchAttachment`). Cache gains a v5 `attachments` table (metadata + lazy bytes) with a separate size backstop. Both v1 backends (JMAP, IMAP) parse body-structure and download parts on demand.

**Tech Stack:** Go 1.26, `database/sql` + `modernc.org/sqlite` (cache), `git.sr.ht/~rockorager/go-jmap` (JMAP), `github.com/emersion/go-imap/v2` + `go-message` (IMAP).

**Spec:** `docs/superpowers/specs/2026-05-03-attachments-i-design.md`

---

## File Structure

**Create**

- `internal/cache/attachments.go` — `Attachments`, `FetchAttachment`, `lookupAttachments`, `lookupAttachmentBytes`, `storeAttachments`, `storeAttachmentBytes`, `evictAttachmentBytesBySize`
- `internal/cache/attachments_test.go` — schema migration, lazy population, byte write-through, eviction
- `internal/mailjmap/attachments.go` — JMAP `Attachments`, `FetchAttachment`; `partBlobIDs` map
- `internal/mailjmap/attachments_test.go` — bodyStructure parsing, classification, blob download
- `internal/mailimap/attachments.go` — IMAP `Attachments`, `FetchAttachment`; BODYSTRUCTURE walk
- `internal/mailimap/attachments_test.go` — fake-server BODYSTRUCTURE + `BODY[<part>]` cases

**Modify**

- `internal/mail/types.go` — add `Disposition`, `Attachment`
- `internal/mail/backend.go` — add `Attachments`, `FetchAttachment` to interface
- `internal/mail/mock.go` — implement new methods (return zero values)
- `internal/cache/cache_test.go` — `fakeBackend.Attachments`, `FetchAttachment` stubs
- `internal/cache/schema.go` — `migrateV5`, bump `schemaVersion = 5`
- `internal/cache/account.go` — `Config.MaxAttachmentSize`, `Account.maxAttachmentSize`
- `internal/config/cache.go` — `CacheConfig.MaxAttachmentSize`, parse `max-attachment-size`
- `internal/config/cache_test.go` — coverage for new field
- `internal/mailjmap/jmap.go` — `Backend.partBlobIDs map[mail.UID]map[string]string`, init in `New`/`Connect`
- `internal/mailimap/client.go` — extend `imapClient` interface with `FetchBodyStructure`, `FetchBodyPart`
- `internal/mailimap/realclient.go` — implement new client methods
- `internal/mailimap/fake_test.go` — fake implementations of new client methods

---

## Task 1: `mail.Disposition` and `mail.Attachment` types

**Files:**
- Modify: `internal/mail/types.go`
- Test: `internal/mail/types_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/mail/types_test.go`:

```go
// SPDX-License-Identifier: MIT

package mail

import "testing"

func TestDispositionString(t *testing.T) {
	cases := []struct {
		d    Disposition
		want string
	}{
		{DispAttachment, "attachment"},
		{DispInline, "inline"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Errorf("Disposition(%d).String() = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestParseDisposition(t *testing.T) {
	cases := []struct {
		in      string
		want    Disposition
		wantErr bool
	}{
		{"attachment", DispAttachment, false},
		{"inline", DispInline, false},
		{"ATTACHMENT", DispAttachment, false},
		{"", 0, true},
		{"bogus", 0, true},
	}
	for _, c := range cases {
		got, err := ParseDisposition(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("ParseDisposition(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseDisposition(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mail/ -run 'TestDisposition|TestParseDisposition'`
Expected: FAIL with "undefined: Disposition" / "undefined: ParseDisposition" / "undefined: DispAttachment".

- [ ] **Step 3: Add the types**

Append to `internal/mail/types.go`:

```go
// Disposition classifies a MIME part as a user-visible attachment or
// an inline body fragment. Source of truth is the Content-Disposition
// header (JMAP `disposition`, IMAP body-structure dispositional ext);
// when missing, ContentID != "" implies DispInline.
type Disposition uint8

const (
	DispAttachment Disposition = iota
	DispInline
)

// String returns the canonical lowercase token. The string form is
// what the cache stores in attachments.disposition and what backends
// emit on the wire.
func (d Disposition) String() string {
	switch d {
	case DispInline:
		return "inline"
	default:
		return "attachment"
	}
}

// ParseDisposition is the inverse of String. Empty / unknown input
// returns an error so callers can apply their own fallback.
func ParseDisposition(s string) (Disposition, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "attachment":
		return DispAttachment, nil
	case "inline":
		return DispInline, nil
	default:
		return 0, fmt.Errorf("unknown disposition %q", s)
	}
}

// Attachment carries metadata for a non-body MIME part. PartID is
// protocol-native (JMAP partId, IMAP section "2", "2.1") and is
// opaque to consumers — pass it back to FetchAttachment unchanged.
type Attachment struct {
	PartID      string
	Filename    string
	MIMEType    string
	Size        uint32
	ContentID   string
	Disposition Disposition
}
```

Add to the existing imports at the top of `types.go`:

```go
import (
	"fmt"
	"strings"
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mail/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mail/types.go internal/mail/types_test.go
git commit -m "Pass 8.6: mail — add Disposition and Attachment types

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Backend interface — `Attachments` and `FetchAttachment`

**Files:**
- Modify: `internal/mail/backend.go`
- Modify: `internal/mail/mock.go`
- Modify: `internal/cache/cache_test.go` (`fakeBackend`)

- [ ] **Step 1: Write the failing test**

The compile-time interface check is the test here. Add to `internal/mail/mock_test.go` (extend the existing assertion if present, or append):

```go
func TestMockBackendImplementsBackend(t *testing.T) {
	var _ Backend = (*MockBackend)(nil)
}
```

If this test already exists, no edit needed — Step 2 surfaces the missing-method failure either way.

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./...`
Expected: FAIL — `*MockBackend` does not implement `Backend` (missing `Attachments`, `FetchAttachment`).

- [ ] **Step 3: Add interface methods + mock implementations**

In `internal/mail/backend.go`, inside the `Backend` interface block, after `FetchBody`:

```go
	// Attachments returns metadata for non-body parts of uid.
	// Implementations may issue a roundtrip; callers should expect
	// this to block.
	Attachments(uid UID) ([]Attachment, error)

	// FetchAttachment returns decoded bytes for partID on uid.
	// partID must come from a prior Attachments call on the same
	// backend instance.
	FetchAttachment(uid UID, partID string) ([]byte, error)
```

In `internal/mail/mock.go`, append two methods on `*MockBackend`:

```go
// Attachments returns no attachments. Override in tests that need
// richer fixtures.
func (m *MockBackend) Attachments(_ UID) ([]Attachment, error) { return nil, nil }

// FetchAttachment returns nil bytes. Override in tests that need
// richer fixtures.
func (m *MockBackend) FetchAttachment(_ UID, _ string) ([]byte, error) { return nil, nil }
```

In `internal/cache/cache_test.go`, append two methods on `*fakeBackend`:

```go
func (f *fakeBackend) Attachments(_ mail.UID) ([]mail.Attachment, error) { return nil, nil }
func (f *fakeBackend) FetchAttachment(_ mail.UID, _ string) ([]byte, error) {
	return nil, nil
}
```

- [ ] **Step 4: Run build + tests to verify**

Run: `go build ./... && go test ./internal/mail/ ./internal/cache/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mail/backend.go internal/mail/mock.go internal/mail/mock_test.go internal/cache/cache_test.go
git commit -m "Pass 8.6: mail — add Attachments + FetchAttachment to Backend

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Cache schema v5 — `attachments` table

**Files:**
- Modify: `internal/cache/schema.go`
- Test: `internal/cache/cache_test.go` (extend `TestSchemaMigration`)

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/cache_test.go`:

```go
func TestAttachmentsTableShape(t *testing.T) {
	a := openTestAccount(t)
	rows, err := a.db.Query(`PRAGMA table_info(attachments)`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()
	cols := map[string]string{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = ctype
	}
	want := []string{"id", "message", "part_id", "filename", "mime_type", "size", "content_id", "disposition", "bytes", "fetched_at"}
	for _, n := range want {
		if _, ok := cols[n]; !ok {
			t.Errorf("attachments missing column %q", n)
		}
	}
}
```

Add `"database/sql"` to the imports in `cache_test.go` if missing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache/ -run TestAttachmentsTableShape`
Expected: FAIL — every column missing.

- [ ] **Step 3: Add migration v5 + bump schemaVersion**

In `internal/cache/schema.go`:

Change `const schemaVersion = 4` to `const schemaVersion = 5`.

Append `migrateV5` to the `migrations` slice:

```go
var migrations = []migration{
	migrateV1,
	migrateV2,
	migrateV3,
	migrateV4,
	migrateV5, // v4 → v5: attachments table (metadata + lazy bytes)
}
```

Append after `migrateV4`:

```go
// migrateV5 adds the attachments table. Metadata populates lazily on
// first Attachments(uid) call; bytes populate lazily on first
// FetchAttachment(uid, partID), gated by a separate size backstop.
func migrateV5(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE attachments (
            id           INTEGER PRIMARY KEY AUTOINCREMENT,
            message      INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
            part_id      TEXT    NOT NULL,
            filename     TEXT    NOT NULL DEFAULT '',
            mime_type    TEXT    NOT NULL,
            size         INTEGER NOT NULL,
            content_id   TEXT    NOT NULL DEFAULT '',
            disposition  TEXT    NOT NULL,
            bytes        BLOB,
            fetched_at   INTEGER,
            UNIQUE (message, part_id)
        )`,
		`CREATE INDEX attachments_message ON attachments(message)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v5: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/cache/`
Expected: PASS, including the existing `TestSchemaMigration` (now expecting version 5).

- [ ] **Step 5: Commit**

```bash
git add internal/cache/schema.go internal/cache/cache_test.go
git commit -m "Pass 8.6: cache — schema v5 attachments table

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Config — `max-attachment-size`

**Files:**
- Modify: `internal/config/cache.go`
- Modify: `internal/config/cache_test.go`

- [ ] **Step 1: Write the failing test**

Append a case to the table inside `TestLoadCache` in `internal/config/cache_test.go`:

```go
		{
			name: "explicit max-attachment-size",
			toml: `[cache]` + "\n" +
				`max-size = "1GB"` + "\n" +
				`max-attachment-size = "500MB"`,
			want: CacheConfig{
				MaxSize:           1024 * 1024 * 1024,
				MaxAttachmentSize: 500 * 1024 * 1024,
			},
		},
		{
			name: "default attachment cap when section absent",
			toml: ``,
			want: CacheConfig{
				MaxSize:           2 * 1024 * 1024 * 1024,
				MaxAttachmentSize: 2 * 1024 * 1024 * 1024,
			},
		},
```

Update existing cases that compare full `CacheConfig` literals to include `MaxAttachmentSize: 2 * 1024 * 1024 * 1024` for default-equivalent rows. (Read the file before editing — only update rows that previously asserted the bare default.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadCache`
Expected: FAIL — `unknown field MaxAttachmentSize`.

- [ ] **Step 3: Add the field and parsing**

In `internal/config/cache.go`:

```go
type CacheConfig struct {
	// MaxSize is the body-cache size cap in bytes. 0 disables.
	MaxSize int64
	// MaxAttachmentSize is the attachment-bytes-cache size cap in
	// bytes. 0 disables. Tracked separately from MaxSize so a flood
	// of attachments cannot push out cached bodies and vice-versa.
	MaxAttachmentSize int64
}

func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize:           2 * 1024 * 1024 * 1024,
		MaxAttachmentSize: 2 * 1024 * 1024 * 1024,
	}
}

type rawCache struct {
	MaxSize           string `toml:"max-size"`
	MaxAttachmentSize string `toml:"max-attachment-size"`
}
```

Inside `LoadCache`, after the existing `if raw.Cache.MaxSize != ""` block:

```go
	if raw.Cache.MaxAttachmentSize != "" {
		n, err := parseSize(raw.Cache.MaxAttachmentSize)
		if err != nil {
			return CacheConfig{}, fmt.Errorf("cache.max-attachment-size: %w", err)
		}
		out.MaxAttachmentSize = n
	}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/cache.go internal/config/cache_test.go
git commit -m "Pass 8.6: config — max-attachment-size

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Cache — thread `MaxAttachmentSize` into `Account`

**Files:**
- Modify: `internal/cache/account.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/cache_test.go`:

```go
func TestOpenThreadsAttachmentMax(t *testing.T) {
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}
	ct := &fakeChangeTracker{}
	a, err := Open("Test", be, ct, t.TempDir(), Config{MaxSize: 100, MaxAttachmentSize: 200})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	if a.maxSize != 100 {
		t.Errorf("maxSize = %d, want 100", a.maxSize)
	}
	if a.maxAttachmentSize != 200 {
		t.Errorf("maxAttachmentSize = %d, want 200", a.maxAttachmentSize)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache/ -run TestOpenThreadsAttachmentMax`
Expected: FAIL — `unknown field MaxAttachmentSize` / `a.maxAttachmentSize undefined`.

- [ ] **Step 3: Extend `Config` and `Account`**

In `internal/cache/account.go`:

Update `Config`:

```go
type Config struct {
	// MaxSize is the body-cache size cap in bytes. 0 disables.
	MaxSize int64
	// MaxAttachmentSize is the attachment-bytes-cache size cap in
	// bytes. 0 disables. Tracked separately from MaxSize.
	MaxAttachmentSize int64
}
```

Add field to `Account` (after `maxSize`):

```go
	// maxAttachmentSize is the attachment-bytes-cache size cap. 0 disables.
	maxAttachmentSize int64
```

Inside `Open`, when constructing the `Account` struct literal, add after `maxSize: cfg.MaxSize,`:

```go
		maxAttachmentSize: cfg.MaxAttachmentSize,
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/cache/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/account.go internal/cache/cache_test.go
git commit -m "Pass 8.6: cache — thread MaxAttachmentSize into Account

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Cache — metadata lazy population (`Attachments`)

**Files:**
- Create: `internal/cache/attachments.go`
- Create: `internal/cache/attachments_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cache/attachments_test.go`:

```go
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// attachBackend extends fakeBackend with Attachments / FetchAttachment fixtures.
type attachBackend struct {
	fakeBackend
	atts          map[mail.UID][]mail.Attachment
	parts         map[string][]byte // key: uid + "::" + partID
	attachCalls   int
	fetchCalls    int
	fetchErr      error
}

func (b *attachBackend) Attachments(uid mail.UID) ([]mail.Attachment, error) {
	b.attachCalls++
	return b.atts[uid], nil
}
func (b *attachBackend) FetchAttachment(uid mail.UID, partID string) ([]byte, error) {
	b.fetchCalls++
	if b.fetchErr != nil {
		return nil, b.fetchErr
	}
	return b.parts[string(uid)+"::"+partID], nil
}

func openAttachAccount(t *testing.T, be *attachBackend) *Account {
	t.Helper()
	be.fakeBackend.folders = []mail.Folder{{Name: "INBOX", Role: "inbox"}}
	ct := &fakeChangeTracker{}
	a, err := Open("Test", be, ct, t.TempDir(), Config{MaxAttachmentSize: 1 << 30})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	return a
}

func TestAttachments_LazyPopulate(t *testing.T) {
	be := &attachBackend{
		atts: map[mail.UID][]mail.Attachment{
			"u1": {
				{PartID: "2", Filename: "report.pdf", MIMEType: "application/pdf", Size: 1234, Disposition: mail.DispAttachment},
				{PartID: "3", Filename: "logo.png", MIMEType: "image/png", Size: 999, ContentID: "logo@x", Disposition: mail.DispInline},
			},
		},
	}
	a := openAttachAccount(t, be)
	seedMessage(t, a, "u1", time.Now())

	got, err := a.Attachments(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if !reflect.DeepEqual(got, be.atts["u1"]) {
		t.Fatalf("first call: got %+v want %+v", got, be.atts["u1"])
	}
	if be.attachCalls != 1 {
		t.Fatalf("attachCalls = %d, want 1", be.attachCalls)
	}

	got2, err := a.Attachments(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Attachments (2nd): %v", err)
	}
	if !reflect.DeepEqual(got2, got) {
		t.Errorf("second call differs: %+v vs %+v", got2, got)
	}
	if be.attachCalls != 1 {
		t.Errorf("attachCalls = %d after 2nd call, want 1 (cache hit)", be.attachCalls)
	}
}

func TestAttachments_EmptyMessage(t *testing.T) {
	be := &attachBackend{atts: map[mail.UID][]mail.Attachment{"u1": nil}}
	a := openAttachAccount(t, be)
	seedMessage(t, a, "u1", time.Now())

	got, err := a.Attachments(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
	// Second call still hits backend because we cannot distinguish
	// "populated, zero parts" from "not yet populated" without a marker.
	// Document this in the implementation; test asserts current behavior.
	_, _ = a.Attachments(context.Background(), "u1")
	if be.attachCalls < 1 {
		t.Errorf("attachCalls = %d, want >= 1", be.attachCalls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache/ -run TestAttachments_`
Expected: FAIL — `a.Attachments undefined`.

- [ ] **Step 3: Implement `Attachments`**

Create `internal/cache/attachments.go`:

```go
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// Attachments returns metadata for non-body parts of uid. On cache
// miss the backend is consulted and the rows persisted. On cache
// hit no backend roundtrip occurs.
//
// A zero-length result is not cached: empty cannot be distinguished
// from "not yet populated" without a marker, and the cost of an
// occasional re-fetch on truly attachment-free messages is lower
// than a schema column for the marker.
func (a *Account) Attachments(ctx context.Context, uid mail.UID) ([]mail.Attachment, error) {
	rows, err := a.lookupAttachments(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("attachments %s: lookup: %w", uid, err)
	}
	if len(rows) > 0 {
		return rows, nil
	}
	if a.Backend == nil {
		return nil, errors.New("cache: no backend")
	}
	atts, err := a.Backend.Attachments(uid)
	if err != nil {
		return nil, err
	}
	if len(atts) == 0 {
		return nil, nil
	}
	if err := a.storeAttachments(ctx, uid, atts); err != nil {
		// Store failure is non-fatal: caller still has valid metadata.
		_ = err
	}
	return atts, nil
}

// lookupAttachments returns cached metadata rows for uid, ordered by
// id. Empty slice on miss; never returns nil error with non-nil rows
// on a true miss.
func (a *Account) lookupAttachments(ctx context.Context, uid mail.UID) ([]mail.Attachment, error) {
	const q = `
        SELECT a.part_id, a.filename, a.mime_type, a.size, a.content_id, a.disposition
        FROM attachments a
        JOIN messages m ON m.id = a.message
        WHERE m.protocol_id = ?
        ORDER BY a.id ASC`
	rs, err := a.db.QueryContext(ctx, q, string(uid))
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []mail.Attachment
	for rs.Next() {
		var (
			att   mail.Attachment
			disp  string
			size  int64
		)
		if err := rs.Scan(&att.PartID, &att.Filename, &att.MIMEType, &size, &att.ContentID, &disp); err != nil {
			return nil, err
		}
		att.Size = uint32(size)
		d, err := mail.ParseDisposition(disp)
		if err != nil {
			return nil, fmt.Errorf("attachments %s: invalid disposition %q in row", uid, disp)
		}
		att.Disposition = d
		out = append(out, att)
	}
	return out, rs.Err()
}

// storeAttachments writes metadata rows for uid. Caller has already
// determined the cache was empty; this is the populate path. Errors
// if uid has no row in messages.
func (a *Account) storeAttachments(ctx context.Context, uid mail.UID, atts []mail.Attachment) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		var msgID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM messages WHERE protocol_id = ?`, string(uid)).Scan(&msgID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store attachments %s: unknown message uid", uid)
		}
		if err != nil {
			return fmt.Errorf("store attachments %s: resolve message: %w", uid, err)
		}
		for _, att := range atts {
			_, err := tx.ExecContext(ctx, `
                INSERT INTO attachments
                  (message, part_id, filename, mime_type, size, content_id, disposition)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT (message, part_id) DO UPDATE SET
                  filename    = excluded.filename,
                  mime_type   = excluded.mime_type,
                  size        = excluded.size,
                  content_id  = excluded.content_id,
                  disposition = excluded.disposition`,
				msgID, att.PartID, att.Filename, att.MIMEType, int64(att.Size),
				att.ContentID, att.Disposition.String())
			if err != nil {
				return fmt.Errorf("store attachments %s part %s: %w", uid, att.PartID, err)
			}
		}
		return nil
	})
}

// time import retained for FetchAttachment in Task 7.
var _ = time.Now
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/cache/ -run TestAttachments_`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/attachments.go internal/cache/attachments_test.go
git commit -m "Pass 8.6: cache — Attachments metadata lazy population

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Cache — byte lazy population (`FetchAttachment`) + size backstop

**Files:**
- Modify: `internal/cache/attachments.go`
- Modify: `internal/cache/attachments_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cache/attachments_test.go`:

```go
func TestFetchAttachment_LazyPopulate(t *testing.T) {
	be := &attachBackend{
		atts: map[mail.UID][]mail.Attachment{
			"u1": {{PartID: "2", Filename: "r.pdf", MIMEType: "application/pdf", Size: 4, Disposition: mail.DispAttachment}},
		},
		parts: map[string][]byte{"u1::2": []byte("PDF!")},
	}
	a := openAttachAccount(t, be)
	seedMessage(t, a, "u1", time.Now())
	if _, err := a.Attachments(context.Background(), "u1"); err != nil {
		t.Fatalf("populate metadata: %v", err)
	}

	got, err := a.FetchAttachment(context.Background(), "u1", "2")
	if err != nil {
		t.Fatalf("FetchAttachment: %v", err)
	}
	if string(got) != "PDF!" {
		t.Errorf("got %q, want %q", got, "PDF!")
	}
	if be.fetchCalls != 1 {
		t.Fatalf("fetchCalls = %d, want 1", be.fetchCalls)
	}

	// Second call: cache hit, no backend roundtrip.
	got2, err := a.FetchAttachment(context.Background(), "u1", "2")
	if err != nil {
		t.Fatalf("FetchAttachment 2: %v", err)
	}
	if string(got2) != "PDF!" {
		t.Errorf("got2 %q, want %q", got2, "PDF!")
	}
	if be.fetchCalls != 1 {
		t.Errorf("fetchCalls = %d after 2nd call, want 1", be.fetchCalls)
	}
}

func TestFetchAttachment_EvictBySize(t *testing.T) {
	be := &attachBackend{
		atts: map[mail.UID][]mail.Attachment{
			"old":   {{PartID: "2", MIMEType: "application/octet-stream", Size: 100, Disposition: mail.DispAttachment}},
			"newer": {{PartID: "2", MIMEType: "application/octet-stream", Size: 100, Disposition: mail.DispAttachment}},
		},
		parts: map[string][]byte{
			"old::2":   make([]byte, 100),
			"newer::2": make([]byte, 100),
		},
	}
	be.fakeBackend.folders = []mail.Folder{{Name: "INBOX", Role: "inbox"}}
	ct := &fakeChangeTracker{}
	a, err := Open("Test", be, ct, t.TempDir(), Config{MaxAttachmentSize: 150})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.SyncFolders(context.Background()); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	seedMessage(t, a, "old", time.Now().Add(-2*time.Hour))
	seedMessage(t, a, "newer", time.Now())
	if _, err := a.Attachments(context.Background(), "old"); err != nil {
		t.Fatalf("metadata old: %v", err)
	}
	if _, err := a.Attachments(context.Background(), "newer"); err != nil {
		t.Fatalf("metadata newer: %v", err)
	}

	if _, err := a.FetchAttachment(context.Background(), "old", "2"); err != nil {
		t.Fatalf("fetch old: %v", err)
	}
	if _, err := a.FetchAttachment(context.Background(), "newer", "2"); err != nil {
		t.Fatalf("fetch newer: %v", err)
	}

	// After both fetches, total = 200 > cap 150. Older row (by sent_at)
	// must have been evicted before the second insert; total should be 100.
	var total int64
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(length(bytes)), 0) FROM attachments WHERE bytes IS NOT NULL`).Scan(&total); err != nil {
		t.Fatalf("sum: %v", err)
	}
	if total != 100 {
		t.Errorf("total cached bytes = %d, want 100", total)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache/ -run TestFetchAttachment_`
Expected: FAIL — `a.FetchAttachment undefined`.

- [ ] **Step 3: Implement `FetchAttachment` + helpers**

Replace the trailing `var _ = time.Now` line in `internal/cache/attachments.go` with:

```go
// FetchAttachment returns bytes for partID on uid. Cache miss →
// backend → store under the attachment-size backstop → return.
// Storage failure is non-fatal: returned bytes are still valid.
func (a *Account) FetchAttachment(ctx context.Context, uid mail.UID, partID string) ([]byte, error) {
	if buf, ok, err := a.lookupAttachmentBytes(ctx, uid, partID); err != nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: lookup: %w", uid, partID, err)
	} else if ok {
		return buf, nil
	}
	if a.Backend == nil {
		return nil, errors.New("cache: no backend")
	}
	body, err := a.Backend.FetchAttachment(uid, partID)
	if err != nil {
		return nil, err
	}
	if storeErr := a.storeAttachmentBytes(ctx, uid, partID, body); storeErr != nil {
		_ = storeErr
	}
	return body, nil
}

// lookupAttachmentBytes reads cached bytes for (uid, partID). Misses
// when the row is absent OR the row exists with bytes IS NULL.
func (a *Account) lookupAttachmentBytes(ctx context.Context, uid mail.UID, partID string) ([]byte, bool, error) {
	const q = `
        SELECT a.bytes
        FROM attachments a
        JOIN messages m ON m.id = a.message
        WHERE m.protocol_id = ? AND a.part_id = ?`
	var buf []byte
	err := a.db.QueryRowContext(ctx, q, string(uid), partID).Scan(&buf)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if buf == nil {
		return nil, false, nil
	}
	return buf, true, nil
}

// storeAttachmentBytes writes bytes onto the (uid, partID) row,
// evicting older rows by messages.sent_at when the attachment-size
// budget would be exceeded. Errors if the row is absent — caller
// must populate metadata via Attachments() first.
func (a *Account) storeAttachmentBytes(ctx context.Context, uid mail.UID, partID string, body []byte) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		newSize := int64(len(body))
		if a.maxAttachmentSize > 0 {
			var total int64
			if err := tx.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(length(bytes)), 0) FROM attachments WHERE bytes IS NOT NULL`).Scan(&total); err != nil {
				return fmt.Errorf("store attachment %s/%s: sum: %w", uid, partID, err)
			}
			if total+newSize > a.maxAttachmentSize {
				target := a.maxAttachmentSize - newSize
				if target < 0 {
					target = 0
				}
				if _, _, err := a.evictAttachmentBytesBySize(ctx, tx, total, target); err != nil {
					return err
				}
			}
		}
		res, err := tx.ExecContext(ctx, `
            UPDATE attachments
               SET bytes      = ?,
                   fetched_at = ?
             WHERE part_id = ?
               AND message  = (SELECT id FROM messages WHERE protocol_id = ?)`,
			body, time.Now().UnixNano(), partID, string(uid))
		if err != nil {
			return fmt.Errorf("store attachment %s/%s: update: %w", uid, partID, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("store attachment %s/%s: unknown row (call Attachments first)", uid, partID)
		}
		return nil
	})
}

// evictAttachmentBytesBySize removes oldest-by-sent-date attachment
// bytes (clears bytes + fetched_at, keeps the metadata row) until
// total cached bytes are at or below target.
func (a *Account) evictAttachmentBytesBySize(ctx context.Context, tx *sql.Tx, total, target int64) (rows int, freed int64, err error) {
	const batchSize = 32
	const pickQ = `
        SELECT a.id, length(a.bytes)
        FROM attachments a
        JOIN messages m ON m.id = a.message
        WHERE a.bytes IS NOT NULL
        ORDER BY m.sent_at ASC NULLS LAST
        LIMIT ?`
	for total > target {
		rs, err := tx.QueryContext(ctx, pickQ, batchSize)
		if err != nil {
			return rows, freed, fmt.Errorf("evict attach: pick batch: %w", err)
		}
		var ids []int64
		var batchFreed int64
		remaining := total - target
		for rs.Next() {
			var id, sz int64
			if err := rs.Scan(&id, &sz); err != nil {
				rs.Close()
				return rows, freed, fmt.Errorf("evict attach: scan: %w", err)
			}
			ids = append(ids, id)
			batchFreed += sz
			remaining -= sz
			if remaining <= 0 {
				break
			}
		}
		rs.Close()
		if len(ids) == 0 {
			return rows, freed, nil
		}
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE attachments SET bytes = NULL, fetched_at = NULL WHERE id IN (`+sqlPlaceholders(len(ids))+`)`,
			args...); err != nil {
			return rows, freed, fmt.Errorf("evict attach: clear: %w", err)
		}
		rows += len(ids)
		freed += batchFreed
		total -= batchFreed
	}
	return rows, freed, nil
}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/cache/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/attachments.go internal/cache/attachments_test.go
git commit -m "Pass 8.6: cache — FetchAttachment lazy bytes + size backstop

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: JMAP — `Attachments` (metadata)

**Files:**
- Modify: `internal/mailjmap/jmap.go` (add `partBlobIDs` map; init in `New` and reset in `Connect`)
- Create: `internal/mailjmap/attachments.go`
- Create: `internal/mailjmap/attachments_test.go`

- [ ] **Step 1: Write the failing test**

Inspect `internal/mailjmap/jmap_test.go` and `fake_test.go` first to see how the fake JMAP server is constructed. Create `internal/mailjmap/attachments_test.go` mirroring the existing fixture pattern. The test should:

1. Set up a fake JMAP server that, on `Email/get` with properties including `bodyStructure`/`attachments`, returns a fixture email containing one PDF attachment (`disposition="attachment"`) and one inline image (`cid="logo@x"`, no explicit disposition).
2. Call `b.Attachments("uid-1")`.
3. Assert the result contains exactly two `mail.Attachment` rows with the expected `Filename`, lowercased `MIMEType`, `Size`, `ContentID`, `Disposition` (DispAttachment for PDF, DispInline for image because `cid != ""`).
4. Assert the in-memory `partBlobIDs` map now contains `partBlobIDs["uid-1"]["<partId-pdf>"] = "<blob-pdf>"` and the inline image's mapping.

(Use the exact server-fake style from `internal/mailjmap/jmap_test.go`. If existing tests use a helper like `newTestServer`, reuse it.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mailjmap/ -run TestAttachments`
Expected: FAIL — `b.Attachments undefined`.

- [ ] **Step 3: Implement**

In `internal/mailjmap/jmap.go`, add the field to `Backend` (read the existing struct first; place near `blobIDs`):

```go
	// partBlobIDs caches per-uid (partID → blobID) maps so
	// FetchAttachment doesn't require an extra Email/get when the
	// caller has already invoked Attachments.
	partBlobIDs map[mail.UID]map[string]string
```

Initialize in `New` alongside `blobIDs`:

```go
		partBlobIDs: make(map[mail.UID]map[string]string),
```

Reset in `Connect` where `blobIDs` is reset:

```go
	b.partBlobIDs = make(map[mail.UID]map[string]string)
```

Create `internal/mailjmap/attachments.go`:

```go
// SPDX-License-Identifier: MIT

package mailjmap

import (
	"fmt"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"

	"github.com/glw907/poplar/internal/mail"
)

// attachmentProperties is the Email/get property set for attachment
// metadata. bodyStructure carries every part with disposition + cid;
// attachments is the server-precomputed non-body subset, useful as a
// hint but not authoritative for inline-vs-attachment classification.
var attachmentProperties = []string{"id", "bodyStructure", "attachments"}

// Attachments satisfies mail.Backend. Issues one Email/get with
// bodyStructure + attachments, walks the part tree, and returns the
// non-body parts. Side effect: populates b.partBlobIDs[uid].
func (b *Backend) Attachments(uid mail.UID) ([]mail.Attachment, error) {
	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&email.Get{
		Account:    accountID,
		IDs:        []jmap.ID{jmap.ID(uid)},
		Properties: attachmentProperties,
	})
	resp, err := b.do(req)
	if err != nil {
		return nil, fmt.Errorf("attachments %s: %w", uid, err)
	}

	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*email.GetResponse)
		if !ok || len(gr.List) == 0 {
			continue
		}
		e := gr.List[0]
		atts, partMap := walkBodyStructure(e.BodyStructure)
		b.mu.Lock()
		b.partBlobIDs[uid] = partMap
		b.mu.Unlock()
		return atts, nil
	}
	return nil, fmt.Errorf("attachments %s: no Email/get response", uid)
}

// walkBodyStructure flattens the JMAP body structure into a list of
// non-body parts, applying the spec §Q1 classification rule. The
// returned map carries partID→blobID for every walked leaf so
// FetchAttachment can resolve without a second roundtrip.
func walkBodyStructure(bp *email.BodyPart) ([]mail.Attachment, map[string]string) {
	if bp == nil {
		return nil, map[string]string{}
	}
	var atts []mail.Attachment
	parts := map[string]string{}
	var walk func(p *email.BodyPart, isTopLevelBody bool)
	walk = func(p *email.BodyPart, isTopLevelBody bool) {
		if p == nil {
			return
		}
		if len(p.SubParts) > 0 {
			for _, sp := range p.SubParts {
				walk(sp, false)
			}
			return
		}
		mt := strings.ToLower(p.Type)
		// Skip the displayable body candidates at the top level.
		if isTopLevelBody && (mt == "text/plain" || mt == "text/html") {
			return
		}
		parts[p.PartID] = string(p.BlobID)
		atts = append(atts, mail.Attachment{
			PartID:      p.PartID,
			Filename:    pickFilename(p),
			MIMEType:    mt,
			Size:        uint32(p.Size),
			ContentID:   strings.Trim(p.CID, "<>"),
			Disposition: classifyDisposition(p),
		})
	}
	// At the top, the textual body (text/plain / text/html) and its
	// alternatives are usually the first leaf or the first child of a
	// multipart/alternative; treat root + immediate children as
	// "top-level body" candidates for the purposes of the skip rule.
	if len(bp.SubParts) > 0 {
		for _, sp := range bp.SubParts {
			walk(sp, true)
		}
	} else {
		walk(bp, true)
	}
	return atts, parts
}

// classifyDisposition implements Q1: trust Content-Disposition; when
// missing, ContentID != "" → inline, else attachment.
func classifyDisposition(p *email.BodyPart) mail.Disposition {
	if d, err := mail.ParseDisposition(p.Disposition); err == nil {
		return d
	}
	if strings.TrimSpace(p.CID) != "" {
		return mail.DispInline
	}
	return mail.DispAttachment
}

// pickFilename prefers the disposition filename, falls back to the
// Content-Type name. Empty when neither is set.
func pickFilename(p *email.BodyPart) string {
	if p.Name != "" {
		return p.Name
	}
	return ""
}
```

> **Note for implementer:** verify the exact field names on `*email.BodyPart` in the vendored `git.sr.ht/~rockorager/go-jmap/mail/email` package. Likely-correct names: `PartID`, `BlobID`, `Type`, `Size`, `Disposition`, `CID`, `Name`, `SubParts`. If a field is named differently (e.g. `Cid`, `BodyParts`), adjust the references; do not invent fields. The shape of the walk does not change.

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/mailjmap/ -run TestAttachments`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mailjmap/jmap.go internal/mailjmap/attachments.go internal/mailjmap/attachments_test.go
git commit -m "Pass 8.6: mailjmap — Attachments via Email/get bodyStructure

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: JMAP — `FetchAttachment` (bytes via Download)

**Files:**
- Modify: `internal/mailjmap/attachments.go`
- Modify: `internal/mailjmap/attachments_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/mailjmap/attachments_test.go` a test that:

1. Calls `b.Attachments("uid-1")` first (to populate `partBlobIDs`).
2. Calls `b.FetchAttachment("uid-1", "<partID>")`.
3. Asserts the returned bytes equal the fixture blob bytes.

A second sub-test calls `b.FetchAttachment` directly without prior `Attachments`, and asserts that the implementation issues an `Email/get` to populate `partBlobIDs` before downloading (assert via the fake server's request log).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mailjmap/ -run TestFetchAttachment`
Expected: FAIL — `b.FetchAttachment undefined`.

- [ ] **Step 3: Implement**

Append to `internal/mailjmap/attachments.go`:

```go
// FetchAttachment satisfies mail.Backend. Resolves (uid, partID) to
// a blobID via the cached partBlobIDs map (issuing one Email/get
// when the cache is cold), then downloads via cli.Download.
func (b *Backend) FetchAttachment(uid mail.UID, partID string) ([]byte, error) {
	b.mu.Lock()
	parts := b.partBlobIDs[uid]
	dl := b.downloadBlob
	b.mu.Unlock()

	if parts == nil {
		// Cold map: populate via Attachments. Discard the metadata
		// — the caller already has it, and the side-effect of
		// populating partBlobIDs is what we need.
		if _, err := b.Attachments(uid); err != nil {
			return nil, fmt.Errorf("fetch attachment %s/%s: prime: %w", uid, partID, err)
		}
		b.mu.Lock()
		parts = b.partBlobIDs[uid]
		b.mu.Unlock()
	}
	blobID, ok := parts[partID]
	if !ok {
		return nil, fmt.Errorf("fetch attachment %s/%s: unknown partID", uid, partID)
	}
	if dl == nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: not connected", uid, partID)
	}
	body, err := dl(blobID)
	if err != nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: download: %w", uid, partID, err)
	}
	return body, nil
}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/mailjmap/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mailjmap/attachments.go internal/mailjmap/attachments_test.go
git commit -m "Pass 8.6: mailjmap — FetchAttachment via cli.Download

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: IMAP client interface — BODYSTRUCTURE + part fetch

**Files:**
- Modify: `internal/mailimap/client.go`
- Modify: `internal/mailimap/realclient.go`
- Modify: `internal/mailimap/fake_test.go`

- [ ] **Step 1: Write the failing test**

The test for this is the IMAP `Attachments` test in Task 11; this task is a pure interface widening. Skip directly to step 3.

- [ ] **Step 2: (skipped)**

- [ ] **Step 3: Extend the `imapClient` interface and implementations**

In `internal/mailimap/client.go`, append two methods to the `imapClient` interface (before the closing brace):

```go
	// FetchBodyStructure issues UID FETCH (BODYSTRUCTURE) for one UID
	// and returns the parsed structure. Caller walks the tree.
	FetchBodyStructure(uid mail.UID) (BodyStructure, error)

	// FetchBodyPart returns the decoded bytes of one MIME part.
	// section is an IMAP section identifier ("2", "2.1", etc.).
	FetchBodyPart(uid mail.UID, section string) ([]byte, error)
```

Add the `BodyStructure` type to the same file (after the existing `listEntry`):

```go
// BodyStructure is the protocol-agnostic shape of a parsed IMAP
// BODYSTRUCTURE response. Only the fields mailimap needs are
// retained; the underlying go-imap type carries more.
type BodyStructure struct {
	Section     string         // "" for root, "1", "2", "2.1" for parts
	MIMEType    string         // "text/plain" lowercased
	Filename    string
	SizeBytes   uint32
	ContentID   string
	Disposition string         // "attachment" | "inline" | "" if unset
	Children    []BodyStructure
}
```

In `internal/mailimap/realclient.go`, implement the two methods on the real adapter. The exact mapping uses `imapclient.Client.Fetch(...)` with `imap.FetchOptions{BodyStructure: ...}` and `imap.FetchOptions{BodySection: []*imap.FetchItemBodySection{{Specifier: imap.PartSpecifierNone, Part: parsePartPath(section)}}}`. Walk the resulting `imap.BodyStructure` (interface implemented by `*imap.BodyStructureMultiPart` / `*imap.BodyStructureSinglePart`) and populate `BodyStructure`.

> **Note for implementer:** the v2 emersion API surfaces body structure as an interface (`imap.BodyStructure`). Use a type switch and recurse on `*imap.BodyStructureMultiPart`. For each `*imap.BodyStructureSinglePart`, read `Type`, `Subtype`, `Size`, `Disp` (an `*imap.BodyStructureDisposition`), `Params["filename"]` or `Params["name"]`, and `ID` (Content-ID). Section paths are dot-joined indices ("2.1"). Verify field names against the vendored `github.com/emersion/go-imap/v2` package before relying on them.

In `internal/mailimap/fake_test.go`, add fake implementations on `*fakeClient` that return the values the test fixtures need. Mirror the shape of existing `fakeClient.Fetch`/`FetchBody` methods.

- [ ] **Step 4: Run tests to verify**

Run: `go build ./... && go test ./internal/mailimap/`
Expected: PASS (the existing tests continue to pass; new methods are present but unused until Task 11).

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/client.go internal/mailimap/realclient.go internal/mailimap/fake_test.go
git commit -m "Pass 8.6: mailimap — extend imapClient with BODYSTRUCTURE + part fetch

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: IMAP — `Attachments` (BODYSTRUCTURE walk)

**Files:**
- Create: `internal/mailimap/attachments.go`
- Create: `internal/mailimap/attachments_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mailimap/attachments_test.go` modeled on `internal/mailimap/messages_test.go`. The test sets `fakeClient.bodyStructure["7"]` to a fixture tree containing:

- root multipart/mixed
  - section "1": text/plain (body — must be skipped)
  - section "2": application/pdf with `Disp.Type="attachment"`, `Filename="r.pdf"`, `Size=4096`
  - section "3": image/png with `ID="<logo@x>"`, no explicit disposition

Then `b.Attachments("7")` should return two `mail.Attachment` rows: PDF as DispAttachment and image as DispInline (Q1 fallback via ContentID).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mailimap/ -run TestAttachments`
Expected: FAIL — `b.Attachments undefined`.

- [ ] **Step 3: Implement**

Create `internal/mailimap/attachments.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"fmt"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// Attachments satisfies mail.Backend. Issues UID FETCH BODYSTRUCTURE
// for uid, walks the part tree, and returns the non-body parts.
// Top-level text/plain and text/html parts are dropped (they are
// the displayable body, not attachments).
func (b *Backend) Attachments(uid mail.UID) ([]mail.Attachment, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	bs, err := cmd.FetchBodyStructure(uid)
	if err != nil {
		return nil, fmt.Errorf("attachments %s: %w", uid, err)
	}
	return walkBodyStructure(bs, true), nil
}

// walkBodyStructure flattens bs to leaves, skipping top-level
// text/plain and text/html bodies. Inline images and binary leaves
// are kept; the Q1 classification rule sets Disposition.
func walkBodyStructure(bs BodyStructure, isTopLevel bool) []mail.Attachment {
	if len(bs.Children) > 0 {
		var out []mail.Attachment
		for _, c := range bs.Children {
			out = append(out, walkBodyStructure(c, isTopLevel)...)
		}
		return out
	}
	mt := strings.ToLower(bs.MIMEType)
	if isTopLevel && (mt == "text/plain" || mt == "text/html") {
		return nil
	}
	return []mail.Attachment{{
		PartID:      bs.Section,
		Filename:    bs.Filename,
		MIMEType:    mt,
		Size:        bs.SizeBytes,
		ContentID:   strings.Trim(bs.ContentID, "<>"),
		Disposition: classifyDisposition(bs),
	}}
}

func classifyDisposition(bs BodyStructure) mail.Disposition {
	if d, err := mail.ParseDisposition(bs.Disposition); err == nil {
		return d
	}
	if strings.TrimSpace(bs.ContentID) != "" {
		return mail.DispInline
	}
	return mail.DispAttachment
}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/mailimap/ -run TestAttachments`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/attachments.go internal/mailimap/attachments_test.go
git commit -m "Pass 8.6: mailimap — Attachments via BODYSTRUCTURE walk

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: IMAP — `FetchAttachment`

**Files:**
- Modify: `internal/mailimap/attachments.go`
- Modify: `internal/mailimap/attachments_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/mailimap/attachments_test.go`: with `fakeClient.bodyParts["7::2"]` set to `[]byte("PDF bytes")`, `b.FetchAttachment("7", "2")` returns those bytes.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mailimap/ -run TestFetchAttachment`
Expected: FAIL — `b.FetchAttachment undefined`.

- [ ] **Step 3: Implement**

Append to `internal/mailimap/attachments.go`:

```go
// FetchAttachment satisfies mail.Backend. Issues UID FETCH BODY[<part>]
// and returns the decoded bytes. The transfer encoding (base64,
// quoted-printable) is decoded by the go-imap client adapter; this
// returns raw decoded bytes ready to write to disk.
func (b *Backend) FetchAttachment(uid mail.UID, partID string) ([]byte, error) {
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	body, err := cmd.FetchBodyPart(uid, partID)
	if err != nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: %w", uid, partID, err)
	}
	return body, nil
}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/mailimap/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/attachments.go internal/mailimap/attachments_test.go
git commit -m "Pass 8.6: mailimap — FetchAttachment via BODY[<part>]

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 13: Pass-end checklist

- [ ] **Step 1: Run /simplify**

Invoke the `simplify` skill against this pass's diff. Aggregate findings, apply genuine wins, commit any resulting changes with message `Pass 8.6: /simplify cleanup`.

- [ ] **Step 2: Run `make check`**

```bash
make check
```

Expected: vet + full test suite green.

- [ ] **Step 3: Write ADRs**

For each design decision in this pass, write an ADR in `docs/poplar/decisions/` using the next chronological number. Minimum:

- `0135-attachment-cache-shape.md` — attachments live in a separate v5 table; metadata + bytes share the row; bytes column nullable for lazy population. Reasons: bodies and attachments have different size profiles + budgets; conflating into a parts table would force every body lookup to disambiguate.
- `0136-attachment-classification.md` — Q1 settled: Content-Disposition primary, ContentID-based fallback. Top-level text/plain & text/html excluded as body, not attachments.
- `0137-attachment-mime-normalization.md` — store `type/subtype` lowercased, drop params. Filename in its own column; charset irrelevant for binary parts.

- [ ] **Step 4: Update invariants.md**

Edit `docs/poplar/invariants.md` in place:

- Add a new "Attachments" subsection under "Mail model" naming the `mail.Attachment` shape, the Q1 classification rule, and the JMAP/IMAP retrieval mechanisms (Email/get bodyStructure; UID FETCH BODYSTRUCTURE / BODY[<part>]).
- Extend the cache section: schema v5, `attachments` table shape, lazy metadata, lazy bytes with separate size backstop, `MaxAttachmentSize` knob.
- Update the `Backend` minimum surface bullet to include `Attachments` and `FetchAttachment`.
- Update the decision-index table with the three new ADRs.

Verify the file remains ≤ 300 lines.

- [ ] **Step 5: Update STATUS.md**

- Mark Pass 8.6 done.
- Replace the starter prompt with Pass 8.7 (Attachments II — viewer).
- Keep ≤ 60 lines.

- [ ] **Step 6: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-04-attachments-i.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-03-attachments-i-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 7: Commit ADRs + invariants + STATUS + archive**

```bash
git add docs/poplar/decisions/ docs/poplar/invariants.md docs/poplar/STATUS.md docs/superpowers/
git commit -m "Pass 8.6: ADRs 0135-0137, invariants, archive plan/spec

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 8: Push and install**

```bash
git push
make install
```

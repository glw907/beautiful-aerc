---
title: Pass 9m — CardDAV ingest + autocomplete
date: 2026-05-07
status: draft
---

# Pass 9m: CardDAV ingest + autocomplete

Swap `contacts.FixtureSuggestions` and the fixture-backed
`LookupByEmail` for a real CardDAV-driven contacts cache. Read-only
side: discovery, sync, ingest, autocomplete, sender lookup. Write
path (form → CardDAV PUT) splits into Pass 9m.1 — see *Scope split*
below.

Backlog: #34. Compose-dropdown contract is fixed by ADR-0174;
this pass changes only the function pointer behind `SuggestFn`.

## Scope split

The original starter prompt rolled "vCard ingest of saved-form
contacts" into 9m. Counting tasks against the pass-size budget put
it past 12 and the ADR straddled two clearly separable subsystems
(read sync vs. outbound write through the outbox), which the
poplar-pass skill names as a split signal. So:

- **Pass 9m** (this spec) — config, sync, schema, ranking, App
  wiring. Form save remains logged-and-discarded as today.
- **Pass 9m.1** (deferred) — extend `cache.OpKind` for
  `KindContactPut`/`KindContactDelete`, wire the form save through
  the existing outbox drainer, finish the round-trip.

Storage shape in 9m is forward-compatible with 9m.1 (per-resource
ETags, full vCard blob preserved). No 9m.1 schema migration.

## Settled decisions

1. **Schema location** — tables in the existing per-account
   `mail.db` as schema v8. Cache invariants pin one SQLite per
   account; CardDAV creds belong to the account; a separate
   `contacts.db` would duplicate WAL/migration/pool plumbing for
   no win.
2. **Sync model** — `sync-collection` REPORT (RFC 6578) when the
   server advertises it; CTAG-gated full pull as fallback; full
   pull on first contact and on UID-validity-equivalent failures
   (token rejection). Probed once per address book and remembered.
3. **Ranking** — frequency × recency decay over From/To/Cc fields
   in the existing `messages` table. SQL:
   `SUM(1.0 / (1 + days_since_seen))` grouped by email. Joined
   against the contact pool by email; unmatched rows still rank
   (we autocomplete addresses we've corresponded with even if the
   user hasn't carded them).
4. **Save destination** — discover all address books, expose them
   all in the form's "Save to" cycler, pin the initial cursor via
   `[[account.contacts]] default-addressbook = "..."`. First-
   discovered is the silent fallback when the pin is empty or
   stale. (Form save itself stays logged-and-discarded in 9m.)
5. **vCard storage** — preserve the full vCard blob alongside the
   normalized projection columns. CardDAV servers expect the full
   vCard back on PUT; round-tripping a stripped form would lose
   ADR / BDAY / PHOTO / X-* extensions we don't model. Storage
   cost is trivial.
6. **Periodic refresh** — 15-minute ticker started by App on
   first sync completion; also re-syncs on compose-tab open
   (kicks the ticker, doesn't add a second one).

## Non-goals

- Form write-back (Pass 9m.1).
- Sent-history scrape outside the message cache. The interaction
  signal is the `messages` table we already have.
- Photo / address / birthday rendering. The popover and form stay
  on the four-field contact shape (Kind / Name / Org / Note +
  Emails + Phones); other vCard fields preserve in the blob and
  surface in 1.1.
- Contact groups (vCard `KIND=group`, `MEMBER`). Filtered out at
  ingest; logged once per sync.
- Multiple CardDAV servers per account. `[[account.contacts]]` is
  a single block, not a slice.

## Architecture

```
config.AccountConfig
    └── ContactsConfig                       (new: URL, auth,
                                              default-addressbook,
                                              refresh-interval)

internal/contacts/                            (new package, UI-free)
    ├── client.go      CardDAV client wrapping go-webdav/carddav
    ├── sync.go        sync-collection + CTAG fallback + full pull
    ├── vcard.go       vCard → normalized projection (go-vcard)
    └── types.go       AddressBook, Contact, Email, Phone wire types

internal/cache/
    ├── schema_v8.go   addressbooks, contacts, contact_emails,
                        contact_phones, message_recipients
    ├── contacts.go    SyncContacts, SuggestAddresses,
                        LookupContact, populate-recipients hook

internal/ui/                                  (wiring only)
    ├── app.go         SuggestFn := acct.SuggestAddresses,
                        sender lookup := acct.LookupContact
    └── compose/       (unchanged — ADR-0174 contract)

internal/ui/contacts/                         (form/popover unchanged
                                              for shape; phone
                                              validation upgraded)
```

`internal/contacts/` is UI-free, lives next to `internal/mail/` and
`internal/cache/`.

### Type ownership

The plain-value types currently in `internal/ui/contacts/types.go`
(`Contact`, `Email`, `Phone`, `Kind`, `Suggestion`, `AddressBook`)
move down to `internal/contacts/`. They have no UI dependencies;
they belong in the layer that produces them. `internal/ui/contacts/`
keeps the UI sub-models (`Popover`, `Sidebar`, `List`, `Form`,
`RenderDetailCard`, per-package `Styles`) and re-imports the value
types under the same `contacts` package alias — call sites remain
`contacts.Contact`, just from a different path.

The cache returns `[]contacts.Suggestion` directly. `internal/cache/`
is permitted to import `internal/contacts/` (it already imports
`internal/mail/` for the same kind of value-type sharing).
`internal/ui/compose.SuggestFn` keeps its signature unchanged.

## Config

```toml
[[account]]
name = "personal"
provider = "fastmail"
# ...

[account.contacts]
url = "https://carddav.fastmail.com/dav/addressbooks/user/geoff@907.life/"
username = "geoff@907.life"
password-cmd = "fastmail-dav-password"
default-addressbook = "Default"           # optional; href display name
refresh-interval = "15m"                  # optional; default "15m"
```

`url` is the principal's `addressbook-home-set` URL or any URL
under it — the sync engine probes upward to discover the home set
when given a deeper URL. `auth` follows the same shape as
`[account]` itself: explicit `password` or `password-cmd`, default
to mirroring the IMAP/JMAP creds when omitted (Fastmail uses the
app password for both).

Validation (at config load):

- `url` parses as `https://...` (or `http://` for self-hosted with
  an `insecure-tls = true` echo of the parent block — same shape
  as IMAP).
- `default-addressbook`, when set, is a non-empty string. Existence
  is checked at sync time, not config time, since servers can
  rename collections; mismatch surfaces a one-line warning and
  falls back to first-discovered.
- `refresh-interval` parses as `time.ParseDuration` and is ≥ 1m
  (lower bound to keep the server polite).

JMAP-backed accounts may carry `[account.contacts]` independently
— Fastmail JMAP for mail, Fastmail CardDAV for contacts is the
canonical case.

## Schema v8

Five new tables. All scoped per-account (per-database).

```sql
-- One row per discovered address book collection.
CREATE TABLE addressbooks (
    href           TEXT PRIMARY KEY,        -- stable server identity
    display_name   TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    sync_token     TEXT NOT NULL DEFAULT '', -- RFC 6578 token; '' = none
    ctag           TEXT NOT NULL DEFAULT '', -- fallback when no sync-collection
    supports_sync  INTEGER NOT NULL DEFAULT 0, -- 1 = sync-collection probed OK
    last_synced_at INTEGER NOT NULL DEFAULT 0  -- unix seconds
);

-- One row per vCard. uid is the vCard UID property.
CREATE TABLE contacts (
    uid              TEXT PRIMARY KEY,
    addressbook_href TEXT NOT NULL REFERENCES addressbooks(href) ON DELETE CASCADE,
    href             TEXT NOT NULL UNIQUE,    -- per-resource URL
    etag             TEXT NOT NULL,
    vcard            BLOB NOT NULL,           -- raw vCard bytes (round-trip)
    rev              TEXT NOT NULL DEFAULT '',-- vCard REV; sync-time tiebreak
    fn               TEXT NOT NULL,           -- display name
    family           TEXT NOT NULL DEFAULT '',-- N component
    given            TEXT NOT NULL DEFAULT '',
    org              TEXT NOT NULL DEFAULT '',
    title            TEXT NOT NULL DEFAULT '',
    note             TEXT NOT NULL DEFAULT '',
    last_synced_at   INTEGER NOT NULL
);
CREATE INDEX contacts_by_book ON contacts(addressbook_href);

CREATE TABLE contact_emails (
    contact_uid TEXT NOT NULL REFERENCES contacts(uid) ON DELETE CASCADE,
    address     TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',  -- HOME, WORK, ...
    pref        INTEGER NOT NULL DEFAULT 0, -- 0 = unset; lower = preferred
    PRIMARY KEY (contact_uid, address)
);
CREATE INDEX contact_emails_by_addr ON contact_emails(address);

CREATE TABLE contact_phones (
    contact_uid TEXT NOT NULL REFERENCES contacts(uid) ON DELETE CASCADE,
    number      TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    pref        INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (contact_uid, number)
);

-- Recipient projection: one row per (message, address) for ranking.
-- Populated transactionally with messages inserts.
CREATE TABLE message_recipients (
    message_uid INTEGER NOT NULL REFERENCES messages(uid) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK (role IN ('from','to','cc')),
    address     TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    sent_at     INTEGER NOT NULL,           -- unix seconds; mirrors messages.sent_at
    PRIMARY KEY (message_uid, role, address)
);
CREATE INDEX message_recipients_by_addr_sent ON message_recipients(address, sent_at);
```

Migration v7 → v8 also backfills `message_recipients` from existing
`messages` rows by re-parsing `from`/`to`/`cc` headers. Backfill
runs inside the migration transaction; on a cold cache it's a
no-op, on a warm cache it scales with stored message count
(typically tens of thousands; one-time cost).

## Sync flow

`(*cache.Account).SyncContacts(ctx)` is the single entry point.
Driven by App on startup (after first frame) and on the 15-minute
ticker. Per call:

1. **Discover** (only when `addressbooks` is empty or the URL
   changed): walk `current-user-principal` → `addressbook-home-set`
   → PROPFIND depth=1 for `addressbook` resourcetype. Upsert one
   row per discovered collection.
2. **Per book**:
   - If `supports_sync = 1` and `sync_token != ''`: issue
     `sync-collection` REPORT with the stored token. Apply the
     returned changeset (added/changed/deleted hrefs); fetch
     full vCards for added/changed via `addressbook-multiget`.
     Update `sync_token`.
   - Else if `ctag != ''`: PROPFIND for `getctag`; if equal to
     stored, skip; if different, full pull via PROPFIND depth=1
     + multiget. Update `ctag`. After a successful full pull,
     probe `sync-collection` once (REPORT with empty token); on
     2xx set `supports_sync = 1` and store the returned token
     for next run.
   - Else (first sync): full pull. Probe `sync-collection`. Store
     whichever of token/ctag the server gave us.
3. **Recover** from `412 Precondition Failed` or
   `403 valid-sync-token` by clearing the stored token and
   restarting the per-book branch as full-pull.
4. Filter `KIND:group` vCards at parse time — log once per sync
   if any are skipped, then move on.
5. Update `last_synced_at` on each book.

The whole call runs synchronously off a `tea.Cmd`; failures
propagate as `uicore.ErrorMsg`. `mail.ErrAuth` from the underlying
HTTP layer wraps the same way the mail backends do (401/403 →
`ErrAuth`).

## Ranking SQL

`(*Account).SuggestAddresses(prefix string) []Suggestion` runs:

```sql
WITH scored AS (
    SELECT mr.address                                            AS addr,
           MAX(mr.name)                                          AS name,
           SUM(1.0 / (1 + (julianday('now') - julianday(mr.sent_at, 'unixepoch'))))
                                                                 AS score
      FROM message_recipients mr
     WHERE LOWER(mr.address) LIKE LOWER(? || '%')
        OR LOWER(mr.name)    LIKE LOWER(? || '%')
     GROUP BY mr.address
)
SELECT s.addr,
       COALESCE(c.fn, s.name) AS display,
       c.uid                  AS contact_uid,
       s.score
  FROM scored s
  LEFT JOIN contact_emails ce ON LOWER(ce.address) = LOWER(s.addr)
  LEFT JOIN contacts c        ON c.uid = ce.contact_uid
 ORDER BY s.score DESC, s.addr ASC
 LIMIT 7;
```

Plus a UNION arm that surfaces carded contacts the user hasn't
emailed yet but whose name/email matches the prefix, with a
floor score of 0 (so they appear when no history exists).
Implementation lives in one Go function, not a single SQL string,
so the floor-score arm can be skipped when scored ≥ 7.

`LookupContact(addr string) (Contact, bool)` is a single-row
lookup against `contact_emails` joined to `contacts`, returning
the same `internal/ui/contacts.Contact` value the popover renders
today. Misses fall through to the existing `parseSender` shape.

## Recipient population

Hook into the existing message-syncer's insert path: every
`UPSERT INTO messages` runs in the same transaction as
`INSERT OR REPLACE INTO message_recipients` rows decoded from
the same `from`/`to`/`cc` headers. Address normalization (lowercase,
trim) lives in `internal/content` to share with the parse path
already used elsewhere.

Re-parse on conflict-update is fine; the table is keyed on
`(message_uid, role, address)`. Deletes cascade via foreign key.

## App wiring

Three call sites change in `internal/ui/app.go`:

- L611, L620, L721 — `contacts.FixtureSuggestions` →
  `m.acct.SuggestAddresses` (function-typed value, captured into
  the compose model).
- L247 — `contacts.LookupByEmail(contacts.Fixtures(), msg.Email)`
  → `m.acct.LookupContact(msg.Email)`.

`internal/ui/contacts/fixtures.go` keeps `Fixtures()` and
`FixtureSuggestions` for tests; nothing in App imports them after
this pass.

A new `tea.Cmd` — `syncContactsCmd` — fires on initial App init
(after the first cache-event subscription) and from a 15-minute
ticker. Errors surface through the standard `uicore.ErrorMsg`
channel.

## Phone validation

Pull in `github.com/nyaruka/phonenumbers`. Replace the "any
non-empty string" check in `internal/ui/contacts/form.go` with
`phonenumbers.Parse(num, defaultRegion)` where `defaultRegion`
defaults to "US" (matches workstation locale; revisit when a
locale config arrives). Invalid numbers block save with the
existing error-message channel; valid numbers store as-typed (no
forced E.164 reformat — vCards preserve user-entered form).

## Tests

Unit:

- `internal/contacts/vcard_test.go` — table-driven vCard parse
  fixtures: minimal, multi-email, group (filtered), unknown X-*
  preserved on round-trip.
- `internal/contacts/sync_test.go` — mock CardDAV server (`net/http/httptest`),
  exercise sync-collection happy path, CTAG fallback, token-reject
  recovery, group filtering.
- `internal/cache/contacts_test.go` — schema migration v7→v8 +
  backfill; `SuggestAddresses` ranking with synthetic message
  history; `LookupContact` hit/miss.

Live tmux verification at 80×24 and 120×40:

- Compose with To: prefix matching a real cached contact, dropdown
  shows expected ranking.
- Reader popover (`i` on a sender) renders cache-backed Contact.
- 15-minute refresh observable via debug log line on tick.

## Risks / open tells

- **`sync-collection` probe noise.** The first post-CTAG-pull
  probe issues an extra REPORT against servers that don't
  support it; some return 501 / 405 / 403. Treat 4xx/5xx on the
  probe as "doesn't support," cache `supports_sync = 0`, never
  probe again. No retry.
- **`message_recipients` backfill on giant caches.** If the
  table holds 100k+ messages, the v7→v8 migration runs the
  backfill in a single transaction. SQLite handles it but it
  will block first launch for seconds. Acceptable — pre-beta,
  one-time, log a "migrating contacts index" notice. Revisit
  if a real user trips it.
- **Default-region for phone parse.** Hardcoding "US" works for
  the workstation user but is wrong for international users.
  Logged as a follow-up when a locale config arrives — not a
  9m blocker.
- **vCard 4.0 vs 3.0.** Fastmail returns 3.0; some servers
  return 4.0; `go-vcard` handles both. The `PREF` semantics
  differ (3.0: `TYPE=PREF`; 4.0: `PREF=1..100`). Normalize at
  parse time into the `pref` integer column.

## Pass-end deliverable

ADR-0175 (or next available) — *CardDAV ingest and contacts
ranking*. Updates invariants.md with: `internal/contacts/`
package, schema v8 (five new tables), `[[account.contacts]]` config
shape, `SyncContacts` / `SuggestAddresses` / `LookupContact` cache
methods, App wiring swap. Updates `docs/poplar/decisions/INDEX.md`.
Pass 9m.1 starter prompt drafted into STATUS.md.

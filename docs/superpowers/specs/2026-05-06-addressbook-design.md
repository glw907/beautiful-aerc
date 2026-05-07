# Address book — design

**Pass:** 9.1 (UI mockups), 9.2 (data + read wiring), 9.3 (edit + CardDAV write)
**Issue:** #34
**Date:** 2026-05-06

## Goal

Poplar grows a built-in address book — a streamlined digital
rolodex backed by SQLite, syncing read-and-write to CardDAV,
reachable from compose autocomplete, the mail viewer (`i`-popover
on a focused message), and a dedicated Contacts mode (`C` from the
account view). Five stored fields per contact: name, optional
org/title, multi-email with labels, multi-phone with labels,
optional note. No addresses, URLs, birthdays, photos, IM handles,
PGP keys. The card you'd actually keep on a desk, not the HR
record.

A user typing `ali` into compose's `To:` sees Alice Chen's two
emails as separate suggestions. Pressing `i` in the reader on a
message from `support@acme.com` opens a popover showing the ACME
Support card. Pressing `C` from the account view opens Contacts
mode: T9 sidebar, scrollable list, detail card on the right,
`n`/`e` to add or edit. Edits sync back via CardDAV PUT.

## Scope

### In (initiative — three passes)

**Pass 9.1 — UI mockups, stub data.** All four visual surfaces
implemented against in-memory fixture contacts. Iterate visual
until shape feels right; no data layer, no sync, no persistence.
Live tmux verification at 80×24 and 120×40 for every surface. Pass
ends with screenshots and an ADR locking visual decisions.

**Pass 9.2 — Data foundation + read wiring.**
`internal/addressbook/` package backed by a single user-global
SQLite at `~/.local/state/poplar/contacts.db`. All CardDAV sync
state (sync tokens, addressbook URLs, last-sync timestamps) lives
in this DB keyed by account name — `cache.Account` is not
extended. Three sources: file (vCard import), CardDAV read sync,
header-cache (automatic from message-fetch). `Query` interface
with source-priority cascade. Wire autocomplete + `i`-popover +
Contacts mode browse to real data. `poplar contacts
import/export/list` CLI.

**Pass 9.3 — Edit + CardDAV write.** Wire the contact form's save
paths. CardDAV PUT with ETag conflict handling. New
`KindContactPush` outbox op (mirrors `KindPushDraft` shape from
9h.6). Conflict surface: ETag mismatch on PUT routes through the
existing conflict overlay (`!`).

### Out (deferred to 1.x or never)

- Addresses (street/city/zip), URLs, birthdays, categories,
  IMPP, PGP/KEY, PHOTO, RELATED, GEO, custom X-* fields.
  Imported but discarded; never displayed; not round-tripped.
- vCard 4.0 export's full type taxonomy. We support a small
  enumerated label set per field (Email: work/home; Phone:
  mobile/work/home/fax). Other TYPE values from imports are
  dropped; they don't round-trip.
- Multi-addressbook per CardDAV account. v1 syncs the principal's
  default book only; multi-book picker is post-1.0.
- Sync conflict UX for concurrent multi-device edits beyond
  ETag-mismatch routing through `!`. No three-way merge.
- Contact groups / mailing lists. vCard `KIND:group` is read as
  `'individual'` and the GROUP-CONTAINS membership is dropped.
- Avatar / photo support (terminal client; PHOTO field always
  dropped).
- LDAP, Google People API, Exchange GAL — CardDAV is the only sync
  protocol.
- Address book editing of header-cache rows in place. Editing a
  header-cache contact in the form forces a "save to" picker
  (Local file or CardDAV account) — effectively promoting it.

## Field set

The "minimum useful" rolodex fields, after iteration in the
brainstorm:

| Stored | Displayed | Form input | vCard mapping |
|---|---|---|---|
| `kind` | controls org/title rendering | Person/Business toggle | `KIND` (4.0) |
| `name` | first line | First+Last (Person) or Name (Business) | `FN` |
| `family` | last-name sort key | Last field (Person only) | `N` family component |
| `given` | first-name sort key | First field (Person only) | `N` given component |
| `org` | second line, after title | Org field (Person only) | `ORG` |
| `title` | second line, before org | Title field (Person only) | `TITLE` |
| `emails[]` + label + position | one line each (`label, primary`) | Repeating rows | `EMAIL` + `TYPE` + `PREF` |
| `phones[]` + label + position | one line each (`label, primary`) | Repeating rows | `TEL` + `TYPE` + `PREF` |
| `note` | bottom block, under separator | Multi-line textarea | `NOTE` |

`kind` enum: `'individual'` (default) | `'org'`. vCard `KIND:org`
imports as `'org'`; anything else (or absent) imports as
`'individual'`. Business contacts skip family/given/org/title in
the form and at render time.

## Schema

User-global SQLite at `~/.local/state/poplar/contacts.db` (XDG
state dir). Three tables; no joins required for autocomplete or
list rendering.

```sql
CREATE TABLE contacts (
  id          INTEGER PRIMARY KEY,
  kind        TEXT NOT NULL DEFAULT 'individual',  -- 'individual' | 'org'
  name        TEXT NOT NULL,                       -- FN; display string
  family      TEXT,                                -- N family;  NULL for kind='org'
  given       TEXT,                                -- N given;   NULL for kind='org'
  org         TEXT,                                -- NULL for kind='org' or unset
  title       TEXT,                                -- NULL for kind='org' or unset
  note        TEXT,                                -- NOTE; multi-line
  source      TEXT NOT NULL,                       -- 'file' | 'carddav' | 'header-cache'
  account     TEXT,                                -- NULL for 'file'
  external_id TEXT NOT NULL,                       -- vCard UID, or normalized email for header-cache
  rev         TEXT,                                -- CardDAV ETag/REV; NULL otherwise
  updated_at  INTEGER NOT NULL,                    -- unix seconds
  UNIQUE(source, account, external_id)
);
CREATE INDEX contacts_name_idx ON contacts(name COLLATE NOCASE);

CREATE TABLE emails (
  contact_id INTEGER NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  address    TEXT NOT NULL,                        -- as-imported casing
  normalized TEXT NOT NULL,                        -- lowercased + trimmed
  label      TEXT,                                 -- 'work' | 'home' | NULL
  PRIMARY KEY (contact_id, normalized)
);
CREATE INDEX emails_normalized_idx ON emails(normalized);

CREATE TABLE phones (
  contact_id INTEGER NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  e164       TEXT NOT NULL,                        -- '+15555550100'
  label      TEXT,                                 -- 'mobile' | 'work' | 'home' | 'fax' | NULL
  PRIMARY KEY (contact_id, e164)
);
CREATE INDEX phones_e164_idx ON phones(e164);

CREATE TABLE addressbook_state (
  account     TEXT PRIMARY KEY,                    -- account name
  sync_token  TEXT,                                -- RFC 6578 sync-token
  ctag        TEXT,                                -- fallback when sync-token unsupported
  url         TEXT NOT NULL,                       -- addressbook collection URL
  last_sync_at INTEGER                             -- unix seconds; NULL on first run
);

CREATE TABLE schema_version (version INTEGER NOT NULL);
INSERT INTO schema_version VALUES (1);
```

**Position-as-primary.** Neither `emails` nor `phones` carries an
`is_primary` flag. Primary is whichever row sorts first by
`rowid`. The form rewrites all rows for the contact on save in the
user's chosen order; first row wins. Import primary cascade
(`PREF=1` → `TYPE=pref` → first occurrence) controls insertion
order, not a flag.

**Schema versioning.** v1 ships the shape above. Future migrations
get a `v2`/`v3`/etc., transactional, applied on `Open` like
`internal/cache/`'s pattern.

**Phone normalization.** All `e164` values pass through
`nyaruka/phonenumbers.Parse(raw, defaultRegion)` at insert time.
Default region from `[ui] phone_default_region` (defaults to
`"US"`, resolvable from `$LANG` country code). Unparseable phones
are dropped at import with a debug log; the form blocks save until
a parseable value (or empty field) replaces an invalid entry.

**Sort order.** Configurable via `[ui] contacts_sort = "first"`
(default) or `"last"`. Both modes use a single COALESCE-shaped
`ORDER BY`:

```sql
-- "first"
ORDER BY COALESCE(given, name) COLLATE NOCASE, family COLLATE NOCASE

-- "last"
ORDER BY COALESCE(family, name) COLLATE NOCASE, given COLLATE NOCASE
```

Org-kind rows have NULL family/given; COALESCE returns `name` so
"ACME Support" sorts on its FN regardless of mode. No dedicated
sort index — contact tables stay small (low thousands at the high
end).

## Sources

Three sources behind one `addressbook.Query` interface. Source
priority for autocomplete + `Lookup`: `file > carddav >
header-cache`. Dedup on `emails.normalized`.

**`file`** — User-global. Imported via `poplar contacts import
<path.vcf>`. Rows have `source='file', account=NULL`. Re-import is
**additive upsert** by vCard `UID` — rows present in DB but absent
from the re-import batch are *not* deleted (supports importing
multiple .vcf files independently). `poplar contacts purge
--source=file` clears the file source explicitly when the user
wants a full reset. Export emits all `source='file'` contacts to
a single vCard file.

**`carddav`** — Per-account. Configured under `[[account.contacts]]`
sub-table on each `[[account]]` (mirrors `[account.smtp]` from
ADR-0157, including credential-defaulting to the IMAP/JMAP-side
password when absent). Sync state (sync-token, CTag, addressbook
URL, last-sync timestamp) lives in the user-global
`addressbook_state` table keyed by account name — per-resource
ETags ride on `contacts.rev`. Read loop via `emersion/go-webdav`'s
`carddav` subpackage; `sync-collection` REPORT (RFC 6578) when
supported, CTag + ETag-per-resource fallback. 30-min ticker plus
app-start fire. Failures route through `ErrorMsg`. Multi-addressbook
deferred — v1 picks the principal's default book.

**`header-cache`** — Per-account. Auto-harvested from
`mail.MessageInfo.From` during message-list fetch. Rows have
`source='header-cache', account=<account-name>, external_id=
<normalized-email>`. Name parsing: simple "last word = family,
rest = given" heuristic; imperfect but acceptable for an automated
source. Editing a header-cache row promotes it to `file` or
`carddav` (forces destination picker in the form).

## Display surfaces

### Autocomplete dropdown (compose To/Cc/Bcc)

Inline dropdown anchored under the focused header textinput. One
row per email — Alice with two emails appears twice. Tab/Enter
accepts the cursor row (top match by default), arrows navigate,
Esc dismisses. Per-keystroke query after the first 2 characters.

```
To: ali|
    ┌───────────────────────────────────────────────────┐
    │ Alice Chen      <alice@example.com>     · ACME    │
    │ Alice Chen      <a.chen@work.com>       · ACME    │
    │ Alison Park     <ali@park.io>           · Park…   │
    └───────────────────────────────────────────────────┘
```

Dim `· {org}` suffix only when `org` is set and `kind='individual'`.
Org rows render the org name as the primary visible text already
(`name` IS the org), so no suffix.

Accept rewrites the textinput to `Name <email>, ` and bumps
header-cache `seen_at` (when the matched row is `source='header-
cache'`). The `seen_at` bump biases future ranking toward
recently-used addresses without exposing a separate frequency
field in the UI.

### `i`-popover (mail viewer / message list)

Press `i` in mail mode (account view, message focused in list or
reader open) → modal popover containing the sender's contact card.
Modal shape: `ModalShell`-rendered, dim underlay, centered. Same
detail-card render as Contacts mode.

Lookup: `Query.LookupByEmail(normalized_from)` against all sources
with priority cascade. If no match, popover shows the From
header's display name + email plus a hint:

```
 ┌─ Sender ─────────────────────────┐
 │                                  │
 │  Alice Chen                      │
 │  <alice@example.com>             │
 │                                  │
 │  No contact in address book.     │
 │                                  │
 │  n add contact   Esc dismiss     │
 └──────────────────────────────────┘
```

`n` on the no-match popover opens the contact form pre-filled with
the From header's name + email; user picks save destination
(file / CardDAV account); save closes both modal and form.

`i` (re-press) and `Esc` dismiss the popover.

### Contacts mode

`C` from the account view enters Contacts mode. `M` returns to
mail. No top tab bar; mode is indicated in the sidebar header row
(`CONTACTS · {book} · {account}` for CardDAV-backed; `CONTACTS ·
Local file` for the file source; `CONTACTS · All sources` when
nothing is filtered).

Three-column layout, mirroring mail mode:

- **Left (sidebar)** — T9 letter index. Eight rows: `ABC`, `DEF`,
  `GHI`, `JKL`, `MNO`, `PQRS`, `TUV`, `WXYZ`. Right-aligned per-
  group count; blank row between groups. `J/K` walks groups; the
  matching letter within the active group renders with an inline
  `┃` indicator (per-letter micro-highlight, see WIP memory). `a`
  through `z` jumps to per-letter precision in the right panel
  and auto-follows the sidebar cursor.
- **Middle (list)** — Scrollable list of contacts. Row shape (in
  default first-name sort):

  ```
  Alice Chen          alice@example.com  +1 555-0100   Senior Engineer · ACME
  Bob Iyer            bob@iyer.dev       —             —
  Carla Méndez        c@mendez.org   +2  +1 555-0199   CEO · Globex
  ```

  In last-name sort: `Chen, Alice` ... `Iyer, Bob` ... etc.

  Org-kind rows render `name` directly with em-dash in the org/
  title column.

  `j/k` cursor; `Enter` is inert (the detail card auto-renders for
  the cursor row); `n` opens the new-contact form; `e` opens the
  edit form for the cursor; `D` deletes the cursor (with
  ConfirmModal).

- **Right (detail card)** — Auto-renders for the cursor row. Same
  renderer as the `i`-popover. See "Detail card" below.

### Detail card (Contacts mode right column + `i`-popover content)

Person:
```
 Alice Chen
 Senior Engineer · ACME

 alice@example.com         (work, primary)
 a.chen@personal.io        (home)

 +1 555-0100               (mobile, primary)
 +1 555-0199               (work)

 ─────────────────────────
 Met at GopherCon 2024.
 Cares about error messages.
```

Business:
```
 ACME Support

 support@acme.com          (primary)
 +1 555-0199

 ─────────────────────────
 Vendor for the
 build-pipeline contract.
```

Render rules:
- Line 1: `name` (FN).
- Line 2: `{title} · {org}` for `kind='individual'`. Skip if both
  empty. Single-side render if only one populated. Suppressed
  entirely for `kind='org'`.
- Email block: always present (at least one row required by
  schema invariant). Each row: `address    (label, primary)` for
  the first row; `address    (label)` for subsequent rows.
  Label parens omitted entirely if `label IS NULL`.
- Phone block: present iff at least one phone exists. Same
  per-row format as emails.
- Note block: present iff `note IS NOT NULL` and non-empty after
  trim. Separator rule above.

### Contact edit form

Modal opened by `n` (new) or `e` (edit on cursor row). Form
covers the right panel of Contacts mode (or the full-screen modal
when invoked from `i`-popover). `Tab`/`Shift+Tab` cycles fields;
`Ctrl+S` saves; `Esc` cancels with ConfirmModal if dirty.

Person form:
```
┌─ New contact ──────────────────────────────────────────┐
│                                                        │
│  Kind:    ● Person   ○ Business                        │
│                                                        │
│  First:   [Alice___________________________]           │
│  Last:    [Chen____________________________]           │
│  Org:     [ACME____________________________]           │
│  Title:   [Senior Engineer_________________]           │
│                                                        │
│  Emails:  [alice@example.com_______________] ◀Work▶ ★− │
│           [a.chen@personal.io______________] ◀Home▶  − │
│           [+ add email]                                │
│                                                        │
│  Phones:  [+1 555-0100_____________________] ◀Mob.▶ ★− │
│           [+1 555-0199_____________________] ◀Work▶  − │
│           [+ add phone]                                │
│                                                        │
│  Note:    [Met at GopherCon 2024.________________]     │
│           [Cares about error messages.___________]     │
│                                                        │
│  Save to: ● Fastmail (CardDAV)   ○ Local file          │
│                                                        │
│  Tab/Shift+Tab navigate · Ctrl+S save · Esc cancel     │
└────────────────────────────────────────────────────────┘
```

Business form (kind toggled): hides First / Last / Org / Title.
Replaces with single `Name:` field.

Field rules:
- **Kind toggle** — `Tab` reaches it; on the toggle, `Space` or
  `←/→` flips. Flipping clears family/given/org/title from form
  state (preserves Name), removes those rows from tab order.
- **Name fields** — `name` derived from `given + ' ' + family` on
  Person save (or just one if the other is empty). Required;
  validation blocks save if empty after trim.
- **Org / Title** — both optional. Only present in Person form.
- **Emails** — minimum 1 row (validated on save). `+ add email`
  appends to bottom. Each row: address textinput + label cycler
  (`Work` / `Home` / blank) + `★`/`☆` position-as-primary
  indicator + `−` delete. `★` on row 0 is non-actionable
  (already primary). `☆` on rows 1+ promotes that row to
  position 0 (demoting the previous row 0 to row 1). `−` is
  disabled on the only-email row.
- **Phones** — minimum 0. Same row pattern. Label cycler offers
  `Mobile` / `Work` / `Home` / `Fax` / blank. E.164 normalization
  on field blur.
- **Note** — small multiline textarea, 3 visible rows, scrolls.
- **Save to** — required. Options: `Local file` plus one entry
  per CardDAV-configured account by name. Header-cache is never
  a save destination. Prefilled from the source of the edited
  row (or first available for `n`).

Validation summary (Save blocked unless all true):
- Name (Person: First or Last non-empty; Business: Name non-empty)
- ≥1 email
- All emails parse as RFC 5322 addresses
- All phones parse to E.164 (or are empty)
- Save destination selected

Dirty-check on `Esc` opens the standard ConfirmModal asking
"Discard changes?" — same shape as compose's cancel flow.

## Form-within-Contacts-mode UX

When `n` or `e` fires from Contacts mode, the form replaces the
right panel (the detail card). Sidebar + middle list stay visible
and scrollable but inert (keys route into the form). Save closes
the form and returns the right panel to detail-card mode for the
(possibly new or modified) cursor row.

When the form opens from the `i`-popover's "no contact" affordance,
it renders as a centered overlay (`ModalShell`) over the dimmed
mail-mode chrome.

## CardDAV write (Pass 9.3)

CardDAV writes go through a new outbox op, `KindContactPush`,
modeled after `KindPushDraft` from ADR-0165. Args carry the contact
ID; payload is the gob-encoded `addressbook.Contact` value. The
drainer's dispatch:

1. Build vCard 4.0 bytes via `addressbook.EncodeVCard(c)`.
2. PUT to `<addressbook-url>/<contact-uid>.vcf` with
   `If-Match: <current rev>` (or `If-None-Match: *` on create).
3. On 200/201/204: parse the new ETag from response, update
   `contacts.rev`, mark `OpDone`.
4. On 412 (precondition failed — server changed): re-pull that
   contact, mark `OpConflict precondition-failed`. The conflict
   overlay (`!`) lets the user retry (re-encode against the
   freshly-pulled rev) or discard (revert form-state changes).
5. On `ErrAuth` / network: standard outbox conflict matrix.

Local file writes don't go through the outbox — they're synchronous
inline (atomic write of the .vcf file). File-source contacts in
the DB get their `updated_at` bumped; the .vcf file is rewritten
in full from the current `source='file'` set.

## Pass 9.1 — UI mockups, stub data

This pass owns the visual decisions; nothing wires to data.

**Deliverables:**

1. `internal/ui/contactspopover/` (or similar — naming TBD at
   plan time). Renders the `i`-popover detail card from a stub
   `Contact` value. Key dispatch from mail mode (`i` toggles).
2. `internal/ui/contacts/` — Contacts mode shell. Three columns
   (T9 sidebar, list, detail card). Renders against an in-memory
   fixture slice of ~30 contacts (mix of Person + Business,
   single + multi-email, with/without notes). `M`/`C` mode toggle
   from the account view.
3. `internal/ui/contacts/form.go` — Contact edit form modal.
   Person + Business variants, all field types, position-as-
   primary widget, validation feedback. Save and cancel are
   no-ops (dispatch a future `ContactSaveMsg` for 9.2 to consume).
4. Autocomplete dropdown shell in `compose.Suggest` — wires the
   focused-textinput overlay positioning, key dispatch (Up/Down/
   Tab/Enter/Esc), row rendering. Backed by a stub
   `SuggestFn(prefix string) []Suggestion` that returns 5–7
   fixture rows from the same fixture pool.
5. Live-tmux verification at 80×24 and 120×40 for every surface.
   Screenshots committed under `docs/poplar/wireframes/contacts/`
   (the existing wireframes doc gets new sections).
6. ADR locking the visual decisions (component layouts, key
   bindings, fixture-driven mockup pattern) and confirming the
   pass-9.2 wiring contract (the `SuggestFn` signature, the
   `ContactSaveMsg` shape, the popover's `LookupFn` signature).
7. Standard `poplar-pass` ritual: invariants update, plan
   archival, commit + push + install.

**Out (this pass):**
- Database, sources, sync, vCard parser/emitter, `Query`
  interface implementation. Stub fixtures only.
- CLI commands (`poplar contacts import/export/list`).
- Header-cache harvesting hooks.
- ETag conflict handling.
- Form save side effects.

## vCard library

`emersion/go-vcard` (MIT, GitHub `emersion/go-vcard`). Confirmed
via direct source review of `decoder.go`, `card.go`, and `v4.go`
on master at commit `c9703dd` (Oct 2024).

**The decoder is version-agnostic.** No `VERSION`-field branching
anywhere in the parse path; it reads vCard 2.1, 3.0, and 4.0 into
the same `map[string][]*Field` shape. The "primarily 4.0" framing
on `pkg.go.dev` is misleading — the parser doesn't care about
versions, only the conversion helper `ToV4()` does, and we don't
need that helper because our field set is syntactically identical
across 3.0 and 4.0:

- `FN`, `N`, `ORG`, `TITLE`, `EMAIL`+`TYPE`+`PREF`, `TEL`+`TYPE`+
  `PREF`, `NOTE`, `UID`, `REV` — same syntax in both versions.
- `KIND` is 4.0-only but harmless when missing on 3.0 import (we
  default to `'individual'`).

**`Card` round-trips unknown fields automatically.** The
map-based type means any field we don't read passes through to
export untouched. Solves preservation for `KIND`, `REV`, `UID`,
and any `X-*` fields without explicit code on our side.

**KIND, PREF, TYPE all have dedicated accessors:** `Card.Kind`,
`Card.Preferred`, `Params.Types`, `Params.HasType`.

**Documented limitations** (acceptable):

- Issue #32 — `ENCODING=QUOTED-PRINTABLE` unsupported. This is
  vCard 2.1 territory (old Apple Address Book <= OS X 10.7).
  Modern exports don't use it. Imports of such files fail; we
  document the limitation and accept it.
- Issue #5 — comma-joined multi-value properties (e.g.,
  `EMAIL:a@x.com,b@y.com`) only partially supported. Real-world
  EMAIL/TEL exports use repeated lines, not comma-joins. Not
  affecting our import path.
- No stable tagged release; pseudo-version `v0.0.0-…-c9703dd`.
  Vendor at a pinned commit. Library is ~18 KB total source —
  small enough to fork in-tree if the project ever stalls.

**Maintenance signal:** last commit Oct 2024 (deterministic
encoder merged from `oliverpool`). Sporadic but alive. emersion
maintains the rest of the email-stack we already vendor
(`go-imap`, `go-message`, `go-smtp`, `go-sasl`, `go-webdav`);
library-family consistency is itself a reason to stay.

**Decision: keep `emersion/go-vcard`.** No shim needed for 3.0.
Pass 9.2 imports the library directly.

## Library: phonenumbers

`nyaruka/phonenumbers` (BSD-3, GitHub `nyaruka/phonenumbers`). Go
port of Google's libphonenumber. Used at vCard import + form blur
to normalize phones to E.164 against `[ui] phone_default_region`.
Adds ~MB of metadata; acceptable for a single-binary mail client
(poplar's existing binary is already ~30 MB).

## Configuration

Three new keys in `~/.config/poplar/config.toml`:

```toml
[ui]
contacts_sort = "first"          # "first" (default) or "last"
phone_default_region = "US"      # ISO 3166-1 alpha-2; resolved from $LANG if unset
```

Per-account CardDAV (added in 9.2):

```toml
[[account]]
provider = "fastmail"
# ... existing imap/jmap/smtp config ...

[account.contacts]                # Optional. Provider preset auto-fills for fastmail/icloud.
url = "https://carddav.fastmail.com/dav/addressbooks/user/.../Default/"
# password / password-cmd default to mirroring the IMAP/JMAP block
# when absent (same shape as [account.smtp], ADR-0157).
```

Provider presets that get CardDAV defaults: `fastmail`, `icloud`,
`zoho` (server-side support varies for the rest; user adds
explicit `[account.contacts]` if needed).

## Keybindings

New bindings landing across the three passes (full table goes in
`docs/poplar/keybindings.md` at 9.1 close):

| Key | Mode | Action | Pass |
|---|---|---|---|
| `i` | mail (any) | Open sender card popover | 9.1 |
| `Esc` / `i` | popover | Close sender card popover | 9.1 |
| `n` | popover (no match) | Open contact form pre-filled | 9.1 |
| `C` | account view | Switch to Contacts mode | 9.1 |
| `M` | Contacts mode | Switch back to mail | 9.1 |
| `j`/`k` | Contacts list | Cursor down/up | 9.1 |
| `J`/`K` | Contacts mode | Walk T9 sidebar groups | 9.1 |
| `a`–`z` | Contacts mode | Letter precision jump | 9.1 |
| `n` | Contacts mode | New contact form | 9.1 |
| `e` | Contacts mode | Edit cursor contact | 9.1 |
| `D` | Contacts mode | Delete cursor (ConfirmModal) | 9.2 |
| `Tab` / `Shift+Tab` | autocomplete dropdown | Cycle suggestions | 9.1 |
| `Enter` | autocomplete dropdown | Accept cursor suggestion | 9.1 |
| `Esc` | autocomplete dropdown | Dismiss | 9.1 |
| `Tab` / `Shift+Tab` | contact form | Cycle fields | 9.1 |
| `Space` / `←` / `→` | kind toggle | Flip Person/Business | 9.1 |
| `Ctrl+S` | contact form | Save (ADR-0076 text-entry exempt) | 9.1 (no-op) / 9.2 (real) |
| `Esc` | contact form | Cancel (ConfirmModal if dirty) | 9.1 |

## Open questions

None remaining. Decisions deferred to sub-pass plans:
- Exact module split for `internal/ui/contacts/` vs
  `internal/ui/contactspopover/` (one package vs two). Plan-time.
- Fixture file format for 9.1 mockups (Go literal slice vs JSON).
  Plan-time.

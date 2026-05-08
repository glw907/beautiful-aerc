---
title: CardDAV write-back — outbox kinds, vCard patch policy, nullable outbox.folder
status: accepted
date: 2026-05-07
---

## Context

Pass 9m landed CardDAV ingest read-only — the contact-edit Form
emitted `ContactSaveMsg`, but the App handler logged-and-discarded.
Pass 9m.1 round-trips the form: cache upsert → outbox PUT → CardDAV
server, plus `D`-key delete via the same path. Two design choices
shaped the work:

- **Lossless edits.** Other CardDAV clients (iOS Contacts, etc.)
  routinely set fields poplar doesn't model — BDAY, ADR, IMPP,
  PHOTO, X-properties. Rebuilding the vCard from poplar's
  `Contact` projection on every save would silently strip those.
  A shared address book that loses fields on poplar edits is an
  interop bug.
- **Schema fit.** The outbox row is folder-scoped for mail ops.
  Contact ops have no folder. Pass 9m.1's first cut used a
  sentinel `__contacts__` folder row to satisfy the FK; review
  flagged it as a schema hack. Pre-beta posture endorses
  migrations, so the right fix landed inline.

## Decision

**Two new outbox kinds + Writer seam.** `KindContactPut` and
`KindContactDelete` extend `cache.OpKind`. `ContactPutArgs{BookHref,
Href, IfMatch}` and `ContactDeleteArgs{Href, IfMatch}` carry the
metadata; the assembled vCard rides in `outbox.payload` for puts.
`contacts.Writer` is the dispatch seam (`PutAddressObject`,
`DeleteAddressObject`, `Multiget`); `*contacts.Client` satisfies it
and is wired onto `cache.Account.ContactsWriter` once at App init,
shared with the existing `SyncContacts` path so the client is built
exactly once per account.

**Patch, not rebuild.** `contacts.PatchVCard(stored, c, now)`
decodes the stored vCard, mutates only the keys poplar owns (FN, N,
ORG, TITLE, NOTE, EMAIL, TEL, REV, KIND, UID), and re-encodes —
every other field round-trips verbatim. `contacts.BuildVCard(c, uid,
now)` covers the new-contact case where no stored blob exists. PREF
is re-derived (index 0 gets `PREF=1`, others cleared); retained
EMAIL/TEL rows keep their existing TYPE param (preserves iOS quirks
like `_$!<Work>!$_`); added rows get poplar's canonical labels.

**Schema v9 — nullable `outbox.folder`.** The folder column drops
`NOT NULL`; the contact-op outbox rows insert `NULL` for folder.
Migration rebuilds the table (rename → recreate → copy → drop →
rename), `LEFT JOIN`s folder lookup in the drainer's pickup query,
and removes any `__contacts__` sentinel rows from earlier
intermediate states.

**Conflict matrix mirrors mail-outbox.** Sentinels `contacts.ErrAuth`
/ `ErrNotFound` / `ErrPreconditionFailed` route via `errors.Is`:
auth → `OpConflict auth-failure`; precondition → `OpConflict
precondition-failed` (412 = stale ETag, user retries through the
conflict overlay); not-found → idempotent success (delete already
gone is success; put against a moved href will surface as a
follow-up sync conflict, not a put error). The `D` key is gated on
`existingUID != ""` and on focus *not* being a text input — typing
"Davis" in a Last Name field doesn't open delete confirm. The
confirm cascade picks form-discard before contact-delete before
compose-save before empty-folder.

## Consequences

vCard fields poplar doesn't model survive every edit, so shared
address books between poplar and iOS / Apple Contacts / DAVx⁵ stay
intact. The drainer's existing conflict overlay handles 412
conflicts uniformly — the user retries, the cache re-fetches,
their edit re-applies against the fresh ETag. The single contacts
client cuts a redundant TCP/TLS handshake from each sync. The
nullable `outbox.folder` migration is the second pre-beta schema
bump for contacts (after v8 ingest); future contact-shape changes
ride on the same discipline. `LookupContact` now returns
`(Contact, uid, ok)` so the popover and form can thread the UID
through `OpenFormMsg` to `WithExistingUID` — the form knows when
it's editing an existing contact. Multi-book destination selection
remains post-1.0 (`queueContactPutCmd` calls
`Account.DefaultBookHref`); the `ContactSaveMsg.SaveTo` field is
ignored at the cmd boundary today and exists for that future pass.

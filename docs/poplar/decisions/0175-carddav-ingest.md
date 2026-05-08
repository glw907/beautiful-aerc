---
title: CardDAV ingest and contacts ranking
status: accepted
date: 2026-05-07
---

## Context

Pass 9.1 wired the address-book UI on top of a fixture pool —
suggestions, sender-popover lookup, and the Contacts-mode list all
read from `contacts.Fixtures()`. v1 needs a real backend. Fastmail
exposes CardDAV; most self-hosted servers do too; the emersion
library family already in poplar (go-webdav, go-vcard) covers it
without new vendor surface. The fixture pool's flat pre-sorted shape
hid the question of *what* "good autocomplete" means with real data;
tying the suggestion query to the user's actual sent/received
recipients makes recency a first-class signal alongside the carded
pool.

## Decision

`internal/contacts/` (UI-free) wraps `go-webdav/carddav` for
discovery, multiget, and sync-collection / CTAG strategies, and
`go-vcard` for parse. A `Store` interface (`Books`, `UpsertBook`,
`ApplyChangeset`) lets `internal/cache` own persistence. The
sync orchestrator picks per-book strategy: `sync-collection` when
the server advertised it on a prior pass, CTAG short-circuit
otherwise, full pull as the fallback (and the bootstrapping
path).

Schema v8 adds five tables: `addressbooks`, `contacts`,
`contact_emails`, `contact_phones`, `message_recipients`. The last
is a recency projection populated alongside every message upsert
plus a one-time backfill from existing `messages` rows. The
suggestion query joins `message_recipients` to the carded pool
with a recency-decay score `Σ 1.0 / (1 + days_ago)`; ties break by
address ASC; LIMIT 7 to match the dropdown's row cap.

Config: `[[account.contacts]]` block (URL, username, password /
password-cmd, default-addressbook, refresh-interval, insecure-tls)
with credential fallback to the parent account when fields are
empty. Validation: https-or-http-with-insecure-tls, refresh-interval
≥ 1m, default 15m. Sync runs at startup + every refresh-interval
via `tea.Tick`. Phone validation upgraded from "non-empty string"
to `phonenumbers.Parse` (default region US).

Read-only this pass — no form save, no PUT round-trip. Pass 9m.1
picks up writeback.

## Consequences

Compose autocomplete and the `i`-popover now reflect the user's
real address book. The `message_recipients` projection unlocks
ranking refinements (frequency boosts, contact-status weighting)
without further schema work. Pass 9m.1 inherits `KindContactPut` /
`KindContactDelete` outbox kinds + ETag round-trip + form save
destination cycler. The 14-cell schema bump is the first of pre-
beta's "schema work is welcomed" examples (per ADR-0103); future
contact-shape changes ride on the same migration discipline.

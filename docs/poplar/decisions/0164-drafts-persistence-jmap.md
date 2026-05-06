---
title: Drafts persistence (JMAP, end-to-end)
status: accepted
date: 2026-05-06
---

## Context

Closing compose without sending lost the draft. Cross-device draft
visibility is table stakes for users who flip between poplar and
Fastmail web on phone. Pass 9h.5 lands persistence end-to-end for
JMAP-backed accounts; Pass 9h.6 will layer IMAP on the same pipeline.

The dominant prior-art model — Thunderbird, Apple Mail, Geary,
Outlook, Fastmail web, Gmail web — is server-canonical with
APPEND-then-destroy-old upload, last-write-wins on conflict, and
cadences from ~10s to 5min. Terminal clients (mutt, aerc, alpine)
have no autosave at all. Poplar is a full-featured client, so the
GUI pattern applies.

## Decision

- Cache schema v7 adds a `drafts` table keyed by App-internal UUID.
  `server_uid` and `server_folder` are nullable and paired by a
  CHECK: both NULL means local-only (not yet pushed); both NOT NULL
  means the row has a server image.
- `compose.Model` autosaves to the local cache on a 1s debounce
  (typing pause) and emits `EnqueuePushDraftMsg` every 5min while
  `pushDirty` is set (Thunderbird's cadence) and on close-with-save.
- A new outbox op `KindPushDraft` carries the assembled MIME in
  `outbox.payload` and routes through the existing drainer conflict
  matrix.
- `mail.Backend.PushDraft(folder, mime, prevUID) (UID, error)` is
  the protocol primitive. JMAP batches `Email/import` (with the
  `$draft` keyword) and `Email/set destroy` on `prevUID` in one
  request — atomic at the network layer. notFound on the destroy is
  benign: the prior image was already gone, so the new image is
  canonical (last-write-wins).
- The IMAP impl is a stub returning `mail.ErrUnsupported`. The App
  gates compose-with-cache on `Backend.IsJMAP()` so IMAP accounts
  retain today's in-memory-only flow until Pass 9h.6.
- `Ctrl+C` on a dirty JMAP compose opens a Save / Discard /
  Keep-editing ConfirmModal. Empty compose closes silently and
  removes the placeholder row. Discard removes the local row and
  queues `Destroy` against any server image. Send removes the local
  row and queues `Destroy` against any prior image.
- The Drafts folder's message list projects local-only drafts
  (server_uid IS NULL) as synthetic `draft:<id>` UIDs appended to
  the normal server-synced rows. The App's Enter handler keys off
  the `draft:` prefix and routes through `cache.Account.LoadDraft`.

## Consequences

- Cross-device draft visibility within ~5min of stopping typing on
  JMAP accounts; closing compose pushes immediately on the next
  reachable drainer wakeup.
- IMAP users see no behavior change in 9h.5. 9h.6 lifts the gate.
- Schema v7 is a pure additive migration; the partial index on
  non-null `server_uid` keys the App's Drafts-Enter lookup.
- The `CacheStore` interface in `internal/ui/compose/` is a real
  test seam — `*cache.Account` is the production impl and a fake
  powers compose's unit tests. Single-impl interface justified
  per go-conventions.
- `mail.ErrUnsupported` is a new sentinel for capability-gated
  backend methods. The drainer does NOT route it through the
  conflict matrix as auth/transient — the App gates on capability
  before queueing.
- Conflict policy is last-write-wins; no UI signal in 9h.5. A
  banner ("draft superseded by another client") is queued for 9h.6
  alongside the IMAP impl.
- One known limitation logged as backlog #39: the discard path
  leaves the local row behind in some scenarios — autosave timer
  vs. delete race. Pass 9h.6 will fix as part of the same
  subsystem.

---
title: Cache I split — foundation pass ships first; UI cutover follows
status: accepted
date: 2026-05-02
---

## Context

Pass 8.4a's plan (`docs/superpowers/plans/2026-05-02-cache-i-implementation.md`)
listed 17 ordered tasks across seven phases: cache scaffolding,
ChangeTracker impls, Backend interface collapse, cache reads + writes
+ syncer + drainer, UI strangler-fig cutover (read switch, write
switch, legacy-path delete), and tests. The full set is a coherent
unit but two distinct halves: a self-contained protocol-and-storage
foundation, and a UI rewiring that touches every account-tab call
site and the App-layer optimistic-state plumbing introduced by
ADR-0089.

Landing both in one commit defeats reviewability — the cache
package's correctness can be argued from its own tests, but the UI
cutover requires reading the entire `internal/ui/` diff alongside
the cache calls to verify the new state flow. The strangler-fig
order in the plan (cache writes → cache reads → delete legacy)
naturally suggests the same boundary.

## Decision

Pass 8.4a ships the **cache foundation** (Phases 1–5 + tests):
schema, account, ChangeTracker interface and JMAP/IMAP impls,
QueueOp, syncer, drainer, end-to-end tests via fake backends. UI
continues to call `mail.Backend` directly; no read or write goes
through `cache.Account` yet.

Pass 8.4a-cutover (next) ships **Phase 3 + Phase 6**: shrink
`mail.Backend` to `Flag` (drop `MarkRead`/`MarkUnread`/`MarkAnswered`/
`Delete`), thread `*cache.Account` into `AccountTab`, switch reads
to `QueryFolder`, switch writes to `QueueOp`, delete `MessageList.Apply*`
and the `triageStartedMsg.onUndo` field, delete the App-layer
optimistic-state plumbing.

Pass 8.4b (Cache II) and Pass 8.4c (Cache III) are unchanged.

## Consequences

- The cache package is fully implemented and tested but currently
  has no production caller. That is intentional — it lets review
  focus on the storage/sync correctness without UI noise. The
  cutover pass will exercise it.
- Pre-beta refactor freedom (ADR-0105) means the next pass can
  rename or restructure cache APIs without compat shims if the
  cutover surfaces ergonomic problems.
- The plan doc stays in `docs/superpowers/plans/` (not archived)
  because it covers both this pass and the cutover.

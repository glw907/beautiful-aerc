# ADR-0004: an operation-shaped backend seam

Date 2026-07-27. Status: accepted (Phase 4).

## Context

C4 requires one Fastmail account in v1 with seams that admit a
second account and a Gmail backend later (horizon 1 and 2). The
calendar transport is CalDAV today with a designed JMAP upgrade
(CA-1, C11). A protocol-shaped seam would leak JMAP idioms into
every engine.

## Decision

`internal/backend` defines `Backend` as composable sources
(`Mail()`, `Calendar()`, `Contacts()`, `Push()`) speaking poplar's
model types in operation vocabulary: changes-since-token,
fetch-bodies, apply-mutation, submit-with-lifecycle. A
`Capabilities` struct carries the facts engines branch on (server
thread identity, push transport, delta granularity, submission
semantics). The Fastmail v1 backend composes `backend/jmapsource`
(mail, contacts) with `backend/dav` (calendar); the composition
lives in account config. The JMAP-calendars upgrade swaps the calendar
source; a Gmail backend is a new composition behind the same
interface.

## Alternatives considered

- **JMAP-shaped seam** (methods mirroring Email/changes et al.):
  every future backend would emulate JMAP semantics it does not
  have; capability differences (threading, push) would surface as
  hacks instead of flags.
- **No seam until the second backend exists**: C4 forbids it, and
  the schema half of the seam (account-keyed rows, replaceable
  server ids) is nearly free now and expensive later.
- **Plugin-style dynamic backends**: a non-goal; backends are
  compiled in (C3, one binary).

## Consequences

Engines are backend-agnostic and testable against scriptable
fakes. The seam is also the test seam (ADR-0014). The v1 backend
is allowed to be the only implementation; the single-impl
interface is justified by C4's explicit horizon obligation, and
the interface stays minimal (one method set per source, no
speculative surface).

## Revision 2 (2026-07-27, post-review)

- **Shapes, not vocabulary**: `Changes` returns hydrated
  ChangeSets and mutations go through `ApplyBatch` with
  creation-id references, so JMAP keeps single-request batching
  (changes+get via back-references) and offline
  create-folder-then-move dispatches as one request. Mailbox
  lifecycle (create/rename/delete) and optional `ServerSearch`
  (SR-7) are seam operations; revision 1 omitted both while the
  outbox and search designs required them.
- **Capabilities extended** to what the engines actually branch
  on: thread identity is three-valued (`None` /
  `ReferencesDerived` / `ServerHeuristic`; Gmail's subject-window
  threading is the third case, and TH-1's no-false-merge
  criterion binds `ReferencesDerived` backends, recorded as a
  per-backend carve-out), plus server limits, server-search and
  scheduled-send flags, the RSVP mechanism (probe-resolved per
  backend), and per-capability account ids.
- **A `Credentials` seam** (`Token(ctx)` with single-flight
  refresh and keyring persistence) makes OAuth a backend
  property instead of a three-worker restructure; `auth` gains
  the `refresh-failed` sub-reason.

---
title: IMAP mapping for Destroy primitive
status: accepted
date: 2026-05-02
---

## Context

ADR-0092 declares `mail.Backend.Destroy(uids)` as the irreversible
permanent-delete primitive. The JMAP impl maps it to `Email/set
{ destroy }` and treats `notFound` as success. IMAP needs an
equivalent that is atomic enough to be useful, idempotent, and
restricted to the targeted UIDs (not "expunge everything marked
\Deleted in this folder").

## Decision

`mailimap.Backend.Destroy(uids)` issues:

1. `UID STORE <uids> +FLAGS.SILENT (\Deleted)` — mark each target.
2. `UID EXPUNGE <uids>` — UIDPLUS-scoped expunge that only removes
   the messages we just marked. UIDPLUS is required at Connect
   (capSet.UIDPLUS asserted), so this branch is always available.

Empty input is a no-op (matches ADR-0092). Missing UIDs are
silently ignored by the server, which matches JMAP's `notFound`
treatment. The two commands are not transactional, but the
combination of `+FLAGS.SILENT` + UID-scoped EXPUNGE means any
partial failure leaves the server in a coherent state: the worst
case is a message marked `\Deleted` but not yet expunged, which
the next Destroy or expunge cycle cleans up.

This avoids the trap of plain `EXPUNGE`, which would also delete
any other message in the folder that happened to be marked
`\Deleted` (e.g., by another client or by leftover state from a
crashed prior session).

## Consequences

- Manual Empty Trash (ADR-0094) and the per-session retention
  sweep (ADR-0093) compose Destroy and inherit its semantics.
- The IMAP backend cannot run against a server without UIDPLUS,
  by Connect-time assertion. Documented in `mailimap/README.md`.
- Provider-specific quirks (Gmail's "must select [Gmail]/Trash
  before EXPUNGE truly deletes") are deferred to GmailQuirks; the
  generic path assumes RFC 4315 semantics.

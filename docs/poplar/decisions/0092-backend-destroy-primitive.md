---
title: Backend.Destroy primitive — irreversible permanent delete
status: accepted
date: 2026-05-01
---

## Context

`mail.Backend.Delete` soft-deletes (move to Trash). Two new pass-6.6
features — per-session retention sweep on Disposal folders and a
manual "empty folder" key — both need to bypass Trash and remove
messages permanently from the provider. JMAP exposes
`Email/set { destroy }`; IMAP exposes `\Deleted` + `EXPUNGE` (and
Gmail's UID EXPUNGE). The primitive belongs at the `mail.Backend`
boundary, not duplicated per consumer.

## Decision

Add `Destroy(uids []UID) error` to `mail.Backend` as a peer of
`Delete`. Empty input is a no-op. JMAP implementation issues
`Email/set { destroy: ids }` and treats `notFound` entries in
`NotDestroyed` as success (idempotent — already gone server-side).
The IMAP backend will implement Destroy in Pass 8 alongside the
Gmail rewrite; until then the package does not exist.

## Consequences

- Two consumers (`destroyCmd` for retention sweep, `emptyFolderCmd`
  for manual empty) share a single backend call site.
- The MockBackend mirrors the semantics: records `DestroyCalls` and
  removes UIDs from its source slice.
- Destroy never participates in undo — there is no inverse Cmd. The
  toast for an empty-folder operation suppresses `[u undo]`.

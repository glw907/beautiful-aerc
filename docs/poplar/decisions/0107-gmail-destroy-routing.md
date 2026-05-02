---
title: Gmail Destroy routes via SELECT [Gmail]/Trash
status: accepted
date: 2026-05-02
---

## Context

ADR-0100 specified the IMAP `Destroy` mapping (STORE \Deleted +
UID EXPUNGE) and explicitly deferred Gmail's quirk: EXPUNGE only
truly deletes when the SELECTed mailbox is `[Gmail]/Trash`;
elsewhere it removes the matching label. Pass 8.1 implements that
quirk.

## Decision

`mailimap.Backend.Destroy(uids)` branches on `b.cfg.GmailQuirks`.
Generic path is unchanged. Gmail path:

1. Resolve Trash via `resolveTrashFolder()` (cached on first
   resolution; reused with `Delete`).
2. `cmd.Select(trash, false)`.
3. `cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"})`.
4. `cmd.UIDExpunge(uids)`.

The caller contract: UIDs must reference messages already in
`[Gmail]/Trash`. Both real callers — manual Empty Trash
(ADR-0094) and the per-session retention sweep (ADR-0093) —
satisfy this because they only trigger inside Disposal folders.
A caller that violates the contract gets a NO/BAD response from
the server (UID not in Trash), which surfaces as a clear error
rather than silent data loss.

No selection-restore step. Every other backend method
(`OpenFolder`, `QueryFolder`, …) issues its own `Select` before
acting, so leaving the cmd connection on Trash after `Destroy`
costs nothing.

## Consequences

- Gmail's "delete inside Trash" semantic now matches the JMAP
  `Email/set { destroy }` semantic from `mailjmap`.
- Future "permanent delete from arbitrary folder" UIs would need
  to MOVE-then-Destroy on Gmail; not in scope for v1.
- `internal/mailimap/README.md` documents the contract.

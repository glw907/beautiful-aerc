---
title: IMAP ChangeTracker — scan-and-diff first; CONDSTORE later
status: accepted
date: 2026-05-02
---

## Context

The Cache 0 spec (§B.3) sketches the IMAP `ChangeTracker.Changes`
implementation as `UID FETCH 1:* CHANGEDSINCE <modseq>` plus
`UID SEARCH UID > <maxuid>`, with CONDSTORE asserted at `Connect`
to make this efficient. The spec is right that CONDSTORE is the
correct steady-state implementation — flag-only changes can be
detected without a full FETCH round-trip, and `<modseq>` is the
canonical incremental cursor on RFC 7162 servers.

Implementing the full CONDSTORE path requires extending the
internal `imapClient` interface (`internal/mailimap/client.go`) to
expose CONDSTORE primitives, plumbing modseq through the
`SyncToken`, and adding the CONDSTORE assertion at `finishConnect`.
That's a real chunk of mailimap work tangential to the cache
foundation this pass actually delivers.

## Decision

Cache I ships the simpler "scan-and-diff" form of
`mailimap.Backend.Changes`: select the folder, run `UID SEARCH ALL`,
diff against the prior `maxuid` encoded in the SyncToken, return
the new UIDs as `Added`. `Modified` is always nil. `Removed` is
nil on the first call (no prior snapshot) and best-effort on later
calls (no UIDPLUS/VANISHED stream yet).

`UIDVALIDITY` change still maps to `mail.ErrCannotCalculateChanges`,
so the cache's re-anchor path (spec §D.4) handles the only case
where the diff would be incorrect.

The CONDSTORE-aware implementation is logged to `BACKLOG.md` and
will land in a follow-up pass. The interface is unchanged; only
the `mailimap` impl flips.

## Consequences

- Flag-only changes (the steady-state dominant case) currently
  trigger a full `Backend.FetchHeaders` for the affected UIDs,
  not the cheaper modseq path. Acceptable for online-only
  Cache I scope; unacceptable for high-volume accounts long-term
  — hence the BACKLOG entry.
- The 12-byte SyncToken layout reserves bytes 0–3 for
  `uidvalidity` (currently always 0). When the CONDSTORE pass
  lands it will start populating that field plus the modseq;
  the `decodeIMAPToken` helper handles legacy zero-valued
  uidvalidity gracefully, so old tokens stay valid across the
  upgrade.
- NOMODSEQ servers don't fail `Connect` in this pass (the
  CONDSTORE assertion isn't wired yet). When CONDSTORE lands
  the assertion fires per the spec.

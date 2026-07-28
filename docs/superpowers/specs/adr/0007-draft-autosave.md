# ADR-0007: local-first drafts with atomic server replacement

Date 2026-07-27. Status: accepted (Phase 4).

## Context

CO-6: 1-second-debounce local autosave, server push on close and
every 5 idle minutes, server-canonical last-write-wins, kill -9
losing at most the debounce window. The live probe (2026-07-27)
found Fastmail silently no-ops in-place Email body updates: the
call returns success, the state token does not advance, the body
is unchanged. Error-checking alone would report phantom saves.

## Decision

The local store is the edit buffer: autosave writes the body and
`draft_meta` locally on the debounce. Server push uses the
probe-verified atomic pattern, one `Email/set` call creating the
replacement draft and destroying its predecessor (single state
transition, confirmed atomic). Push success is verified by the
returned created id, never by absence of error on an update. Send
deletes the draft and dispatches in one intent. `Email/changes`
coalescing a create+destroy pair inside one window into nothing is
documented expected behavior, not a missed change.

## Alternatives considered

- **In-place Email/set update of bodyValues**: the probe result
  rules it out; it is the dangerous path precisely because it
  looks like it works.
- **Server-side autosave on every debounce tick**: a JMAP round
  trip per second of typing, against measured 0.4-0.5s call
  latency; the close/idle cadence matches observed webmail
  behavior and CO-6's loss bound is carried by the local write.
- **Local-only drafts, push on send**: fails the CO-6 round-trip
  criterion (a draft must be editable from Fastmail web).

## Consequences

Draft identity is poplar's internal key; the server id rotates on
every push, and `draft_meta` tracks the mapping. The 50-run
kill-during-compose test (CO-6) exercises the local half; a
tagged live check exercises the replacement pattern against the
real server.

## Revision 2 (2026-07-27, post-review)

The rotating server id left drafts unanchored across resync and
blind to concurrent web edits. Three additions:

- **A stable anchor**: poplar mints a Message-ID header at draft
  creation (`draft_meta.anchor_msgid`), preserved byte-identically
  across every replacement; resync re-anchors on it.
- **Revision accounting**: `pushed_rev` replaces the timestamp;
  dirty is the computed comparison `local_rev > pushed_rev`, so a
  push that raced typing leaves later revisions correctly
  unpushed with no wall-clock reasoning (SY-3).
- **The contended branch**: `notFound` on the destroy half means
  another client replaced the draft; reconcile against the server
  draft carrying the anchor instead of creating a duplicate. An
  encrypt-on-send compose (CO-11, later) suppresses server draft
  push entirely.

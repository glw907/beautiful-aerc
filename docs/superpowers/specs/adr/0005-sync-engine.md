# ADR-0005: two-watermark delta sync with push and normal resync

Date 2026-07-27. Status: accepted (Phase 4).

## Context

SY-2 requires delta sync from persisted state with push keeping
the inbox live, poll fallback with backoff, and clean recovery
from server state resets. JMAP reports that state changed, never
what changed field-by-field.

## Decision

Per account and object kind, two persisted watermarks: the opaque
server state token and a local revision counter (the mujmap/lieer
shape). The mail worker runs changes-since batches through the
writer, listens to the EventSource push stream (coalescing bursts
in a 200ms window), falls back to polling with jittered
exponential backoff on stream loss, and treats
`cannotCalculateChanges` or token expiry as a normal full-resync
trigger that rebuilds server-derived state while preserving
local-only state (bodies, outbox, draft revisions). Conflicts
resolve by server state ordering with local losing ties (SY-3);
poplar never field-merges. Calendar and contacts poll by
collection state on the SY-2 cadence with focus refresh.

## Alternatives considered

- **Field-level merge on conflict**: JMAP and CalDAV cannot say
  what changed remotely; merging would be guessing. Every
  surveyed bridge (mujmap, lieer) refuses to merge for the same
  reason.
- **Resync as an error path**: JMAP servers may expire state at
  any time; a client that treats expiry as exceptional corrupts
  or alarms on a routine event.
- **Websocket push** (RFC 8887): go-jmap has no support and
  Fastmail's EventSource endpoint is live and templated (probe).
  EventSource with poll fallback is the supported path.

## Consequences

Sync state is two columns per object kind, trivially inspectable.
The 30s p95 push-recovery criterion is a synctest scenario.

## Revision 2 (2026-07-27, post-review)

- **The EventSource auth risk is resolved**: a live probe
  (2026-07-27) confirmed Bearer-header auth works (200,
  StateChange on connect, pings on the requested cadence); the
  2021 401 report does not reproduce. The stall detector treats
  a missing ping past 2x the requested interval as a drop.
- **queryChanges is dropped**: the local store is the list view;
  revision 1 carried two mechanisms for the same job and the
  second had no state-token home and a second resync trigger.
- **Push coalescing is a fixed 200 ms delay from the first
  event**, never a resetting debounce (a steady remote burst
  must not defer sync indefinitely).
- **Self-echo suppression**: the dispatcher records the state
  tokens its dispatches produce and the sync worker skips
  applying its own changes, so autosave pushes do not round-trip
  into draft re-hydrations.
- Thread rows derive from Email threadIds; there is no
  Thread/changes round trip.

# ADR-0018: `Last-Event-ID` is an optimization, `/changes` is the resume point

Date 2026-07-29. Status: proposed (Phase 5, pass 1b). Implements
the push half of ADR-0005 against the `jmap` library's own
EventSource client. Evidence: the JMAP test inventory's JT-22
(`docs/poplar/research/2026-07-28-jmap-test-inventory.md`).

## Context

RFC 8620 section 7.3 hands push resumption to the server-sent
events standard in one sentence and no worked example: "a client
following the server-sent events specification will send a
Last-Event-ID HTTP header field with the last id it saw, which the
server can use to work out whether the client has missed some
changes. If so, it SHOULD send these changes immediately on
connection."

Three things follow from that sentence and are the whole problem.
The behavior is a SHOULD, so a conformant server may ignore the
header. The RFC defines no error for "I do not remember that
event id", unlike the analogous `/changes` case, which has
`cannotCalculateChanges`. And a server that ignores the header
answers a resumed connection exactly as it answers a fresh one, so
a client cannot tell the two apart from anything on the wire.

The field offers no cover. A grep of the whole `apache/james-project`
repository for `Last-Event-ID` returns one hit, in a specification
document, and none in server code or tests. James does not
implement it. Stalwart and Fastmail are unverified. The discarded
go-jmap client never read an SSE `id:` field at all, so the
question never arose for it.

The consequence poplar has to avoid is precise. A client that
sends the header and then treats the reconnected stream as
continuous drops every change from the disconnected window, in
silence, on the servers that ignore it. That is a mailbox that
quietly stops matching the server.

## Decision

**poplar sends the header and assumes nothing.**

`jmap`'s EventSource client captures the `id:` field of every
dispatched event as the WHATWG rules define it, keeps it across
reconnects, and sends it as `Last-Event-ID` on every reconnect
after the first. The id it sends is the id of the last event it
actually dispatched, never one from an event abandoned half-read
at the end of a broken stream.

**Every connection is a gap.** The client reports each connection
to its caller through one callback, and that callback carries no
"resumed" flag and no replay count, because either would invite
the caller to skip work on the strength of a courtesy no server
owes. The obligation that falls out of this is on the sync
engine. Every one of those reports must produce a `/changes` pull
from the persisted state token for each subscribed type, whatever
arrived on the stream afterwards. `internal/sync` does not do
this yet, and the Consequences below carry it as task 6's.

**The state token is the resume point. The SSE id is an
optimization.** ADR-0005's two watermarks per account and object
kind are durable server-side tokens with a defined failure mode
when they expire, `cannotCalculateChanges`, which triggers a
normal full resync. Resume correctness rests entirely on them. A
server that honors `Last-Event-ID` saves poplar a round trip and
shortens the recovery window. A server that ignores it costs
poplar one `/changes` call it was going to make anyway. Neither
can lose a change.

**A replayed change is a duplicate, not a hazard.** A server that
does honor the header pushes the missed state events on
connection, and the caller then pulls `/changes` as well. Both
paths converge on the same state token, and applying a delta
poplar already holds is a no-op, so the redundancy costs one
request.

## Alternatives considered

- **Do not send the header.** Simpler, and correct under this
  decision's own logic, since nothing depends on it. It forfeits
  a real recovery-time win against every server that does
  implement resumption, and the price of keeping that win is one
  HTTP header.
- **Sniff whether the server honored it**, by comparing the state
  tokens replayed on connection against the last one held. An
  absence of replay is indistinguishable from having missed
  nothing, so the sniff is unfalsifiable, and it would license
  skipping the `/changes` pull on exactly the servers where the
  pull is load-bearing. DV-08's rule against sniffing for
  capabilities applies here too.
- **Ask the server whether it supports resumption.** There is
  nothing to ask. No capability, session property, or error code
  reports it.
- **Report the connection as "resumed" when the header was sent**,
  leaving the caller to decide. Rejected: it moves an
  undecidable question one layer up and hands the caller a flag
  whose only honest use is to ignore it.

## Consequences

JT-22 is testable without a live server and without guessing what
a real one does, because both servers exist in the test suite.
One fake honors the header and replays a missed state event, one
ignores it and replays nothing, and the client behaves
identically against both. Task 5's Stalwart run and any later
Fastmail probe become evidence about recovery latency, not about
correctness, so an answer either way changes no code.

**Flush on connect is task 6's obligation, and the cutover is not
complete without it.** `internal/sync`'s push loop flushes on a
coalescing timer started by an arriving notification, so a
reconnect that replays nothing produces no `/changes` pull at
all. That is precisely the server this record was written about,
the one that ignores `Last-Event-ID`, and against it the
disconnected window is dropped in silence. The adapter that binds
this client to `backend.Push` has to turn every connect report
into a notification.

The push recovery bound in ADR-0005 (30s p95) is met by the
reconnect schedule rather than by resumption. The first retry
after a drop from a healthy stream waits under 250ms, and the
`/changes` pull that follows the reconnect is the same call the
poll fallback makes. The bound is stated for that case. A
schedule saturated by roughly seven consecutive failures draws
uniformly from `[0, 30s)`, whose p95 is 28.5s before the pull, and
that trade buys a server in trouble the room to recover.

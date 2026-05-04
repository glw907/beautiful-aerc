---
title: Status-bar outbox depth segment + offline UI hint
status: accepted
date: 2026-05-03
---

## Context

The user needs to see at a glance whether ops are queued, and
whether anything has gone wrong. The connection indicator
(ADR-0097) covers transport state but says nothing about the
queue.

A new chrome row was considered and rejected — at the 80×24
polish bar (ADR-0109), each row is precious.

## Decision

Augment the existing status bar (`internal/ui/status_bar.go`)
with an inline segment between counts and the connection icon:

- `⇅N` (FgDim) when in-flight count > 0 and conflict count = 0.
  N = pending + executing + failed.
- `⚠N` (ColorWarning) when conflict count > 0. N = full queue
  depth (in-flight + conflict). Conflict glyph dominates;
  count shows scale.
- Segment hidden when both counts are 0.

Offline framing uses the existing `mail.ConnState` signal
(ADR-0097). When ConnState transitions to `Offline` and the
outbox is non-empty, App emits a one-shot ErrorMsg banner
"offline — queued ops will sync on reconnect" and latches
`offlineHinted` to suppress repeats. Connected clears the
latch. Empty outbox stays silent — no point flagging "offline"
if there's nothing pending.

The drainer's behavior is unchanged by ConnState. It keeps
attempting with the existing exponential backoff (cap 60s);
ConnState is a UI belief about the network, not a drainer
policy.

## Consequences

- Outbox depth is visible without opening any overlay.
- One ErrorMsg banner per offline transition (not a stream).
- No new chrome row; 80×24 polish bar preserved.
- Drainer policy stays simple — no offline-detection
  heuristics, no ConnState gate on the drainer goroutine.

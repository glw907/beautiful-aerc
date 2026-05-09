---
title: List-Unsubscribe — RFC 8058 one-click preferred, mailto fallback opens compose
status: accepted
date: 2026-05-09
---

## Context

RFC 2369 advertises an unsubscribe path via the `List-Unsubscribe`
header (mailto and/or http URLs). RFC 8058 layers a one-click POST
profile on top: when the sender opts in via
`List-Unsubscribe-Post: List-Unsubscribe=One-Click`, the client
posts `List-Unsubscribe=One-Click` to the https URL with no human
in the loop. Every major client in poplar's matrix (Thunderbird,
Apple Mail, Outlook, mutt, aerc, K-9, Geary, Evolution) surfaces
the affordance whenever the header is present and fires on click;
none remember prior unsubscribes.

## Decision

- Single key `U` in the viewer (modifier-free uppercase per
  ADR-0076; free in the keybinding map). Inert when no
  List-Unsubscribe headers are present.
- Confirmation prompt via `ConfirmModal`. POST is irreversible.
- Action precedence: https one-click POST > mailto into compose >
  plain http via the existing `URLOpener` seam.
- Mailto fallback opens poplar compose pre-filled (To/Subject/Body
  parsed from the mailto URL); we don't route mailto through
  `xdg-open` since poplar is the mail client.
- Plain http (no one-click promotion) routes through `URLOpener`
  with no POST — same path as `1`–`9` link launch.
- No client-side memory of prior unsubscribes. The unsub endpoint
  is the source of truth (idempotent in practice); a well-behaved
  list stops sending after a successful unsub. Universal across
  the matrix.
- Header parsing lives in `internal/content/listunsubscribe.go` as
  a pure function `ParseListUnsubscribe(textproto.MIMEHeader)
  Unsubscribe`. Parse runs in the existing body-fetch Cmd; result
  rides back on `reader.BodyLoadedMsg.Unsub`. `mail.MessageInfo`
  is not extended — the affordance is viewer-only.

## Consequences

- Viewer footer gains a conditional `U unsub` hint at drop rank 6.
- Confirm modal cascade picks up one new pending state
  (`pendingUnsub`); precedence is unsubscribe > empty > others.
- Success surfaces via a new `lastNotice` tier in the chrome
  banner row (between error and triage toast). 5-second visibility
  with auto-clear.
- Pre-1.0 revisit: if usage shows users want a "you've already
  done this" signal, add per-List-Id memory in a new schema
  table. Not in scope here.

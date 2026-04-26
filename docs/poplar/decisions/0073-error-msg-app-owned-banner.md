---
title: ErrorMsg as canonical Cmd error type, App-owned banner
status: accepted
date: 2026-04-25
---

## Context

Pass 2.5b-4 introduced four backend Cmds (`loadFolders`, `loadFolder`,
`loadBody`, `markRead`) that all funneled failures into a private
`backendErrMsg` and dropped them in `AccountTab`. The user had no
visible feedback when mark-read silently failed, when body fetch
errored, or when `xdg-open` could not launch. Pass 6 (toast + undo)
needs an error surface to build on, and Pass 9 (send progress) needs
the same wiring. The audit-3 plan-shape findings called for
consolidating these dropped errors into one banner before adding more
sources.

## Decision

`ErrorMsg` is the canonical `tea.Msg` type emitted by every poplar
`tea.Cmd` that can fail:

```go
type ErrorMsg struct {
    Op  string
    Err error
}
```

Every Cmd that returns an error populates `Op` with a short
human-readable verb phrase ("mark read", "fetch body", "open folder"),
not a stack trace. The renderer formats banners as `⚠ <Op>: <Err>` —
or `⚠ <Err>` when `Op` is empty.

Banner state is hoisted to `App` (`lastErr ErrorMsg`), mirroring the
help-popover ownership pattern from ADR-0071. `App.Update` intercepts
every `ErrorMsg`, stores it, and forwards. Last-write-wins: a
subsequent `ErrorMsg` replaces the prior one. There is no dismiss key,
no severity field, and no error queue in v1 — those land in Pass 6
when toast + undo arrive.

The banner renders one row above the status bar via
`renderErrorBanner`. It is foreground-only (`ColorError` over the
default panel background, no fill) so the eye reads it as content
overflow rather than a chrome alert. When `lastErr.Err != nil` the
account region shrinks by one row so total view height is unchanged;
when help is open `View` short-circuits and the banner is hidden along
with everything else underneath.

## Consequences

- **Surface unlocked.** Every silent-failure path from Pass 2.5b-4 now
  bubbles up through one consolidated render. Pass 6 toast/undo can
  build on the same Msg pipeline without adding a parallel channel.
- **Banner is chrome, not modal.** It does not steal keys; routing
  continues normally. This is intentionally distinct from the modal
  infrastructure in ADR-0071 (help popover).
- **Op naming becomes a convention.** Future Cmds add a new `Op` value
  rather than a new message type. The banner formatter stays trivial.
- **No structured error stream.** Errors are last-write-wins; multiple
  errors collapse to whichever came last. Acceptable for the v1
  silent-failure rescue. Pass 6's undo bar will introduce a queue if
  the toast surface needs persistence.
- **No dismiss key in v1.** The banner remains visible until a new
  successful operation clears `lastErr` (set to `ErrorMsg{}` by
  whichever future code path acknowledges recovery). Pass 6 will
  revisit when toast/undo arrives.

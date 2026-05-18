# Async Backend Connect

## Problem

`cmd/poplar/root.go:147` blocks the main goroutine on
`backend.Connect(ctx)` before launching the bubbletea UI. The JMAP
session fetch against `api.fastmail.com/jmap/session` typically
takes 1–3 s cold, and a 5.9 s instance was observed on
2026-05-17. Nothing renders during this wait. The terminal looks
hung.

The data the UI would render is already on disk — cache schema v13
holds folders, message metadata, and recently-viewed bodies. The
binary is refusing to draw it.

## Goal

UI renders within ~100 ms of launch, populated from cache. Backend
connect runs in a `tea.Cmd`; results merge in via `Msg`. Status
bar carries the connecting / connected / disconnected signal. No
"is it frozen?" perception, ever.

## Non-goals

- Multi-account parallel connect. Single-account today; stays
  single-account here.
- Auto-retry with backoff for initial connect. Mid-session
  reconnect already works through the existing
  `mail.ConnReconnecting` path. A failed initial connect surfaces
  in the status bar; the user retries by keystroke. Backoff is a
  follow-up if it bites in dogfood.
- Offline body reads for messages whose bodies are not yet cached.
  `FetchBodyCached` works pre-wire; cache misses surface
  `cache.ErrNotConnected`.

## Architecture shift

**Today:**

```
openBackend → backend.Connect [blocks 1–6 s] → cache.Open(backend)
            → StartDrainer → ui.NewApp → tea.Run
```

**After:**

```
openBackend → cache.Open(nil) → ui.NewApp(acct) → tea.Run
                                          │
                                          ▼ tea.Cmd: backend.Connect
                                          │
                                          ▼ Msg: BackendReadyMsg | BackendErrMsg
                              acct.WireBackend(backend, ct)
                              → StartDrainer → push subscription → initial sync
```

The backend value exists from `openBackend` onward (`mailjmap.New`
is pure construction). What changes is *when authentication runs*
and *who owns the wiring step*.

## Components

### `internal/cache`

Split `Open` into two phases.

- `cache.Open(name, dir string, cfg Config, log *slog.Logger)
  (*Account, error)` — sqlite open + migrations only. No backend,
  no `ChangeTracker`. `Account.Backend` and `Account.ChangeTracker`
  are nil after `Open`.
- `(*Account).WireBackend(backend mail.Backend, ct
  mail.ChangeTracker) error` — assigns the fields, starts the
  drainer goroutine, subscribes to push updates. Called exactly
  once per account lifetime (on `BackendReadyMsg`). Re-calling
  returns an error; reconnects after a `ConnReconnecting` cycle
  reuse the wired backend.
- `(*Account).AccountName()` reads a new `accountName string`
  field set at `Open` time. Previously delegated to
  `a.Backend.AccountName()` even though the value originates in
  the account config — a layering bug surfaced by this work.
  Fix it inline.
- `(*Account).AccountEmail()` returns `""` pre-wire (no JMAP
  session yet, so no primary-account email). Callers that render
  it already tolerate `""`.

Read paths that hit cache only (`QueryFolder`, `ListFolders`,
`FetchBodyCached`, `SuggestAddresses`, `LookupContact`) work
pre-wire. Read paths that backfill against the backend
(`FetchHeaders` for unknown UIDs in `reads.go:229`, `FetchBody`
on miss in `reads.go:261`, `Attachments` /
`FetchAttachment` in `attachments.go`) return a new sentinel
`cache.ErrNotConnected`. Callers in the UI gate on this sentinel
and show a transient "waiting for connection" state instead of an
error overlay.

Outbox queueing works pre-wire — writes go to the `outbox` table.
Rows sit in `queued` until `WireBackend` starts the drainer, then
dispatch normally. This is the same state the drainer reaches
during a `ConnReconnecting` window.

`internal/cache/syncer.go:69,109` (initial sync) runs only after
`WireBackend`. The first sync kicks off as part of `WireBackend`,
not as a separate UI-driven step.

### `cmd/poplar/root.go`

Reorder. `openBackend` stays synchronous (zero I/O). `cache.Open`
runs before UI launch with `nil` for the backend slot. The
`backend.Connect` call moves into a `tea.Cmd` returned from
`App.Init()`.

`defer backend.Disconnect()` stays at the same level — the value
exists; it's just unauthenticated until the Cmd resolves.
`Disconnect` on an unconnected JMAP/IMAP backend must be a no-op
(verify; add a guard if needed).

### `internal/ui` (App)

State additions in `App`:

```go
backendState uicore.BackendState // connecting | connected | failed
backendErr   error               // last connect error, if any
```

New msgs in `internal/ui/cmds.go`:

```go
type BackendReadyMsg struct{}
type BackendErrMsg   struct{ Err error }
```

`connectBackendCmd(backend, acct)` performs `backend.Connect(ctx)`
and on success calls `acct.WireBackend(backend, ct)` before
emitting `BackendReadyMsg`. On failure emits `BackendErrMsg{err}`.
The Cmd owns the wire step so the Update path sees a fully-wired
account.

`App.Init()` returns `connectBackendCmd` alongside its existing
init Cmds. `App.Update` claims the two new msg types in
`app_chrome.go` (or a new `app_connect.go` if the chrome file is
already crowded — settle in implementation).

On `BackendReadyMsg`: set `backendState = connected`, dispatch
`pumpUpdatesCmd` + initial sync Cmd (today these run because the
backend exists at `NewApp` time; they move to post-wire).

On `BackendErrMsg`: set `backendState = failed`, store err. UI
remains usable for cached reads. User keystroke re-runs
`connectBackendCmd`. Key binding: surface in the status-bar help
line ("r to retry") only when `backendState == failed`. Keep
single-key, no modifiers (per ADRs and memory).

### `internal/ui/status_bar.go`

Already renders `connText = "connected"` at line 132. Extend to:

- `connecting` — spinner + "Connecting to Fastmail…"
- `connected` — existing rendering
- `disconnected` — error string + " · r to retry"

Driven by the existing `mail.ConnState` enum. Add
`ConnConnecting` to the enum if not present. The current
`ConnReconnecting` value covers mid-session drops; `ConnConnecting`
covers the pre-authenticated initial state.

### `internal/ui/messagelist`

When `messages` is empty and the account is in `connecting` state
(first run, no cached data), render centered "Connecting to
Fastmail…" + spinner instead of the existing empty-folder
placeholder. Once any rows exist, render rows normally and let
the status bar carry the connection signal.

### `internal/ui/account` and `internal/ui/sidebar`

No behavior change. Render whatever the cache returns. Empty
folder list pre-first-sync is fine — the status bar tells the user
why.

### `internal/uicore`

Add `BackendState` enum (`Connecting`, `Connected`, `Failed`).
Add the spinner instance for the "Connecting to Fastmail…"
placeholder via existing `NewSpinner` factory.

## Data flow

1. `runRoot` loads configs, calls `openBackend` (pure), calls
   `cache.Open` (sqlite + migrations, ~5 ms), builds `ui.NewApp`,
   starts `tea.Run`.
2. UI renders frame 1: sidebar shows cached folders, message list
   shows cached inbox rows (or the connecting placeholder if
   first run), status bar shows the connecting spinner. Wall time
   from binary launch: ~100 ms.
3. `App.Init()`'s connect Cmd runs `backend.Connect(ctx)`. On
   success, it calls `acct.WireBackend(backend, ct)` (which
   starts the drainer and subscribes to push), then emits
   `BackendReadyMsg`.
4. `App.Update` handles `BackendReadyMsg`: transitions
   `backendState` to `connected`, dispatches `pumpUpdatesCmd` +
   initial sync. Status bar transitions to `connected`. The
   initial sync emits `cache.UpdateMsg` events as folders and
   messages refresh; the UI re-renders normally.

## Error handling

Connect failure during the initial Cmd surfaces as
`BackendErrMsg{err}`. Status bar shows the error string and `r to
retry`. UI is fully usable for cached reads and outbox queueing.
A second `r` keystroke re-runs `connectBackendCmd`.

Connect failure mid-session is unchanged: the existing
`pumpUpdates` loop emits `ConnReconnecting` updates and redials.

Cache-backfill misses (UI tries to fetch a body or headers that
aren't cached, backend not yet wired) return
`cache.ErrNotConnected`. Callers in the reader / messagelist
catch the sentinel and render an inline "waiting for
connection…" hint where the body would go. No error overlay.

## Testing

**Unit:**

- `cache.Open` without backend: open, migrate, query fixture
  rows, get them back. Then `WireBackend(mockBackend)` and verify
  a queued outbox row dispatches.
- `cache.WireBackend` called twice returns an error; first call's
  side effects (drainer goroutine) are not duplicated.
- `cache.Account.FetchBody` for an uncached UID pre-wire returns
  `cache.ErrNotConnected`.
- `App.Init()` returns a Cmd; feeding `BackendReadyMsg` to
  `App.Update` transitions `backendState` to `connected` and
  dispatches the expected follow-up Cmds.
- `App.Update` for `BackendErrMsg` sets `failed` + stores err;
  re-firing the connect Cmd on the retry-key path produces the
  next `BackendReadyMsg` / `BackendErrMsg` cycle.

**Live tmux** (`.claude/docs/tmux-testing.md`):

- Launch poplar against the Fastmail account with a populated
  cache. Screenshot at t≈200 ms: cached message list visible.
  Screenshot at t≈3 s: refreshed.
- Launch with `--config` pointing at a freshly-wizard'd account
  (no cache yet). Expect "Connecting to Fastmail…" placeholder,
  then populated list after connect.
- Block egress to `api.fastmail.com` (e.g. `iptables -A OUTPUT -d
  api.fastmail.com -j REJECT`) and launch. Expect status bar
  shows the error within Connect's timeout; cached reads still
  work; `r` retries.

## Out-of-scope adjacencies noticed

These are surfaced for the implementing pass to decide inline (per
pre-beta stance), not deferred to a future pass:

- `cache.Account.AccountName()` delegating to backend is a layering
  bug — fix inline as part of the cache split.
- `cache.Account.AccountEmail()` returning `""` pre-wire is the
  honest answer; callers must already tolerate empty (header
  rendering, etc.). Verify each call site behaves on empty.
- `mail.ConnState` may not include `Connecting`; add the enum
  value inline.

## Touched files

- `internal/cache/cache.go` — `Open` signature change, new
  `accountName` field, `WireBackend` method.
- `internal/cache/account.go` — `AccountName` / `AccountEmail`
  pre-wire behavior.
- `internal/cache/drainer.go` — drainer start moves into
  `WireBackend`.
- `internal/cache/reads.go`, `internal/cache/syncer.go`,
  `internal/cache/attachments.go` — backend-touching paths gate on
  `a.Backend != nil` and return `cache.ErrNotConnected`.
- `cmd/poplar/root.go` — reorder. `cache.Open` moves before
  `ui.NewApp`. `backend.Connect` deleted from `runRoot`; new
  Cmd-based path runs inside the UI.
- `internal/uicore` — `BackendState` enum, `ConnConnecting` if
  added to `mail.ConnState`.
- `internal/ui/cmds.go` — `BackendReadyMsg`, `BackendErrMsg`,
  `connectBackendCmd`.
- `internal/ui/app.go` (or `app_chrome.go`) — state fields,
  `Init` returning the connect Cmd, Update handlers.
- `internal/ui/status_bar.go` — connecting / disconnected
  rendering.
- `internal/ui/messagelist/messagelist.go` — connecting
  placeholder when rows empty + state connecting.
- New test files alongside each touched package.

Pass-budget estimate: 9–11 tasks, one ADR titled "Async backend
connect; cache opens before UI; backend wired via Msg".

## ADR scope

One ADR codifying:

- Cache lifecycle splits into `Open` (no backend) and
  `WireBackend`.
- UI is responsible for driving backend connect via `tea.Cmd`,
  not `cmd/poplar/root.go`.
- Backend-touching cache paths return `cache.ErrNotConnected`
  pre-wire; UI renders cached state without blocking.
- Status bar gains `Connecting` state distinct from
  `Reconnecting`.

Cross-reference the layering-bug fix in `AccountName`.

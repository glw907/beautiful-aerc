# ADR-0242: Async Backend Connect; Cache Opens Before UI

**Status:** accepted
**Date:** 2026-05-17

## Context

Synchronous `backend.Connect` in `cmd/poplar/root.go` blocked the
bubbletea program from starting until the JMAP session fetch
completed. Typical Fastmail session response is 1–3 s cold; 5.9 s
observed once on 2026-05-17. Nothing rendered during the wait; the
binary appeared frozen. The cache already had everything the UI
needed for the initial frame.

A secondary layering bug: `cache.Account.AccountName()` delegated to
the backend. The cache layer had no business calling up into the
mail layer for a value it already held.

## Decision

- `cache.Open(name, dir, cfg, log)` opens SQLite and runs migrations
  only. No backend, no ChangeTracker.
- `(*Account).WireBackend(backend, ct)` attaches the backend, assigns
  the change tracker, and starts the backfiller and drainer goroutines.
  Call exactly once per Account lifetime; a second call returns an error.
- Backend-touching cache reads return `cache.ErrNotConnected` pre-wire.
  Cache-only reads (`ListFolders`, `QueryFolder`, `FetchBodyCached` on
  hit, `SuggestAddresses`, `LookupContact`) work unchanged.
- `App.Init` returns `connectBackendCmd(ctx, backend, acct)`. The Cmd
  calls `Connect`, then `WireBackend`, then emits `BackendReadyMsg{}`.
  On failure: `BackendErrMsg{Err}`.
- App owns `backendState uicore.BackendState` + `backendErr error`.
  The status bar renders these pre-wire; falls through to
  `mail.ConnState` post-wire. Messagelist empty-state shows
  '◐ Connecting…' when `backendState == BackendConnecting` and the
  source is empty.
- `mail.ConnConnecting` distinguishes the pre-authenticated initial
  state from `ConnReconnecting` ("was connected, lost it"). Appended
  so `ConnOffline = 0` zero-value stays meaningful.
- `r` retries `connectBackendCmd` when state is `BackendFailed`;
  otherwise unclaimed so `r` reaches the reply binding.
- `cache.Account.AccountName()` reads the name passed to `Open`;
  no backend delegation.

## Consequences

- UI renders within ~100 ms of binary launch (cache-driven).
- Flaky network yields a degraded experience instead of a hung
  terminal; cached reads still work pre-wire.
- Drainer + backfiller lifetimes now match the wired backend's.
- Drainer-blocked outbox ops sit in `queued` until wire, then
  dispatch normally — same as a mid-session `ConnReconnecting` window.
- Backlog #60: `connectBackendCmd` uses `context.Background()`; quit
  during connect orphans the dial goroutine until timeout. Benign for
  single-user daily-driver; revisit if it bites.

## Alternatives considered

Splash screen + sync connect. Cosmetic; doesn't fix the layering or
unlock offline reads.

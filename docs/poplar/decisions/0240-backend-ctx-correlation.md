# ADR-0240: Context threading through mail.Backend + op_id correlation

**Status:** accepted
**Date:** 2026-05-17

## Decision

Six mutating `mail.Backend` methods — `Move`, `Flag`, `Destroy`,
`Send`, `Append`, `PushDraft` — gain `ctx context.Context` as the
first parameter.

`internal/logctx` provides `WithOpID(ctx, id)` and a `Handler`
slog wrapper that injects a context-carried `op_id` attribute into
every record. `installLogger` wraps the text handler with it.
`WithAttrs` and `WithGroup` re-wrap so loggers derived via
`slog.Logger.With` keep the injection.

The drainer's `executeOne` attaches the outbox row ID via
`logctx.WithOpID(ctx, fmt.Sprint(row.ID))` before calling
`dispatch`, and `dispatch` threads that ctx into every backend
call. Every log record emitted during a queued operation now
carries `op_id` automatically. `executeOne` also emits debug
events at dispatch entry and on each terminal-success branch
(`dispatchErr == nil`, `contacts.ErrNotFound`, `mail.ErrNotFound`,
`ErrDraftSuperseded`).

`mailimap.Backend.Send` threads the new ctx into
`smtpClientLocked` so caller cancellation propagates into the lazy
SMTP dial and OAuth refresh paths. `mailjmap` discards ctx — the
underlying `go-jmap` `Do` method takes none. `mailimap` `Move`,
`Destroy`, `Flag`, `Append`, `PushDraft` also discard for the same
reason at the go-imap call seam.

## Consequences

Read-path methods (`FetchBody`, `FetchHeaders`, `QueryFolder`,
`ListFolders`, `Capabilities`, `FetchAttachment`, `Updates`) keep
their existing signatures. They run on the UI thread, not the
drainer, and have no op_id to correlate.

Non-drainer callers of the mutated methods pass
`context.Background()` — currently only tests. When a UI-driven
synchronous mutator lands (e.g. a "send now" path that bypasses
the outbox), it will pass its own ctx and the absence of an op_id
in records is correct.

Adding new log calls anywhere downstream of `dispatch` requires no
per-site changes — calling `slog.DebugContext(ctx, ...)` with the
threaded ctx picks up `op_id` automatically.

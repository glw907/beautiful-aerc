---
title: log/slog adoption — diagnostic logging convention
status: accepted (amended by 0209 — WithLogger collapsed to plain *slog.Logger arg)
date: 2026-05-10
---

## Context

Before Pass 16d, diagnostic output across `internal/` was a mix of
`fmt.Fprintf(os.Stderr, ...)` calls — invisible under bubbletea's
altscreen, untagged, and unfilterable by level. Four sites in
`internal/mailjmap` (push-loop state change, blob refresh, two
dropped-update paths), two in `internal/cache/drainer`, and
previously-silent failure paths in `internal/mailimap` (idle
reconnect, dropped update, SMTP client drop) all wrote to raw
stderr. The 16-series modernization makes Pass 16d the natural
moment to convert to `log/slog` before new subsystems inherit the
old pattern.

## Decision

`log/slog` is the diagnostic logging path for code in `internal/`.
User-facing CLI text in `cmd/poplar/` stays on `os.Stderr` via
`fmt.Fprintln` — those are UX strings, not log events.

**Handler and level.** `cmd/poplar/main.go` installs the root
handler via `installLogger` before cobra executes.
`slog.NewTextHandler` at `slog.LevelInfo`. `POPLAR_LOG=debug`
raises to `slog.LevelDebug`.

**Destination.** When stdout is a TTY (TUI mode), the handler
writes to `$XDG_STATE_HOME/poplar/poplar.log` (default
`~/.local/state/poplar/poplar.log`), append mode, created on
demand. Non-TTY stdout (pipe, redirect, test runner) falls back
to `os.Stderr`. If the log file cannot be opened, startup
silently falls back to stderr — logging must never crash startup.

**Per-component logger.** Backend constructors accept a variadic
`Option` parameter; `WithLogger(*slog.Logger)` is the seam. The
default logger is `slog.Default().With("component", "<pkg>")`.
The same shape is used across `internal/mailjmap`, `internal/mailimap`,
and `internal/cache`. `WithLogger` is the test seam: tests pass
`slog.New(slog.NewTextHandler(&buf, ...))` to capture output
without touching `slog.SetDefault`.

**Sites converted this pass.**

- `internal/mailjmap`: 4 sites (`push.go` ×3, `jmap.go` push-draft
  destroy-prior). `os` import removed from both files.
- `internal/mailimap`: 3 new sites added (`idle.go` reconnect
  attempt Warn with attempt count + delay + err; `idle.go` dropped
  update Warn when updates channel is full; `smtp.go` SMTP client
  dropped Warn on send error).
- `internal/cache`: `stderrLog` package var and its `os` import
  removed from `drainer.go`; 2 sites converted to `a.log.Error`.

`encodeErr` JSON in `outbox.error_json` is unchanged — that is
persisted state on disk, not log output.

## Consequences

All new subsystem code in `internal/` defaults to slog.
`POPLAR_LOG=debug` is the recommended first step when diagnosing
push-loop or IMAP reconnect issues. The log file accumulates
across sessions; no rotation is provided pre-beta. `WithLogger`
is the established test seam; `slog.SetDefault` swap is not used
in any test.

### Alternatives considered

- **Context-carried logger (Option B).** Threading `context.Context`
  through every backend call would let callers attach per-request
  fields, but the backend interface is synchronous blocking with
  no per-call context today. The structural change dwarfs the
  logging benefit; deferred until the interface evolves.
- **`slog.SetDefault` swap in tests (Option C).** Process-global;
  parallel test suites race on it. `WithLogger` per-constructor
  is safe and already idiomatic for functional-option patterns in
  this codebase.

# Logging Infrastructure — Design Spec

**Date:** 2026-05-17
**Passes:** 42 (correlation foundation) + 43 (visibility layer)
**Status:** approved, pending implementation plan

## Background

Three gaps identified through research and a live hang diagnosis:

1. No wire-level protocol tracing for IMAP, JMAP, or SMTP — the most useful
   forensic tool when a connection hangs.
2. Almost no Debug-level instrumentation inside the backends — 19 total log
   call sites in the codebase, mostly Warn/Error on failure paths. Normal
   operations are silent.
3. No operation correlation — log records from a drainer dispatch, its
   backend call, and its cache commit are unrelated entries with no shared
   identifier.

A fourth gap folded in during design: no log rotation. With wire tracing
active the log file grows without bound; size-based rotation via lumberjack
is the right fix and belongs in the same pass as wire tracing.

## Architecture

Two passes, ordered B-then-A to avoid migration scars:

- **Pass 42** establishes the correlation foundation (context threading,
  `internal/logctx`, drainer op_id propagation). Debug checkpoints written
  in Pass 43 use `DebugContext` and carry `op_id` automatically — no rework.
- **Pass 43** adds the visibility layer (wire tracing, protocol checkpoints,
  log rotation, startup marker) built on top of the Pass 42 infrastructure.

---

## Pass 42 — Correlation Foundation

### `internal/logctx` (new package)

Two exported symbols:

```go
func WithOpID(ctx context.Context, id string) context.Context
type Handler struct{ slog.Handler }
```

`Handler.Handle` checks for `op_id` in context via an unexported key type
and calls `r.AddAttrs(slog.String("op_id", id))` before delegating to the
wrapped handler. `installLogger` in `cmd/poplar/log.go` wraps the text
handler with `logctx.Handler` at startup. From that point, any
`slog.DebugContext(ctx, ...)` call whose context carries an op ID gets the
attr automatically — no per-call site changes required.

### `mail.Backend` interface

The six mutating methods called by the drainer gain a `ctx context.Context`
first parameter:

- `Move(ctx, uids, dest)`
- `Flag(ctx, uids, flag, set)`
- `Destroy(ctx, uids)`
- `Send(ctx, env, mime)`
- `Append(ctx, folder, mime, flags)`
- `PushDraft(ctx, folder, mime, prevUID)`

Read-path methods (`FetchBody`, `FetchHeaders`, `QueryFolder`, etc.) are
left unchanged — correlation matters most for async queued operations, not
synchronous UI reads. Non-drainer callers of the changed methods pass
`context.Background()`.

### Cache drainer

`dispatch` gains a `ctx context.Context` parameter. `executeOne` calls it
with the op ID attached:

```go
a.dispatch(logctx.WithOpID(ctx, fmt.Sprint(row.ID)), args, row)
```

The drainer logs `op_id` and `kind` at dispatch entry and at each terminal
outcome (done, conflict, failed, max-attempts). Because `logctx.Handler` is
global, every backend log record produced during that dispatch carries
`op_id` without touching the backend call sites.

### Scope

Files touched: `internal/logctx` (new), `internal/mail/backend.go`,
`internal/mailimap/`, `internal/mailjmap/`, `internal/cache/drainer.go`,
`cmd/poplar/log.go`, `cmd/poplar/backend.go`, and all direct callers of the
six changed Backend methods. `MockBackend` (dev build tag) updated to match.

### ADR

ADR-0240: context threading through `mail.Backend` and op_id correlation.

---

## Pass 43 — Visibility Layer

### `wire-trace` config option

`WireTrace bool` added to `UIConfig`. TOML key `wire-trace` in `[ui]`.
`POPLAR_WIRE_TRACE=1` env var overrides the config value. The effective bool
is resolved in `runRoot` after `uiCfg` loads:

```go
wireTrace := uiCfg.WireTrace || os.Getenv("POPLAR_WIRE_TRACE") == "1"
backend, err := openBackend(accts[0], wireTrace)
```

Independent of `log-level` — both can be set or unset in any combination.
`renderUIBlock` emits the key only when true. Config template gains a comment
documenting credential exposure.

### Log rotation via lumberjack

`openStateLog` is replaced. `installLogger` constructs a
`*lumberjack.Logger` as the slog writer when stdout is a TTY:

```go
w = &lumberjack.Logger{
    Filename:   filepath.Join(stateDir(), "poplar.log"),
    MaxSize:    10,   // MB
    MaxBackups: 2,
}
```

The hand-rolled `os.OpenFile` path is removed. Lumberjack handles atomic
rotation, keeping at most two prior sessions as `.log.1` / `.log.2`. Size-
based rotation is more appropriate than session-based given that a single
wire-trace session can exceed one prior session's worth of data.

### `wireWriter` type

Defined in `cmd/poplar/log.go`. Implements `io.Writer`; splits on newlines
and emits one `slog.Debug` record per line:

```
level=DEBUG msg="imap wire" component=wire data="A001 LOGIN..."
```

Credential exposure is expected — wire-trace is an explicit opt-in with
documentation in the config template.

### Protocol wiring

`mailimap.New`, `mailimap.NewWithOAuth`, and `mailjmap.New` each gain a
`wireTrace bool` parameter. `openBackend` in `cmd/poplar/backend.go` gains
the same and receives `uiCfg.WireTrace` from `runRoot` (which already loads
`uiCfg` before calling `openBackend`).

**IMAP** — `imapclient.Options.DebugWriter` set to a `wireWriter` when
`wireTrace` is true. Applied in `mailimap/auth.go` where `imapclient.Options`
is constructed.

**SMTP** — `gosmtp.Client.DebugWriter` set to a `wireWriter` after dial in
`mailimap/smtp.go`. `gosmtp.Client.DebugWriter` is a standard field on the
emersion/go-smtp client.

**JMAP** — No native `DebugWriter`. `jmap.Client.HttpClient` (an exposed
`*http.Client` field) is set to an `http.Client` with a custom
`loggingTransport` that wraps `http.DefaultTransport`, calls
`httputil.DumpRequestOut` / `httputil.DumpResponse`, and pipes each through
a `wireWriter`. Defined in `mailjmap/`.

### Debug checkpoints

Protocol-step `DebugContext` calls at key boundaries. These carry `op_id`
automatically via the Pass 42 handler. Sites:

**`mailimap`**
- Dial (host, port)
- TLS layer (server name)
- Auth mechanism (no credentials logged)
- Folder SELECT (folder name)
- IDLE start / stop (stop includes reason)
- Cmd-path redial

**`mailjmap`**
- Session fetch (endpoint URL)
- EventSource push connect
- State-change dispatch (type)

**`cache`**
- `Open` (schema version)
- Each migration step (from, to version)
- Drainer pickup (supplements Pass 42 op_id logging)

### Startup session marker

```go
slog.Info("poplar start", "account", accts[0].Name, "config", configPath)
```

Emitted in `runRoot` immediately after `installLogger`. Makes session
boundaries trivially locatable in a multi-session log file even with
lumberjack keeping prior sessions.

### New dependency

`gopkg.in/natefinch/lumberjack.v2` added to `go.mod`.

### ADR

ADR-0241: wire tracing, debug checkpoints, and log rotation.

---

## Task budget

| Pass | Tasks |
|------|-------|
| 42 | ~9 |
| 43 | ~10 |

Both within the 8–12 task budget. Each pass gets one ADR.

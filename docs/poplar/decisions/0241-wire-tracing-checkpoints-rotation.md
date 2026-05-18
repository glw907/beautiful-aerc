# ADR-0241: Wire tracing, debug checkpoints, log rotation

**Status:** accepted
**Date:** 2026-05-17

## Context

Pass 42 landed the correlation foundation (ctx-carried `op_id`).
Daily-driver use surfaces opaque hangs and silent normal-path
operations: the existing log shows errors but not the timeline
that leads up to them, and there is no protocol-level transcript
for diagnosing connection problems. The hand-rolled
`openStateLog` also grows unbounded — a wire-trace session can
easily blow past the previous session's worth of bytes.

## Decision

**Wire tracing.** `wire-trace = true` in `[ui]`
(or `POPLAR_WIRE_TRACE=1`) enables protocol-level traffic logging
via `imapclient.Options.DebugWriter` (IMAP),
`gosmtp.Client.DebugWriter` (SMTP), and a `loggingTransport`
wrapping `jmap.Client.HttpClient.Transport` (JMAP). All three
route through `logctx.WireWriter`, an `io.Writer` that splits on
newlines and emits one `slog.Debug` record per non-empty line with
`component=<protocol>`. Backend constructors gain a `wireTrace
bool` parameter; `runRoot` resolves the flag (env beats config)
and `openBackend` threads it through. Wire-trace is independent of
`log-level` — either flag enables debug records for its scope.

**Debug checkpoints.** `b.log.Debug` / `slog.Debug` calls fill the
"normal operations are silent" gap without touching error paths:

- `mailimap`: `dial`, TLS handshake, auth mechanism, IDLE
  start/stop, cmd-redial complete.
- `mailjmap`: session fetch, EventSource connect, StateChange
  dispatch.
- `cache`: `Open` (schema version), each migration step. Drainer
  dispatch/done already landed in ADR-0240.

**Log rotation.** `openStateLog` replaced by `*lumberjack.Logger`
(10 MB, 2 backups). Size-based rotation matches the actual failure
mode: a wire-trace session is much larger than a quiet one, so
session-based rotation would either truncate useful state or grow
without bound.

**Startup marker.** `slog.Info("poplar start", "account", …,
"config", …)` in `runRoot` immediately after `installLogger` makes
session boundaries trivially locatable in a rotating log.

## Consequences

New dependency: `gopkg.in/natefinch/lumberjack.v2`. Wire-trace
logs contain credentials and message content; the config template
warns about this and frames wire-trace as a debugging-only toggle.
Debug checkpoints inside a drainer dispatch automatically carry
`op_id` via the Pass 42 handler, so a wire-trace transcript and
the surrounding application events join on a single id.

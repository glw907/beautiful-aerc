---
title: TCP keepalive uses stdlib net.KeepAliveConfig — drop vendored helper
status: accepted
date: 2026-05-07
---

## Context

`internal/mailauth/keepalive/` carried two vendored aerc snippets
(`SetTcpKeepaliveProbes`, `SetTcpKeepaliveInterval`) that reached
into `*os.File` socket fds to tune `TCP_KEEPCNT` and `TCP_KEEPINTVL`
via `syscall.SetsockoptInt`. The helper existed because Go's stdlib
historically only exposed `net.TCPConn.SetKeepAlive(bool)` and a
single `KeepAlive` duration on `net.Dialer`, with no access to
probe count or interval.

Go 1.23 (August 2024) added `net.KeepAliveConfig` exposing
`Enable`/`Idle`/`Interval`/`Count` directly on `net.Dialer` and
`net.TCPConn`. Poplar's go.mod is on 1.26, so the stdlib option is
available everywhere we run.

The user wants the codebase free of aerc-derived code where a
better option exists. XOAUTH2 SASL stays vendored (no library
ships an XOAUTH2 mechanism against `emersion/go-sasl`); the
keepalive helper does not.

## Decision

Delete `internal/mailauth/keepalive/`. `mailimap/auth.go:dialRawTCP`
configures `net.Dialer.KeepAliveConfig` with the same constants the
helper used (Idle 30s, Interval 30s, Count 3) and drops the
`*net.TCPConn` syscall step entirely.

## Consequences

- One vendored package and its provenance comments are gone. The
  remaining aerc footprint is a single SASL file
  (`internal/mailauth/xoauth2.go`) that has no library equivalent.
- Cross-platform behavior is the stdlib's, not poplar's: macOS,
  Linux, and Windows now all honor `KeepAliveConfig` rather than
  Linux-only syscall tuning with a no-op fallback elsewhere.
- The `keepalive_dummy.go` no-op build constraint goes away with
  the package.
- `internal/mailauth/README.md` and `invariants.md` lose the
  keepalive entry; the vendored-snippets fact narrows to XOAUTH2.

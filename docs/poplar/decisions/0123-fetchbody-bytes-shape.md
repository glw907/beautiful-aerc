---
title: mail.Backend.FetchBody returns ([]byte, error)
status: accepted
date: 2026-05-03
---

## Context

Cache II needs the body size before insertion (for the size backstop check) and
a hit-return path that avoids re-reading from disk. The prior interface shape
was `FetchBody(uid uint32) (io.Reader, error)`, which forced an `io.ReadAll`
at every consumer and required a separate size measurement step before storing.
The JMAP backend already operated on `[]byte` internally; the `io.Reader` wrap
was an indirection with no benefit.

## Decision

`mail.Backend.FetchBody(uid uint32) ([]byte, error)`. Both JMAP and IMAP
backends and all test fakes conform. `loadBodyCmd` in `internal/ui/cmds.go`
uses the returned slice directly. The internal `clientCmd.FetchBody` helper in
`internal/mailimap/client.go` retains its `io.ReadCloser` shape — only the
public `Backend` interface changes; the IMAP backend impl drains and closes the
reader internally before returning `[]byte`.

## Consequences

One fewer indirection at every call site. JMAP drops a `bytes.NewReader` wrap
on the hit path and a singleflight result wrap. Body size is available without
a separate read. Pre-beta interface change with no compat shim.

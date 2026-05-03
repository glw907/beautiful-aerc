# Pass 8.5 B — Overengineering Audit: `internal/mailimap/`

**Date:** 2026-05-03
**Lens:** IMAP backend; capability-negotiation paths.

## Findings

- internal/mailimap/auth.go:32 — 2 — `dialCommand` is a one-line wrapper calling `dial(cfg, pw, "command")`; one call site in `imap.go`.
  Action: inline
  Rationale: The wrapper adds no logic; `Connect` can call `dial(b.cfg, pw, "command")` directly, eliminating a spurious indirection layer alongside `dialIdle`.

- internal/mailimap/auth.go:38 — 2 — `dialIdle` is a one-line wrapper calling `dial(cfg, pw, "idle")`; one call site in `imap.go`.
  Action: inline
  Rationale: Same as `dialCommand` — the role string is the only variation; both wrappers exist solely to give the two calls distinct names already expressed by the role argument.

- internal/mailimap/idle.go:143 — 2 — `handleUnilateral` is a one-line method (`b.emit(u)`) with exactly one call site (`idle.Idle(b.handleUnilateral)` in `runIdleSession`).
  Action: inline
  Rationale: Passing `b.emit` as the `onUpdate` callback directly removes an indirection with no loss of clarity.

- internal/mailimap/changes.go:57 — 2 — `cmdClient` is a two-line accessor method with exactly one call site in `Changes`.
  Action: inline
  Rationale: Every other method snapshots `b.cmd` under `b.mu.Lock()` inline; `cmdClient` is an inconsistent one-off extract.

- internal/mailimap/client.go:62 — 3 — `listEntry.HasChildren` is set in `realclient.go:100` but never read anywhere.
  Action: delete
  Rationale: grep -r HasChildren finds only the declaration and the setter.

- internal/mailimap/imap.go:111 — 4 — `finishConnect(ctx context.Context)` receives `ctx` but never uses it (unparam); spawned `idleLoop` creates its own `context.WithCancel(context.Background())`.
  Action: refactor
  Rationale: Either thread the caller's context into the idle goroutine or drop the parameter — passing a discarded context is a correctness hazard.

- internal/mailimap/imap.go:48 — 3 — `capSet.XGM` is written but only read once inside `finishConnect`; the Gmail `Destroy` path uses `b.cfg.GmailQuirks`, not `b.caps.XGM`.
  Action: delete
  Rationale: The sole reader is a one-time guard inside `finishConnect`; check can be inline against `caps["X-GM-EXT-1"]` without storing.

- internal/mailimap/fake_test.go:118 — 8 — `stringReader` is a custom `io.Reader` whose only use is wrapped by `io.NopCloser`; `strings.NewReader` is a drop-in.
  Action: delete
  Rationale: `strings.NewReader` satisfies `io.Reader` and is already available.
